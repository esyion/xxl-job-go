import { useCallback, useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Select, Space, Table, Tag, message } from 'antd'
import { KeyOutlined, PlusOutlined } from '@ant-design/icons'
import { apiUserPageList, apiUserRemove, apiUserSave, apiUpdatePwd, assertOk, JobUser } from '../api'

export default function UserPage() {
  const [list, setList] = useState<JobUser[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [query, setQuery] = useState({ start: 0, length: 10, username: '', role: -1 })
  const [editing, setEditing] = useState<Partial<JobUser> | null>(null)
  const [form] = Form.useForm()
  const [pwdOpen, setPwdOpen] = useState(false)
  const [pwdForm] = Form.useForm()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const page = await apiUserPageList(query)
      setList(page.data || [])
      setTotal(page.recordsTotal)
    } finally {
      setLoading(false)
    }
  }, [query])

  useEffect(() => { load() }, [load])

  const openEdit = (u?: JobUser) => {
    if (u) {
      setEditing(u)
      form.setFieldsValue({ ...u, password: '' })
    } else {
      setEditing({ username: '', password: '', role: 0, permission: '' })
      form.resetFields()
      form.setFieldsValue({ username: '', password: '', role: 0, permission: '' })
    }
  }

  const save = async () => {
    const values = await form.validateFields()
    if (!values.password) delete values.password
    const ret = await apiUserSave({ ...editing, ...values })
    if (assertOk(ret, '保存成功')) { setEditing(null); load() }
  }

  const changePwd = async () => {
    const values = await pwdForm.validateFields()
    const ret = await apiUpdatePwd(values.password)
    if (assertOk(ret, '密码已修改')) { setPwdOpen(false); pwdForm.resetFields() }
  }

  const columns: any[] = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '账号', dataIndex: 'username', width: 160 },
    {
      title: '角色', dataIndex: 'role', width: 100,
      render: (v) => (v === 1 ? <Tag color="blue">管理员</Tag> : <Tag>普通用户</Tag>),
    },
    { title: '权限（执行器ID）', dataIndex: 'permission', ellipsis: true, render: (v) => v || '全部' },
    {
      title: '操作', width: 150, render: (_, r: JobUser) => (
        <Space>
          <Button size="small" type="link" onClick={() => openEdit(r)}>编辑</Button>
          <Popconfirm title="确认删除该用户？" onConfirm={async () => {
            const ret = await apiUserRemove(r.id)
            if (assertOk(ret, '已删除')) load()
          }}>
            <Button size="small" type="link" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <Card>
      <Space style={{ marginBottom: 16 }}>
        <Input
          style={{ width: 180 }} placeholder="账号" allowClear
          onChange={(e) => setQuery({ ...query, username: e.target.value, start: 0 })}
        />
        <Select
          style={{ width: 120 }} placeholder="角色" allowClear
          options={[{ value: 1, label: '管理员' }, { value: 0, label: '普通用户' }]}
          onChange={(v) => setQuery({ ...query, role: v ?? -1, start: 0 })}
        />
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openEdit()}>新增用户</Button>
        <Button icon={<KeyOutlined />} onClick={() => setPwdOpen(true)}>修改密码</Button>
      </Space>

      <Table<JobUser>
        rowKey="id"
        loading={loading}
        dataSource={list}
        columns={columns}
        pagination={{
          total, pageSize: query.length, current: query.start / query.length + 1,
          showSizeChanger: true, showTotal: (t) => `共 ${t} 条`,
          onChange: (page, pageSize) => setQuery({ ...query, start: (page - 1) * pageSize, length: pageSize }),
        }}
      />

      <Modal
        title={editing?.id ? `编辑用户 #${editing.id}` : '新增用户'}
        open={!!editing}
        onOk={save}
        onCancel={() => setEditing(null)}
        destroyOnClose
      >
        <Form form={form} labelCol={{ span: 6 }} wrapperCol={{ span: 16 }}>
          <Form.Item name="username" label="账号" rules={[{ required: true, message: '请输入账号' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="password"
            label="密码"
            rules={[
              { validator: (_, v) => (!v && editing?.id) || (v && v.length >= 6)
                ? Promise.resolve() : Promise.reject(new Error('密码至少 6 位（编辑时留空表示不修改）')) },
            ]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item name="role" label="角色">
            <Select options={[{ value: 1, label: '管理员' }, { value: 0, label: '普通用户' }]} />
          </Form.Item>
          <Form.Item name="permission" label="权限">
            <Input placeholder="可访问的执行器ID，逗号分隔；留空为全部" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="修改当前用户密码" open={pwdOpen} onOk={changePwd} onCancel={() => setPwdOpen(false)} destroyOnClose>
        <Form form={pwdForm} labelCol={{ span: 6 }} wrapperCol={{ span: 16 }}>
          <Form.Item name="password" label="新密码" rules={[{ required: true, min: 6, message: '密码至少 6 位' }]}>
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}

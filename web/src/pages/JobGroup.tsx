import { useCallback, useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Radio, Select, Space, Table, message } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { apiGroupPageList, apiGroupRemove, apiGroupSave, assertOk, JobGroup } from '../api'

export default function JobGroupPage() {
  const [list, setList] = useState<JobGroup[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [query, setQuery] = useState({ start: 0, length: 10, appName: '' })
  const [editing, setEditing] = useState<(Partial<JobGroup> & { accessToken?: string }) | null>(null)
  const [form] = Form.useForm()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const page = await apiGroupPageList(query)
      setList(page.data || [])
      setTotal(page.recordsTotal)
    } finally {
      setLoading(false)
    }
  }, [query])

  useEffect(() => { load() }, [load])

  const openEdit = (g?: JobGroup) => {
    if (g) {
      setEditing(g)
      form.setFieldsValue({ ...g, accessToken: '' })
    } else {
      setEditing({ appName: '', name: '', addressType: 0, addressList: '', accessToken: 'default_token' })
      form.resetFields()
      form.setFieldsValue({ appName: '', name: '', addressType: 0, addressList: '', accessToken: 'default_token' })
    }
  }

  const save = async () => {
    const values = await form.validateFields()
    const payload = { ...editing, ...values }
    if (!payload.accessToken) delete (payload as any).accessToken
    const ret = await apiGroupSave(payload as any)
    if (assertOk(ret, '保存成功')) { setEditing(null); load() }
  }

  const columns: any[] = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: 'AppName', dataIndex: 'appName', width: 220 },
    { title: '名称', dataIndex: 'name', width: 200 },
    {
      title: '注册方式', dataIndex: 'addressType', width: 110,
      render: (v) => (v === 0 ? <span style={{ color: '#52c41a' }}>自动注册</span> : '手动录入'),
    },
    {
      title: '机器地址', dataIndex: 'addressList', ellipsis: true,
      render: (v: string) => (v ? v.split(',').map((a, i) => <div key={i}>{a}</div>) : <span style={{ color: '#999' }}>暂无注册机器</span>),
    },
    {
      title: '操作', width: 150, render: (_, r: JobGroup) => (
        <Space>
          <Button size="small" type="link" onClick={() => openEdit(r)}>编辑</Button>
          <Popconfirm title="确认删除该执行器？" onConfirm={async () => {
            const ret = await apiGroupRemove(r.id)
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
          style={{ width: 220 }} placeholder="AppName" allowClear
          onChange={(e) => setQuery({ ...query, appName: e.target.value, start: 0 })}
        />
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openEdit()}>新增执行器</Button>
      </Space>

      <Table<JobGroup>
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
        title={editing?.id ? `编辑执行器 #${editing.id}` : '新增执行器'}
        open={!!editing}
        onOk={save}
        onCancel={() => setEditing(null)}
        destroyOnClose
      >
        <Form form={form} labelCol={{ span: 6 }} wrapperCol={{ span: 16 }}>
          <Form.Item name="appName" label="AppName" rules={[{ required: true, message: '请输入 AppName' }]}>
            <Input placeholder="执行器唯一标识，需与执行器侧配置一致" disabled={!!editing?.id} />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="addressType" label="注册方式">
            <Radio.Group>
              <Radio value={0}>自动注册</Radio>
              <Radio value={1}>手动录入</Radio>
            </Radio.Group>
          </Form.Item>
          <Form.Item
            noStyle shouldUpdate={(a, b) => a.addressType !== b.addressType}
          >
            {({ getFieldValue }) => getFieldValue('addressType') === 1 && (
              <Form.Item name="addressList" label="机器地址" rules={[{ required: true, message: '请输入地址列表' }]}>
                <Input placeholder="多个地址逗号分隔，如 http://127.0.0.1:9999" />
              </Form.Item>
            )}
          </Form.Item>
          <Form.Item name="accessToken" label="AccessToken">
            <Input placeholder="留空保持不变；新执行器默认 default_token" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}

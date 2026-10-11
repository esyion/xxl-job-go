import { useCallback, useEffect, useState } from 'react'
import {
  Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Popover, Select, Space,
  Switch, Table, Tag, message,
} from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import {
  apiGroupPageList, apiJobAdd, apiJobUpdate, apiJobPageList, apiJobRemove, apiJobStart,
  apiJobStop, apiJobTrigger, apiNextTriggerTime, apiGlueSave, assertOk,
  GLUE_OPTIONS, JobGroup, JobInfo, ROUTE_OPTIONS,
} from '../api'

const BLOCK_OPTIONS = [
  { value: 'SERIAL_EXECUTION', label: '单机串行' },
  { value: 'DISCARD_LATER', label: '丢弃后续调度' },
  { value: 'COVER_EARLY', label: '覆盖之前调度' },
]
const MISFIRE_OPTIONS = [
  { value: 'DO_NOTHING', label: '忽略' },
  { value: 'FIRE_ONCE_NOW', label: '立即执行一次' },
]
const SCHEDULE_TYPES = [
  { value: 'CRON', label: 'CRON' },
  { value: 'FIX_RATE', label: '固定速度' },
  { value: 'NONE', label: '无' },
]

function emptyJob(): Partial<JobInfo> {
  return {
    jobGroup: undefined, name: '', author: '', alarmEmail: '',
    scheduleType: 'CRON', scheduleConf: '', misfireStrategy: 'DO_NOTHING',
    executorRouteStrategy: 'FIRST', executorHandler: '', executorParam: '',
    executorBlockStrategy: 'SERIAL_EXECUTION', executorTimeout: 0, executorFailRetryCount: 0,
    glueType: 'BEAN', glueRemark: '', childJobId: '',
  }
}

export default function JobInfoPage() {
  const [list, setList] = useState<JobInfo[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [groups, setGroups] = useState<JobGroup[]>([])
  const [query, setQuery] = useState({ start: 0, length: 10, jobGroup: -1, name: '', executorHandler: '', triggerStatus: -1 })
  const [editing, setEditing] = useState<Partial<JobInfo> | null>(null)
  const [form] = Form.useForm()
  const [nextTimes, setNextTimes] = useState<string[]>([])
  const [glueJob, setGlueJob] = useState<JobInfo | null>(null)
  const [glueSource, setGlueSource] = useState('')
  const [triggerJob, setTriggerJob] = useState<JobInfo | null>(null)
  const [triggerParam, setTriggerParam] = useState('')

  const loadGroups = useCallback(async () => {
    const ret = await apiGroupPageList({ start: 0, length: 1000 })
    if (ret.recordsTotal >= 0) setGroups(ret.data || [])
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const page = await apiJobPageList(query)
      setList(page.data || [])
      setTotal(page.recordsTotal)
    } finally {
      setLoading(false)
    }
  }, [query])

  useEffect(() => { loadGroups() }, [loadGroups])
  useEffect(() => { load() }, [load])

  const groupName = (id: number) => groups.find((g) => g.id === id)?.name || id

  const openEdit = (job?: JobInfo) => {
    setNextTimes([])
    if (job) {
      setEditing(job)
      form.setFieldsValue(job)
    } else {
      setEditing(emptyJob())
      form.resetFields()
      form.setFieldsValue(emptyJob())
    }
  }

  const previewNextTime = async () => {
    const scheduleType = Form.useWatch('scheduleType', form)
    const scheduleConf = Form.useWatch('scheduleConf', form)
    if (!scheduleType || scheduleType === 'NONE' || !scheduleConf) {
      setNextTimes([])
      return
    }
    const ret = await apiNextTriggerTime(scheduleType, scheduleConf)
    if (ret.code === 200) setNextTimes(ret.content || [])
    else { setNextTimes([]); message.error(ret.msg) }
  }

  const saveJob = async () => {
    const values = await form.validateFields()
    const ret = editing?.id
      ? await apiJobUpdate({ ...editing, ...values } as any)
      : await apiJobAdd(values)
    if (assertOk(ret, '保存成功')) {
      setEditing(null)
      load()
    }
  }

  const toggleStatus = async (job: JobInfo) => {
    const ret = job.triggerStatus === 1 ? await apiJobStop(job.id) : await apiJobStart(job.id)
    if (assertOk(ret, job.triggerStatus === 1 ? '已停止' : '已启动')) load()
  }

  const doTrigger = async () => {
    if (!triggerJob) return
    const ret = await apiJobTrigger(triggerJob.id, triggerParam)
    if (assertOk(ret, '已触发')) setTriggerJob(null)
  }

  const saveGlue = async () => {
    if (!glueJob) return
    const ret = await apiGlueSave(glueJob.id, glueSource, '页面编辑')
    if (assertOk(ret, 'GLUE 源码已保存')) setGlueJob(null)
  }

  const columns: any[] = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '任务名称', dataIndex: 'name', width: 160, ellipsis: true },
    {
      title: '调度类型', width: 200, render: (_, r: JobInfo) => (
        <span>
          <Tag color={r.scheduleType === 'CRON' ? 'blue' : r.scheduleType === 'FIX_RATE' ? 'cyan' : 'default'}>{r.scheduleType}</Tag>
          {r.scheduleConf}
        </span>
      ),
    },
    { title: '执行器', dataIndex: 'jobGroup', width: 130, render: (v) => groupName(v), ellipsis: true },
    { title: 'JobHandler', dataIndex: 'executorHandler', width: 130, ellipsis: true },
    { title: '路由策略', dataIndex: 'executorRouteStrategy', width: 140, ellipsis: true },
    {
      title: '运行状态', width: 90, render: (_, r: JobInfo) => (
        <Switch checked={r.triggerStatus === 1} checkedChildren="运行" unCheckedChildren="停止" onChange={() => toggleStatus(r)} />
      ),
    },
    {
      title: '下次触发', dataIndex: 'triggerNextTime', width: 150,
      render: (v) => (v > 0 ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    { title: '作者', dataIndex: 'author', width: 90, ellipsis: true },
    {
      title: '操作', width: 260, fixed: 'right', render: (_, r: JobInfo) => (
        <Space size={4}>
          <Popconfirm
            title="确认执行一次该任务？"
            onConfirm={() => { setTriggerJob(r); setTriggerParam('') }}
          >
            <Button size="small" type="link">执行一次</Button>
          </Popconfirm>
          <Button size="small" type="link" onClick={() => openEdit(r)}>编辑</Button>
          {r.glueType !== 'BEAN' && (
            <Button size="small" type="link" onClick={() => { setGlueJob(r); setGlueSource(r.glueSource || '') }}>GLUE</Button>
          )}
          <Popconfirm title="确认删除该任务？" onConfirm={async () => {
            const ret = await apiJobRemove(r.id)
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
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          style={{ width: 180 }} placeholder="执行器"
          allowClear
          options={groups.map((g) => ({ value: g.id, label: g.name }))}
          onChange={(v) => setQuery({ ...query, jobGroup: v ?? -1, start: 0 })}
        />
        <Input
          style={{ width: 180 }} placeholder="任务名称" allowClear
          onChange={(e) => setQuery({ ...query, name: e.target.value, start: 0 })}
        />
        <Input
          style={{ width: 180 }} placeholder="JobHandler" allowClear
          onChange={(e) => setQuery({ ...query, executorHandler: e.target.value, start: 0 })}
        />
        <Select
          style={{ width: 120 }} placeholder="状态" allowClear
          options={[{ value: 1, label: '运行' }, { value: 0, label: '停止' }]}
          onChange={(v) => setQuery({ ...query, triggerStatus: v ?? -1, start: 0 })}
        />
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openEdit()}>新增任务</Button>
      </Space>

      <Table<JobInfo>
        rowKey="id"
        loading={loading}
        dataSource={list}
        columns={columns}
        scroll={{ x: 1400 }}
        pagination={{
          total, pageSize: query.length, current: query.start / query.length + 1,
          showSizeChanger: true, showTotal: (t) => `共 ${t} 条`,
          onChange: (page, pageSize) => setQuery({ ...query, start: (page - 1) * pageSize, length: pageSize }),
        }}
      />

      {/* 新增/编辑任务 */}
      <Modal
        title={editing?.id ? `编辑任务 #${editing.id}` : '新增任务'}
        open={!!editing}
        onOk={saveJob}
        onCancel={() => setEditing(null)}
        width={720}
        destroyOnClose
      >
        <Form form={form} labelCol={{ span: 6 }} wrapperCol={{ span: 16 }}>
          <Form.Item name="jobGroup" label="执行器" rules={[{ required: true, message: '请选择执行器' }]}>
            <Select options={groups.map((g) => ({ value: g.id, label: `${g.name} (${g.appName})` }))} />
          </Form.Item>
          <Form.Item name="name" label="任务名称" rules={[{ required: true, message: '请输入任务名称' }]}>
            <Input placeholder="任务描述信息" />
          </Form.Item>
          <Form.Item name="routeStrategy" label="路由策略" initialValue="FIRST" hidden />
          <Form.Item name="executorRouteStrategy" label="路由策略">
            <Select options={ROUTE_OPTIONS.map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="executorHandler" label="JobHandler"
            rules={[{ required: true, message: '请输入 JobHandler', validator: (_, v) => (form.getFieldValue('glueType') !== 'BEAN' || v) ? Promise.resolve() : Promise.reject(new Error('请输入 JobHandler')) }]}>
            <Input placeholder="任务handler，需与执行器侧注册一致" disabled={form.getFieldValue('glueType') !== 'BEAN'} />
          </Form.Item>
          <Form.Item name="executorParam" label="任务参数">
            <Input.TextArea rows={2} placeholder="任务执行所需的参数" />
          </Form.Item>
          <Form.Item name="scheduleType" label="调度类型">
            <Select options={SCHEDULE_TYPES} onChange={() => setNextTimes([])} />
          </Form.Item>
          <Form.Item
            noStyle shouldUpdate={(a, b) => a.scheduleType !== b.scheduleType || a.glueType !== b.glueType}
          >
            {({ getFieldValue }) => getFieldValue('scheduleType') !== 'NONE' && (
              <Form.Item
                name="scheduleConf"
                label={getFieldValue('scheduleType') === 'CRON' ? 'Cron' : '固定速度(秒)'}
                rules={[{ required: true, message: '请填写调度配置' }]}
              >
                <Input
                  placeholder={getFieldValue('scheduleType') === 'CRON' ? '如：0 * * * * ? （秒 分 时 日 月 周）' : '如：30'}
                  onBlur={previewNextTime}
                />
              </Form.Item>
            )}
          </Form.Item>
          {nextTimes.length > 0 && (
            <Form.Item label="下次触发预览">
              <span style={{ color: '#1890ff' }}>{nextTimes.join('  →  ')}</span>
            </Form.Item>
          )}
          <Form.Item name="misfireStrategy" label="过期策略">
            <Select options={MISFIRE_OPTIONS} />
          </Form.Item>
          <Form.Item name="executorBlockStrategy" label="阻塞处理">
            <Select options={BLOCK_OPTIONS} />
          </Form.Item>
          <Form.Item name="executorTimeout" label="超时时间(秒)">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="0 表示不限制" />
          </Form.Item>
          <Form.Item name="executorFailRetryCount" label="失败重试次数">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="0 表示不重试" />
          </Form.Item>
          <Form.Item name="glueType" label="GLUE类型">
            <Select options={GLUE_OPTIONS.map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="childJobId" label="子任务ID">
            <Input placeholder="多个子任务ID逗号分隔，父任务成功后触发" />
          </Form.Item>
          <Form.Item name="author" label="负责人">
            <Input />
          </Form.Item>
          <Form.Item name="alarmEmail" label="告警邮件">
            <Input placeholder="多个邮箱逗号分隔，为空则不告警" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 执行一次 */}
      <Modal
        title={`执行一次 #${triggerJob?.id || ''} ${triggerJob?.name || ''}`}
        open={!!triggerJob}
        onOk={doTrigger}
        onCancel={() => setTriggerJob(null)}
        destroyOnClose
      >
        <Input.TextArea
          rows={3}
          placeholder="本次执行的参数（留空使用任务配置参数）"
          value={triggerParam}
          onChange={(e) => setTriggerParam(e.target.value)}
        />
      </Modal>

      {/* GLUE 编辑 */}
      <Modal
        title={`GLUE 源码编辑 #${glueJob?.id || ''}（${glueJob?.glueType || ''}）`}
        open={!!glueJob}
        onOk={saveGlue}
        onCancel={() => setGlueJob(null)}
        width={760}
        destroyOnClose
      >
        <Input.TextArea
          rows={16}
          style={{ fontFamily: 'monospace' }}
          value={glueSource}
          onChange={(e) => setGlueSource(e.target.value)}
        />
        <Popover content="保存后，新触发的任务将使用新版源码（依赖 glueUpdatetime 变更检测）">
          <span style={{ color: '#999', marginTop: 8 }}>提示：保存即生效</span>
        </Popover>
      </Modal>
    </Card>
  )
}

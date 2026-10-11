import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Button, Card, DatePicker, Descriptions, Input, InputNumber, Modal, Popover, Select,
  Space, Table, Tag, message,
} from 'antd'
import dayjs, { Dayjs } from 'dayjs'
import {
  apiGroupPageList, apiLogClear, apiLogDetailCat, apiLogKill, apiLogPageList,
  assertOk, JobGroup, JobLog,
} from '../api'

const { RangePicker } = DatePicker

function triggerTag(code: number) {
  if (code === 200) return <Tag color="green">成功</Tag>
  if (code === 0) return <Tag>无</Tag>
  return <Tag color="red">失败</Tag>
}

function htmlContent(html: string) {
  return <div style={{ maxWidth: 520, maxHeight: 400, overflow: 'auto' }} dangerouslySetInnerHTML={{ __html: html || '-' }} />
}

export default function JobLogPage() {
  const [list, setList] = useState<JobLog[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [groups, setGroups] = useState<JobGroup[]>([])
  const [query, setQuery] = useState({
    start: 0, length: 10, jobGroup: -1, jobId: -1, logStatus: -1,
    filterTimeFrom: '', filterTimeTo: '',
  })
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)

  // 滚动日志查看
  const [viewLog, setViewLog] = useState<JobLog | null>(null)
  const [logText, setLogText] = useState('')
  const [fromLine, setFromLine] = useState(1)
  const [logEnded, setLogEnded] = useState(false)
  const timerRef = useRef<ReturnType<typeof setInterval>>()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const page = await apiLogPageList(query)
      setList(page.data || [])
      setTotal(page.recordsTotal)
    } finally {
      setLoading(false)
    }
  }, [query])

  useEffect(() => { apiGroupPageList({ start: 0, length: 1000 }).then((p) => setGroups(p.data || [])) }, [])
  useEffect(() => { load() }, [load])

  const groupName = (id: number) => groups.find((g) => g.id === id)?.name || id

  // ============ 滚动读取执行日志 ============
  const catLog = useCallback(async (log: JobLog, from: number) => {
    const ret = await apiLogDetailCat(log.id, from)
    if (ret.code !== 200) {
      message.error(ret.msg || '读取日志失败')
      setLogEnded(true)
      return
    }
    const d = ret.content
    if (d.logContent) setLogText((prev) => (prev ? prev + '\n' + d.logContent : d.logContent))
    setFromLine(d.toLineNum + 1)
    setLogEnded(d.isEnd)
  }, [])

  const openViewer = async (log: JobLog) => {
    setViewLog(log)
    setLogText('')
    setFromLine(1)
    setLogEnded(false)
    await catLog(log, 1)
  }

  useEffect(() => {
    if (!viewLog) return
    timerRef.current = setInterval(async () => {
      if (!logEnded) await catLog(viewLog, fromLine)
    }, 1000)
    return () => clearInterval(timerRef.current)
  }, [viewLog, logEnded, fromLine, catLog])

  const doKill = async (log: JobLog) => {
    const ret = await apiLogKill(log.id)
    if (assertOk(ret, '终止指令已发送')) load()
  }

  const doClear = async () => {
    const ret = await apiLogClear(query.jobGroup > 0 ? query.jobGroup : 0, query.jobId > 0 ? query.jobId : 0, 2)
    if (assertOk(ret)) load()
  }

  const columns: any[] = [
    { title: '日志ID', dataIndex: 'id', width: 90 },
    { title: '任务ID', dataIndex: 'jobId', width: 70 },
    { title: '执行器', dataIndex: 'jobGroup', width: 120, render: (v) => groupName(v), ellipsis: true },
    {
      title: '调度时间', dataIndex: 'triggerTime', width: 160,
      render: (v) => (v && v !== '0001-01-01T00:00:00Z' ? dayjs(v).format('MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '调度结果', dataIndex: 'triggerCode', width: 100,
      render: (v, r: JobLog) => (
        <Popover title="调度备注" content={htmlContent(r.triggerMsg)} trigger="click">
          <span style={{ cursor: 'pointer' }}>{triggerTag(v)}</span>
        </Popover>
      ),
    },
    {
      title: '执行时间', dataIndex: 'handleTime', width: 160,
      render: (v) => (v && v !== '0001-01-01T00:00:00Z' ? dayjs(v).format('MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '执行结果', dataIndex: 'handleCode', width: 100,
      render: (v, r: JobLog) => (
        <Popover title="执行备注" content={htmlContent(r.handleMsg)} trigger="click">
          <span style={{ cursor: 'pointer' }}>{triggerTag(v)}</span>
        </Popover>
      ),
    },
    { title: '执行器地址', dataIndex: 'executorAddress', width: 170, ellipsis: true },
    { title: 'JobHandler', dataIndex: 'executorHandler', width: 120, ellipsis: true },
    {
      title: '操作', width: 150, fixed: 'right', render: (_, r: JobLog) => (
        <Space size={4}>
          {r.triggerCode === 200 && r.handleCode === 0 && (
            <Button size="small" type="link" danger onClick={() => doKill(r)}>终止任务</Button>
          )}
          <Button size="small" type="link" onClick={() => openViewer(r)} disabled={r.triggerCode !== 200}>执行日志</Button>
        </Space>
      ),
    },
  ]

  return (
    <Card>
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          style={{ width: 170 }} placeholder="执行器" allowClear
          options={groups.map((g) => ({ value: g.id, label: g.name }))}
          onChange={(v) => setQuery({ ...query, jobGroup: v ?? -1, start: 0 })}
        />
        <InputNumber
          style={{ width: 120 }} placeholder="任务ID" min={-1}
          onChange={(v) => setQuery({ ...query, jobId: v ?? -1, start: 0 })}
        />
        <Select
          style={{ width: 120 }} placeholder="状态" allowClear
          options={[
            { value: 1, label: '成功' }, { value: 2, label: '失败' }, { value: 3, label: '进行中' },
          ]}
          onChange={(v) => setQuery({ ...query, logStatus: v ?? -1, start: 0 })}
        />
        <RangePicker
          showTime={{ format: 'HH:mm' }}
          format="YYYY-MM-DD HH:mm:ss"
          onChange={(v) => {
            setRange(v as any)
            setQuery({
              ...query,
              filterTimeFrom: v?.[0] ? v[0].format('YYYY-MM-DD HH:mm:ss') : '',
              filterTimeTo: v?.[1] ? v[1].format('YYYY-MM-DD HH:mm:ss') : '',
              start: 0,
            })
          }}
        />
        <Button danger onClick={doClear}>清理一年前日志</Button>
      </Space>

      <Table<JobLog>
        rowKey="id"
        loading={loading}
        dataSource={list}
        columns={columns}
        scroll={{ x: 1300 }}
        pagination={{
          total, pageSize: query.length, current: query.start / query.length + 1,
          showSizeChanger: true, showTotal: (t) => `共 ${t} 条`,
          onChange: (page, pageSize) => setQuery({ ...query, start: (page - 1) * pageSize, length: pageSize }),
        }}
      />

      <Modal
        title={`执行日志 #${viewLog?.id || ''}`}
        open={!!viewLog}
        footer={null}
        onCancel={() => setViewLog(null)}
        width={860}
        destroyOnClose
      >
        {viewLog && (
          <>
            <Descriptions size="small" column={2} bordered style={{ marginBottom: 12 }}>
              <Descriptions.Item label="任务ID">{viewLog.jobId}</Descriptions.Item>
              <Descriptions.Item label="JobHandler">{viewLog.executorHandler}</Descriptions.Item>
              <Descriptions.Item label="执行器地址">{viewLog.executorAddress}</Descriptions.Item>
              <Descriptions.Item label="状态">
                调度{triggerTag(viewLog.triggerCode)} 执行{triggerTag(viewLog.handleCode)}
              </Descriptions.Item>
            </Descriptions>
            <pre style={{
              background: '#111', color: '#0f0', padding: 12, borderRadius: 6,
              maxHeight: 480, overflow: 'auto', fontSize: 12, whiteSpace: 'pre-wrap',
            }}>
              {logText || '（暂无日志输出）'}
            </pre>
            <div style={{ color: '#999' }}>
              {logEnded ? '日志已全部加载' : `正在滚动读取... 当前第 ${fromLine} 行`}
            </div>
          </>
        )}
      </Modal>
    </Card>
  )
}

import { useEffect, useRef, useState } from 'react'
import { Card, Col, Row, Statistic, Table } from 'antd'
import * as echarts from 'echarts'
import dayjs from 'dayjs'
import { apiChartInfo, LogReport } from '../api'

export default function Dashboard() {
  const [reports, setReports] = useState<LogReport[]>([])
  const [jobCount, setJobCount] = useState(0)
  const [running, setRunning] = useState(0)
  const chartRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    apiChartInfo().then((ret) => {
      if (ret.code === 200) {
        setReports(ret.content.reportList || [])
        setJobCount(ret.content.jobInfoCount)
        setRunning(ret.content.runningJobCount)
      }
    })
  }, [])

  useEffect(() => {
    if (!chartRef.current) return
    const chart = echarts.init(chartRef.current)
    const days = reports.map((r) => dayjs(r.triggerDay).format('MM-DD'))
    chart.setOption({
      tooltip: { trigger: 'axis' },
      legend: { data: ['成功', '失败', '运行中'] },
      grid: { left: 40, right: 20, top: 40, bottom: 30 },
      xAxis: { type: 'category', data: days },
      yAxis: { type: 'value', minInterval: 1 },
      series: [
        { name: '成功', type: 'bar', stack: 'total', data: reports.map((r) => r.sucCount), itemStyle: { color: '#52c41a' } },
        { name: '失败', type: 'bar', stack: 'total', data: reports.map((r) => r.failCount), itemStyle: { color: '#ff4d4f' } },
        { name: '运行中', type: 'line', data: reports.map((r) => r.runningCount), itemStyle: { color: '#1890ff' } },
      ],
    })
    const onResize = () => chart.resize()
    window.addEventListener('resize', onResize)
    return () => {
      window.removeEventListener('resize', onResize)
      chart.dispose()
    }
  }, [reports])

  const todaySuc = reports.length ? reports[reports.length - 1].sucCount : 0
  const todayFail = reports.length ? reports[reports.length - 1].failCount : 0

  return (
    <div>
      <Row gutter={16}>
        <Col span={6}><Card><Statistic title="任务总数" value={jobCount} /></Card></Col>
        <Col span={6}><Card><Statistic title="运行中任务" value={running} valueStyle={{ color: '#1890ff' }} /></Card></Col>
        <Col span={6}><Card><Statistic title="今日成功" value={todaySuc} valueStyle={{ color: '#52c41a' }} /></Card></Col>
        <Col span={6}><Card><Statistic title="今日失败" value={todayFail} valueStyle={{ color: todayFail > 0 ? '#cf1322' : undefined }} /></Card></Col>
      </Row>
      <Card title="近 7 天调度情况" style={{ marginTop: 16 }}>
        <div ref={chartRef} style={{ height: 360 }} />
      </Card>
      <Card title="每日明细" style={{ marginTop: 16 }}>
        <Table<LogReport>
          rowKey={(r) => r.triggerDay}
          dataSource={reports.slice().reverse()}
          pagination={false}
          size="small"
          columns={[
            { title: '日期', dataIndex: 'triggerDay', render: (v) => dayjs(v).format('YYYY-MM-DD') },
            { title: '成功', dataIndex: 'sCount', render: (_, r) => r.sucCount },
            { title: '失败', dataIndex: 'failCount' },
            { title: '运行中', dataIndex: 'runningCount' },
          ]}
        />
      </Card>
    </div>
  )
}

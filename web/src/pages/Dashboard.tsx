import { useEffect, useState } from 'react'
import dayjs from 'dayjs'
import { Activity, CheckCircle2, ListChecks, XCircle } from 'lucide-react'
import { Bar, BarChart, CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { apiChartInfo, LogReport } from '@/api'

const barConfig = {
  suc: { label: '成功', color: 'var(--chart-2)' },
  fail: { label: '失败', color: 'var(--chart-1)' },
} satisfies ChartConfig

const lineConfig = {
  running: { label: '运行中', color: 'var(--chart-3)' },
} satisfies ChartConfig

export default function Dashboard() {
  const [reports, setReports] = useState<LogReport[]>([])
  const [jobCount, setJobCount] = useState(0)
  const [running, setRunning] = useState(0)

  useEffect(() => {
    apiChartInfo().then((ret) => {
      if (ret.code === 200) {
        setReports(ret.content.reportList || [])
        setJobCount(ret.content.jobInfoCount)
        setRunning(ret.content.runningJobCount)
      }
    })
  }, [])

  const today = reports.length ? reports[reports.length - 1] : null
  const chartData = reports.map((r) => ({
    day: dayjs(r.triggerDay).format('MM-DD'),
    suc: r.sucCount,
    fail: r.failCount,
    running: r.runningCount,
  }))

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-sm font-medium">任务总数</CardTitle>
            <ListChecks className="text-muted-foreground size-4" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{jobCount}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-sm font-medium">运行中任务</CardTitle>
            <Activity className="text-muted-foreground size-4" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">{running}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-sm font-medium">今日成功</CardTitle>
            <CheckCircle2 className="text-muted-foreground size-4" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{today?.sucCount ?? 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-sm font-medium">今日失败</CardTitle>
            <XCircle className="text-muted-foreground size-4" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-600">{today?.failCount ?? 0}</div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>近 7 天调度情况</CardTitle>
          <CardDescription>每日成功 / 失败与运行中数量</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 lg:grid-cols-2">
            <ChartContainer config={barConfig} className="h-72 w-full">
              <BarChart data={chartData}>
                <CartesianGrid vertical={false} />
                <XAxis dataKey="day" tickLine={false} axisLine={false} tickMargin={8} />
                <YAxis tickLine={false} axisLine={false} width={32} allowDecimals={false} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <ChartLegend content={<ChartLegendContent />} />
                <Bar dataKey="suc" stackId="a" fill="var(--color-suc)" radius={[0, 0, 0, 0]} />
                <Bar dataKey="fail" stackId="a" fill="var(--color-fail)" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ChartContainer>
            <ChartContainer config={lineConfig} className="h-72 w-full">
              <LineChart data={chartData}>
                <CartesianGrid vertical={false} />
                <XAxis dataKey="day" tickLine={false} axisLine={false} tickMargin={8} />
                <YAxis tickLine={false} axisLine={false} width={32} allowDecimals={false} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <ChartLegend content={<ChartLegendContent />} />
                <Line dataKey="running" type="monotone" fill="var(--color-running)" stroke="var(--color-running)" strokeWidth={2} dot={false} />
              </LineChart>
            </ChartContainer>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>每日明细</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>日期</TableHead>
                <TableHead>成功</TableHead>
                <TableHead>失败</TableHead>
                <TableHead>运行中</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {reports
                .slice()
                .reverse()
                .map((r) => (
                  <TableRow key={r.triggerDay}>
                    <TableCell>{dayjs(r.triggerDay).format('YYYY-MM-DD')}</TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-green-600">
                        {r.sucCount}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-red-600">
                        {r.failCount}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-blue-600">
                        {r.runningCount}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}

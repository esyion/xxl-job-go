import { useCallback, useEffect, useRef, useState } from 'react'
import dayjs from 'dayjs'
import { Eraser, Terminal } from 'lucide-react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import { Calendar } from '@/components/ui/calendar'
import { cn } from '@/lib/utils'
import type { DateRange } from 'react-day-picker'
import {
  apiGroupPageList, apiLogClear, apiLogDetailCat, apiLogKill, apiLogPageList, assertOk, JobGroup, JobLog,
} from '@/api'

function codeBadge(code: number, label: string) {
  if (code === 200) return <Badge variant="outline" className="text-green-600">{label}成功</Badge>
  if (code === 0) return <Badge variant="secondary">{label}无</Badge>
  return <Badge variant="destructive">{label}失败</Badge>
}

function htmlContent(html: string) {
  return <div className="max-h-80 w-[520px] max-w-[80vw] overflow-auto text-xs leading-relaxed" dangerouslySetInnerHTML={{ __html: html || '-' }} />
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
  const [dateRange, setDateRange] = useState<DateRange | undefined>()

  // 滚动日志查看
  const [viewLog, setViewLog] = useState<JobLog | null>(null)
  const [logText, setLogText] = useState('')
  const [fromLine, setFromLine] = useState(1)
  const [logEnded, setLogEnded] = useState(false)
  const timerRef = useRef<ReturnType<typeof setInterval>>(undefined)

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
  const page = query.start / query.length + 1
  const pages = Math.max(1, Math.ceil(total / query.length))

  // ============ 滚动读取执行日志 ============
  const catLog = useCallback(async (log: JobLog, from: number) => {
    const ret = await apiLogDetailCat(log.id, from)
    if (ret.code !== 200) {
      toast.error(ret.msg || '读取日志失败')
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

  const fmtTime = (v?: string) => (v && v !== '0001-01-01T00:00:00Z' ? dayjs(v).format('MM-DD HH:mm:ss') : '-')

  return (
    <Card>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Select value={query.jobGroup > 0 ? String(query.jobGroup) : 'all'} onValueChange={(v) =>
            setQuery({ ...query, jobGroup: v === 'all' ? -1 : Number(v), start: 0 })}>
            <SelectTrigger className="w-40"><SelectValue placeholder="执行器" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部执行器</SelectItem>
              {groups.map((g) => <SelectItem key={g.id} value={String(g.id)}>{g.name}</SelectItem>)}
            </SelectContent>
          </Select>
          <Input type="number" placeholder="任务ID" className="w-28" value={query.jobId > 0 ? query.jobId : ''}
            onChange={(e) => setQuery({ ...query, jobId: e.target.value ? Number(e.target.value) : -1, start: 0 })} />
          <Select value={query.logStatus >= 0 ? String(query.logStatus) : 'all'} onValueChange={(v) =>
            setQuery({ ...query, logStatus: v === 'all' ? -1 : Number(v), start: 0 })}>
            <SelectTrigger className="w-28"><SelectValue placeholder="状态" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部状态</SelectItem>
              <SelectItem value="1">成功</SelectItem>
              <SelectItem value="2">失败</SelectItem>
              <SelectItem value="3">进行中</SelectItem>
            </SelectContent>
          </Select>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" id="date" className="w-64 justify-start font-normal">
                {dateRange?.from ? (
                  dateRange.to ? (
                    <>{dayjs(dateRange.from).format('MM-DD HH:mm')} ~ {dayjs(dateRange.to).format('MM-DD HH:mm')}</>
                  ) : (
                    dayjs(dateRange.from).format('MM-DD HH:mm')
                  )
                ) : (
                  <span className="text-muted-foreground">触发时间范围</span>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0" align="start">
              <Calendar
                mode="range"
                selected={dateRange}
                numberOfMonths={2}
                onSelect={(range) => {
                  setDateRange(range)
                  setQuery({
                    ...query,
                    filterTimeFrom: range?.from ? dayjs(range.from).format('YYYY-MM-DD 00:00:00') : '',
                    filterTimeTo: range?.to ? dayjs(range.to).format('YYYY-MM-DD 23:59:59') : '',
                    start: 0,
                  })
                }}
              />
            </PopoverContent>
          </Popover>
          <Button variant="outline" className="ml-auto" onClick={async () => {
            const ret = await apiLogClear(query.jobGroup > 0 ? query.jobGroup : 0, query.jobId > 0 ? query.jobId : 0, 2)
            if (assertOk(ret)) load()
          }}><Eraser />清理一年前日志</Button>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-20">日志ID</TableHead>
              <TableHead className="w-16">任务</TableHead>
              <TableHead className="min-w-24">执行器</TableHead>
              <TableHead className="min-w-32">触发时间</TableHead>
              <TableHead className="w-20">调度结果</TableHead>
              <TableHead className="min-w-32">执行时间</TableHead>
              <TableHead className="w-20">执行结果</TableHead>
              <TableHead className="min-w-36">执行器地址</TableHead>
              <TableHead className="w-36">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading && list.length === 0 ? (
              [...Array(5)].map((_, i) => (
                <TableRow key={i}>
                  {[...Array(9)].map((_, j) => <TableCell key={j}><Skeleton className="h-5 w-full" /></TableCell>)}
                </TableRow>
              ))
            ) : (
              list.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>{r.id}</TableCell>
                  <TableCell>{r.jobId}</TableCell>
                  <TableCell className="max-w-28 truncate">{groupName(r.jobGroup)}</TableCell>
                  <TableCell className="text-xs">{fmtTime(r.triggerTime)}</TableCell>
                  <TableCell>
                    <Popover>
                      <PopoverTrigger asChild>
                        <span className="cursor-pointer">{codeBadge(r.triggerCode, '调度')}</span>
                      </PopoverTrigger>
                      <PopoverContent className="w-auto p-3">{htmlContent(r.triggerMsg)}</PopoverContent>
                    </Popover>
                  </TableCell>
                  <TableCell className="text-xs">{fmtTime(r.handleTime)}</TableCell>
                  <TableCell>
                    <Popover>
                      <PopoverTrigger asChild>
                        <span className="cursor-pointer">{codeBadge(r.handleCode, '执行')}</span>
                      </PopoverTrigger>
                      <PopoverContent className="w-auto p-3">{htmlContent(r.handleMsg)}</PopoverContent>
                    </Popover>
                  </TableCell>
                  <TableCell className="font-mono text-xs">{r.executorAddress || '-'}</TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      {r.triggerCode === 200 && r.handleCode === 0 && (
                        <Button variant="ghost" size="sm" className="h-7 px-2 text-red-600"
                          onClick={async () => {
                            const ret = await apiLogKill(r.id)
                            if (assertOk(ret, '终止指令已发送')) load()
                          }}>终止</Button>
                      )}
                      <Button variant="ghost" size="sm" className="h-7 px-2" disabled={r.triggerCode !== 200}
                        onClick={() => openViewer(r)}>日志</Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>

        <div className="text-muted-foreground flex items-center justify-between text-sm">
          <span>共 {total} 条</span>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" disabled={page <= 1}
              onClick={() => setQuery({ ...query, start: (page - 2) * query.length })}>上一页</Button>
            <span>第 {page} / {pages} 页</span>
            <Button variant="outline" size="sm" disabled={page >= pages}
              onClick={() => setQuery({ ...query, start: page * query.length })}>下一页</Button>
          </div>
        </div>
      </CardContent>

      {/* 滚动执行日志 */}
      <Dialog open={!!viewLog} onOpenChange={(o) => !o && setViewLog(null)}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Terminal className="size-4" />
              执行日志 #{viewLog?.id}
            </DialogTitle>
            <DialogDescription>
              任务 #{viewLog?.jobId} · {viewLog?.executorHandler || '-'} · {viewLog?.executorAddress}
            </DialogDescription>
          </DialogHeader>
          <pre className={cn(
            'max-h-[60vh] overflow-auto rounded-md bg-zinc-950 p-3 text-xs leading-relaxed text-green-400',
          )}>
            {logText || '（暂无日志输出）'}
          </pre>
          <div className="text-muted-foreground text-xs">
            {logEnded ? '日志已全部加载' : `正在滚动读取... 当前第 ${fromLine} 行`}
          </div>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

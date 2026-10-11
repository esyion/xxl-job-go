import { useCallback, useEffect, useState } from 'react'
import dayjs from 'dayjs'
import { MoreHorizontal, Plus } from 'lucide-react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription,
  AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  Pagination, PaginationContent, PaginationItem, PaginationNext, PaginationPrevious,
} from '@/components/ui/pagination'
import { Skeleton } from '@/components/ui/skeleton'
import {
  apiGroupPageList, apiJobAdd, apiJobPageList, apiJobRemove, apiJobStart, apiJobStop,
  apiJobTrigger, apiJobUpdate, apiNextTriggerTime, apiGlueSave, assertOk,
  BLOCK_OPTIONS, GLUE_OPTIONS, JobGroup, JobInfo, MISFIRE_OPTIONS, ROUTE_OPTIONS,
} from '@/api'

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
  const [nextTimes, setNextTimes] = useState<string[]>([])
  const [glueJob, setGlueJob] = useState<JobInfo | null>(null)
  const [glueSource, setGlueSource] = useState('')
  const [triggerJob, setTriggerJob] = useState<JobInfo | null>(null)
  const [triggerParam, setTriggerParam] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<JobInfo | null>(null)
  const [confirmTrigger, setConfirmTrigger] = useState<JobInfo | null>(null)

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
  const page = query.start / query.length + 1
  const pages = Math.max(1, Math.ceil(total / query.length))

  const openEdit = (job?: JobInfo) => {
    setNextTimes([])
    setEditing(job ? { ...job } : emptyJob())
  }

  const previewNextTime = async () => {
    const st = editing?.scheduleType
    const sc = editing?.scheduleConf
    if (!st || st === 'NONE' || !sc) { setNextTimes([]); return }
    const ret = await apiNextTriggerTime(st, sc)
    if (ret.code === 200) setNextTimes(ret.content || [])
    else { setNextTimes([]); toast.error(ret.msg) }
  }

  const setField = (k: keyof JobInfo, v: any) => setEditing((prev) => ({ ...(prev || {}), [k]: v }))

  const saveJob = async () => {
    const e = editing || {}
    if (!e.jobGroup || !e.name) { toast.error('请填写执行器和任务名称'); return }
    const ret = e.id ? await apiJobUpdate(e as any) : await apiJobAdd(e)
    if (assertOk(ret, '保存成功')) { setEditing(null); load() }
  }

  const toggleStatus = async (job: JobInfo) => {
    const ret = job.triggerStatus === 1 ? await apiJobStop(job.id) : await apiJobStart(job.id)
    if (assertOk(ret, job.triggerStatus === 1 ? '已停止' : '已启动')) load()
  }

  return (
    <Card>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Select value={query.jobGroup > 0 ? String(query.jobGroup) : 'all'} onValueChange={(v) =>
            setQuery({ ...query, jobGroup: v === 'all' ? -1 : Number(v), start: 0 })}>
            <SelectTrigger className="w-44"><SelectValue placeholder="执行器" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部执行器</SelectItem>
              {groups.map((g) => <SelectItem key={g.id} value={String(g.id)}>{g.name}</SelectItem>)}
            </SelectContent>
          </Select>
          <Input placeholder="任务名称" className="w-40" onChange={(e) => setQuery({ ...query, name: e.target.value, start: 0 })} />
          <Input placeholder="JobHandler" className="w-40" onChange={(e) => setQuery({ ...query, executorHandler: e.target.value, start: 0 })} />
          <Select value={query.triggerStatus >= 0 ? String(query.triggerStatus) : 'all'} onValueChange={(v) =>
            setQuery({ ...query, triggerStatus: v === 'all' ? -1 : Number(v), start: 0 })}>
            <SelectTrigger className="w-28"><SelectValue placeholder="状态" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部状态</SelectItem>
              <SelectItem value="1">运行</SelectItem>
              <SelectItem value="0">停止</SelectItem>
            </SelectContent>
          </Select>
          <Button onClick={() => openEdit()} className="ml-auto"><Plus />新增任务</Button>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12">ID</TableHead>
              <TableHead className="min-w-36">任务名称</TableHead>
              <TableHead className="min-w-44">调度</TableHead>
              <TableHead className="min-w-28">执行器</TableHead>
              <TableHead className="min-w-28">JobHandler</TableHead>
              <TableHead className="min-w-36">路由策略</TableHead>
              <TableHead className="w-20">状态</TableHead>
              <TableHead className="min-w-36">下次触发</TableHead>
              <TableHead className="w-16">操作</TableHead>
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
                  <TableCell className="max-w-48 truncate font-medium" title={r.name}>{r.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">{r.scheduleType}</Badge>
                    <span className="text-muted-foreground ml-1 font-mono text-xs">{r.scheduleConf}</span>
                  </TableCell>
                  <TableCell className="max-w-32 truncate">{groupName(r.jobGroup)}</TableCell>
                  <TableCell className="max-w-32 truncate font-mono text-xs">{r.executorHandler || '-'}</TableCell>
                  <TableCell className="text-xs">{r.executorRouteStrategy}</TableCell>
                  <TableCell>
                    <Switch checked={r.triggerStatus === 1} onCheckedChange={() => toggleStatus(r)} />
                  </TableCell>
                  <TableCell className="text-xs">{r.triggerNextTime > 0 ? dayjs(r.triggerNextTime).format('MM-DD HH:mm:ss') : '-'}</TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="size-8"><MoreHorizontal /></Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => { setTriggerJob(r); setTriggerParam('') }}>执行一次</DropdownMenuItem>
                        <DropdownMenuItem onClick={() => openEdit(r)}>编辑</DropdownMenuItem>
                        {r.glueType !== 'BEAN' && (
                          <DropdownMenuItem onClick={() => { setGlueJob(r); setGlueSource(r.glueSource || '') }}>GLUE 编辑</DropdownMenuItem>
                        )}
                        <DropdownMenuSeparator />
                        <DropdownMenuItem variant="destructive" onClick={() => setConfirmDelete(r)}>删除</DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>

        <div className="flex items-center justify-between">
          <span className="text-muted-foreground text-sm">共 {total} 条</span>
          <Pagination>
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  href="#"
                  aria-disabled={page <= 1}
                  className={page <= 1 ? 'pointer-events-none opacity-50' : ''}
                  onClick={(e) => { e.preventDefault(); setQuery({ ...query, start: (page - 2) * query.length }) }}
                />
              </PaginationItem>
              <PaginationItem className="text-sm leading-9">第 {page} / {pages} 页</PaginationItem>
              <PaginationItem>
                <PaginationNext
                  href="#"
                  aria-disabled={page >= pages}
                  className={page >= pages ? 'pointer-events-none opacity-50' : ''}
                  onClick={(e) => { e.preventDefault(); setQuery({ ...query, start: page * query.length }) }}
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </div>
      </CardContent>

      {/* 新增/编辑任务 */}
      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{editing?.id ? `编辑任务 #${editing.id}` : '新增任务'}</DialogTitle>
            <DialogDescription>任务的基础配置、调度与路由策略</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label>执行器</Label>
              <Select value={editing?.jobGroup ? String(editing.jobGroup) : undefined}
                onValueChange={(v) => setField('jobGroup', Number(v))}>
                <SelectTrigger><SelectValue placeholder="选择执行器" /></SelectTrigger>
                <SelectContent>
                  {groups.map((g) => <SelectItem key={g.id} value={String(g.id)}>{g.name}（{g.appName}）</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>任务名称</Label>
              <Input value={editing?.name || ''} onChange={(e) => setField('name', e.target.value)} placeholder="任务描述信息" />
            </div>
            <div className="grid gap-2">
              <Label>GLUE 类型</Label>
              <Select value={editing?.glueType || 'BEAN'} onValueChange={(v) => setField('glueType', v)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>{GLUE_OPTIONS.map((v) => <SelectItem key={v} value={v}>{v}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>JobHandler</Label>
              <Input value={editing?.executorHandler || ''} onChange={(e) => setField('executorHandler', e.target.value)}
                disabled={editing?.glueType !== 'BEAN'} placeholder="执行器侧注册的 handler 名" />
            </div>
            <div className="grid gap-2 sm:col-span-2">
              <Label>任务参数</Label>
              <Textarea rows={2} value={editing?.executorParam || ''} onChange={(e) => setField('executorParam', e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label>调度类型</Label>
              <Select value={editing?.scheduleType || 'CRON'} onValueChange={(v) => { setField('scheduleType', v); setNextTimes([]) }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>{SCHEDULE_TYPES.map((t) => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            {editing?.scheduleType !== 'NONE' && (
              <div className="grid gap-2">
                <Label>{editing?.scheduleType === 'CRON' ? 'Cron 表达式' : '固定速度（秒）'}</Label>
                <Input value={editing?.scheduleConf || ''} onChange={(e) => setField('scheduleConf', e.target.value)}
                  onBlur={previewNextTime} placeholder={editing?.scheduleType === 'CRON' ? '0 * * * * ?（秒 分 时 日 月 周）' : '30'} />
              </div>
            )}
            {nextTimes.length > 0 && (
              <div className="text-muted-foreground sm:col-span-2 text-sm">
                下次触发预览：<span className="text-primary">{nextTimes.join('  →  ')}</span>
              </div>
            )}
            <div className="grid gap-2">
              <Label>过期策略</Label>
              <Select value={editing?.misfireStrategy || 'DO_NOTHING'} onValueChange={(v) => setField('misfireStrategy', v)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>{MISFIRE_OPTIONS.map((t) => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>路由策略</Label>
              <Select value={editing?.executorRouteStrategy || 'FIRST'} onValueChange={(v) => setField('executorRouteStrategy', v)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>{ROUTE_OPTIONS.map((v) => <SelectItem key={v} value={v}>{v}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>阻塞处理</Label>
              <Select value={editing?.executorBlockStrategy || 'SERIAL_EXECUTION'} onValueChange={(v) => setField('executorBlockStrategy', v)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>{BLOCK_OPTIONS.map((t) => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>子任务 ID</Label>
              <Input value={editing?.childJobId || ''} onChange={(e) => setField('childJobId', e.target.value)} placeholder="多个逗号分隔，父任务成功后触发" />
            </div>
            <div className="grid gap-2">
              <Label>超时时间（秒）</Label>
              <Input type="number" min={0} value={editing?.executorTimeout ?? 0} onChange={(e) => setField('executorTimeout', Number(e.target.value))} />
            </div>
            <div className="grid gap-2">
              <Label>失败重试次数</Label>
              <Input type="number" min={0} value={editing?.executorFailRetryCount ?? 0} onChange={(e) => setField('executorFailRetryCount', Number(e.target.value))} />
            </div>
            <div className="grid gap-2">
              <Label>负责人</Label>
              <Input value={editing?.author || ''} onChange={(e) => setField('author', e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label>告警邮件</Label>
              <Input value={editing?.alarmEmail || ''} onChange={(e) => setField('alarmEmail', e.target.value)} placeholder="多个逗号分隔，留空不告警" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditing(null)}>取消</Button>
            <Button onClick={saveJob}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 执行一次确认 */}
      <AlertDialog open={!!confirmTrigger} onOpenChange={(o) => !o && setConfirmTrigger(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认执行一次？</AlertDialogTitle>
            <AlertDialogDescription>
              将手动触发任务「{confirmTrigger?.name}」（#{confirmTrigger?.id}），可在下一步填写本次参数。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={() => { setTriggerJob(confirmTrigger); setTriggerParam(''); setConfirmTrigger(null) }}>继续</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 执行一次参数 */}
      <Dialog open={!!triggerJob} onOpenChange={(o) => !o && setTriggerJob(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>执行一次 #{triggerJob?.id} {triggerJob?.name}</DialogTitle>
            <DialogDescription>本次执行的参数，留空使用任务配置参数</DialogDescription>
          </DialogHeader>
          <Textarea rows={3} value={triggerParam} onChange={(e) => setTriggerParam(e.target.value)} placeholder="本次执行的参数" />
          <DialogFooter>
            <Button variant="outline" onClick={() => setTriggerJob(null)}>取消</Button>
            <Button onClick={async () => {
              if (!triggerJob) return
              const ret = await apiJobTrigger(triggerJob.id, triggerParam)
              if (assertOk(ret, '已触发')) setTriggerJob(null)
            }}>触发</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 删除确认 */}
      <AlertDialog open={!!confirmDelete} onOpenChange={(o) => !o && setConfirmDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除任务？</AlertDialogTitle>
            <AlertDialogDescription>
              将删除任务「{confirmDelete?.name}」（#{confirmDelete?.id}），运行中的任务需先停止。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={async () => {
              if (!confirmDelete) return
              const ret = await apiJobRemove(confirmDelete.id)
              if (assertOk(ret, '已删除')) { setConfirmDelete(null); load() }
            }}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* GLUE 编辑 */}
      <Dialog open={!!glueJob} onOpenChange={(o) => !o && setGlueJob(null)}>
        <DialogContent className="sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>GLUE 源码编辑 #{glueJob?.id}（{glueJob?.glueType}）</DialogTitle>
            <DialogDescription>保存后，新触发的任务将使用新版源码</DialogDescription>
          </DialogHeader>
          <Textarea rows={18} className="font-mono text-xs" value={glueSource} onChange={(e) => setGlueSource(e.target.value)} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setGlueJob(null)}>取消</Button>
            <Button onClick={async () => {
              if (!glueJob) return
              const ret = await apiGlueSave(glueJob.id, glueSource, '页面编辑')
              if (assertOk(ret, 'GLUE 源码已保存')) setGlueJob(null)
            }}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

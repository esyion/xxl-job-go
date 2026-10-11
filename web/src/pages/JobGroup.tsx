import { useCallback, useEffect, useState } from 'react'
import dayjs from 'dayjs'
import { MoreHorizontal, Plus } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription,
  AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { apiGroupPageList, apiGroupRemove, apiGroupSave, assertOk, JobGroup } from '@/api'

export default function JobGroupPage() {
  const [list, setList] = useState<JobGroup[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [query, setQuery] = useState({ start: 0, length: 10, appName: '' })
  const [editing, setEditing] = useState<(Partial<JobGroup> & { accessToken?: string }) | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<JobGroup | null>(null)

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
    setEditing(
      g
        ? { ...g, accessToken: '' }
        : { appName: '', name: '', addressType: 0, addressList: '', accessToken: 'default_token' },
    )
  }

  const setField = (k: string, v: any) => setEditing((prev) => ({ ...(prev || {}), [k]: v }))

  const save = async () => {
    const g = editing || {}
    if (!g.appName || !g.name) return
    const payload = { ...g }
    if (!payload.accessToken) delete (payload as any).accessToken
    const ret = await apiGroupSave(payload as any)
    if (assertOk(ret, '保存成功')) { setEditing(null); load() }
  }

  const page = query.start / query.length + 1
  const pages = Math.max(1, Math.ceil(total / query.length))

  return (
    <Card>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Input placeholder="AppName" className="w-52" onChange={(e) => setQuery({ ...query, appName: e.target.value, start: 0 })} />
          <Button className="ml-auto" onClick={() => openEdit()}><Plus />新增执行器</Button>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12">ID</TableHead>
              <TableHead>AppName</TableHead>
              <TableHead>名称</TableHead>
              <TableHead className="w-24">注册方式</TableHead>
              <TableHead>机器地址</TableHead>
              <TableHead>更新时间</TableHead>
              <TableHead className="w-16">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading && list.length === 0 ? (
              [...Array(3)].map((_, i) => (
                <TableRow key={i}>
                  {[...Array(7)].map((_, j) => <TableCell key={j}><Skeleton className="h-5 w-full" /></TableCell>)}
                </TableRow>
              ))
            ) : (
              list.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>{r.id}</TableCell>
                  <TableCell className="font-mono text-xs">{r.appName}</TableCell>
                  <TableCell>{r.name}</TableCell>
                  <TableCell>
                    {r.addressType === 0 ? (
                      <Badge variant="outline" className="text-green-600">自动注册</Badge>
                    ) : (
                      <Badge variant="secondary">手动录入</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    {r.addressList ? (
                      r.addressList.split(',').map((a) => <div key={a} className="font-mono text-xs">{a}</div>)
                    ) : (
                      <span className="text-muted-foreground text-xs">暂无注册机器</span>
                    )}
                  </TableCell>
                  <TableCell className="text-xs">{r.updateTime && r.updateTime !== '0001-01-01T00:00:00Z' ? dayjs(r.updateTime).format('YYYY-MM-DD HH:mm') : '-'}</TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="size-8"><MoreHorizontal /></Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => openEdit(r)}>编辑</DropdownMenuItem>
                        <DropdownMenuItem variant="destructive" onClick={() => setConfirmDelete(r)}>删除</DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
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

      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editing?.id ? `编辑执行器 #${editing.id}` : '新增执行器'}</DialogTitle>
            <DialogDescription>AppName 需与执行器侧配置一致</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="grid gap-2">
              <Label>AppName</Label>
              <Input value={editing?.appName || ''} onChange={(e) => setField('appName', e.target.value)}
                disabled={!!editing?.id} placeholder="执行器唯一标识" />
            </div>
            <div className="grid gap-2">
              <Label>名称</Label>
              <Input value={editing?.name || ''} onChange={(e) => setField('name', e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label>注册方式</Label>
              <RadioGroup value={String(editing?.addressType ?? 0)} onValueChange={(v) => setField('addressType', Number(v))}
                className="flex gap-6">
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="0" id="rt-auto" />
                  <Label htmlFor="rt-auto">自动注册</Label>
                </div>
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="1" id="rt-manual" />
                  <Label htmlFor="rt-manual">手动录入</Label>
                </div>
              </RadioGroup>
            </div>
            {editing?.addressType === 1 && (
              <div className="grid gap-2">
                <Label>机器地址</Label>
                <Input value={editing?.addressList || ''} onChange={(e) => setField('addressList', e.target.value)}
                  placeholder="多个地址逗号分隔，如 http://127.0.0.1:9999" />
              </div>
            )}
            <div className="grid gap-2">
              <Label>AccessToken</Label>
              <Input value={editing?.accessToken || ''} onChange={(e) => setField('accessToken', e.target.value)}
                placeholder="留空保持不变；新执行器默认 default_token" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditing(null)}>取消</Button>
            <Button onClick={save}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!confirmDelete} onOpenChange={(o) => !o && setConfirmDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除执行器？</AlertDialogTitle>
            <AlertDialogDescription>该执行器下存在任务时将禁止删除。</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={async () => {
              if (!confirmDelete) return
              const ret = await apiGroupRemove(confirmDelete.id)
              if (assertOk(ret, '已删除')) { setConfirmDelete(null); load() }
            }}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}

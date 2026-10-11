import { useCallback, useEffect, useState } from 'react'
import { KeyRound, MoreHorizontal, Plus } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription,
  AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Combobox, ComboboxContent, ComboboxChip, ComboboxChips, ComboboxChipsInput,
  ComboboxEmpty, ComboboxItem, ComboboxList, ComboboxTrigger,
} from '@/components/ui/combobox'
import { Skeleton } from '@/components/ui/skeleton'
import { apiGroupPageList, apiUserPageList, apiUserRemove, apiUserSave, apiUpdatePwd, assertOk, JobGroup, JobUser } from '@/api'

export default function UserPage() {
  const [list, setList] = useState<JobUser[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [groups, setGroups] = useState<JobGroup[]>([])
  const [query, setQuery] = useState({ start: 0, length: 10, username: '', role: -1 })
  const [editing, setEditing] = useState<Partial<JobUser> | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<JobUser | null>(null)
  const [pwdOpen, setPwdOpen] = useState(false)
  const [pwd, setPwd] = useState('')

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

  useEffect(() => {
    apiGroupPageList({ start: 0, length: 1000 }).then((p) => setGroups(p.data || []))
  }, [])
  useEffect(() => { load() }, [load])

  // permission 字符串 <-> 选中 ID 数组
  const permValues: string[] = (editing?.permission || '')
    .split(',').map((s) => s.trim()).filter((s) => s !== '')
  const setPermValues = (ids: string[]) =>
    setEditing((prev) => ({ ...(prev || {}), permission: ids.map(Number).sort((a, b) => a - b).join(',') }))
  const groupName = (id: number) => groups.find((g) => g.id === id)?.name || `执行器#${id}`

  const openEdit = (u?: JobUser) => {
    setEditing(u ? { ...u, password: '' } : { username: '', password: '', role: 0, permission: '' })
  }

  const setField = (k: keyof JobUser, v: any) => setEditing((prev) => ({ ...(prev || {}), [k]: v }))

  const save = async () => {
    const u = editing || {}
    if (!u.username) { return }
    if (!u.id && (u.password || '').length < 6) { return }
    const payload = { ...u }
    if (!payload.password) delete payload.password
    const ret = await apiUserSave(payload)
    if (assertOk(ret, '保存成功')) { setEditing(null); load() }
  }

  const page = query.start / query.length + 1
  const pages = Math.max(1, Math.ceil(total / query.length))

  return (
    <Card>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Input placeholder="账号" className="w-40" onChange={(e) => setQuery({ ...query, username: e.target.value, start: 0 })} />
          <Select value={query.role >= 0 ? String(query.role) : 'all'} onValueChange={(v) =>
            setQuery({ ...query, role: v === 'all' ? -1 : Number(v), start: 0 })}>
            <SelectTrigger className="w-28"><SelectValue placeholder="角色" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部角色</SelectItem>
              <SelectItem value="1">管理员</SelectItem>
              <SelectItem value="0">普通用户</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="outline" onClick={() => { setPwdOpen(true); setPwd('') }}><KeyRound />修改我的密码</Button>
          <Button className="ml-auto" onClick={() => openEdit()}><Plus />新增用户</Button>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12">ID</TableHead>
              <TableHead>账号</TableHead>
              <TableHead className="w-24">角色</TableHead>
              <TableHead>权限（执行器ID）</TableHead>
              <TableHead className="w-16">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading && list.length === 0 ? (
              [...Array(3)].map((_, i) => (
                <TableRow key={i}>
                  {[...Array(5)].map((_, j) => <TableCell key={j}><Skeleton className="h-5 w-full" /></TableCell>)}
                </TableRow>
              ))
            ) : (
              list.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>{r.id}</TableCell>
                  <TableCell className="font-medium">{r.username}</TableCell>
                  <TableCell>
                    {r.role === 1 ? <Badge>管理员</Badge> : <Badge variant="secondary">普通用户</Badge>}
                  </TableCell>
                  <TableCell className="font-mono text-xs">{r.permission || '全部'}</TableCell>
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
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editing?.id ? `编辑用户 #${editing.id}` : '新增用户'}</DialogTitle>
            <DialogDescription>普通用户通过勾选执行器控制可访问范围；管理员不受限</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="grid gap-2">
              <Label>账号</Label>
              <Input value={editing?.username || ''} onChange={(e) => setField('username', e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label>密码</Label>
              <Input type="password" value={editing?.password || ''} onChange={(e) => setField('password', e.target.value)}
                placeholder={editing?.id ? '留空表示不修改' : '至少 6 位'} />
            </div>
            <div className="grid gap-2">
              <Label>角色</Label>
              <Select value={String(editing?.role ?? 0)} onValueChange={(v) => setField('role', Number(v))}>
                <SelectTrigger className="w-40"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">管理员</SelectItem>
                  <SelectItem value="0">普通用户</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {editing?.role === 0 && (
              <div className="grid gap-2">
                <Label>权限（可访问的执行器）</Label>
                <Combobox
                  multiple
                  value={permValues}
                  onValueChange={(v) => setPermValues(v as string[])}
                >
                  <ComboboxTrigger className="w-full">
                    <ComboboxChips>
                      {permValues.map((id) => (
                        <ComboboxChip key={id}>{groupName(Number(id))}</ComboboxChip>
                      ))}
                      <ComboboxChipsInput placeholder={permValues.length ? '搜索...' : '点击选择执行器'} />
                    </ComboboxChips>
                  </ComboboxTrigger>
                  <ComboboxContent>
                    <ComboboxList>
                      <ComboboxEmpty>无匹配执行器</ComboboxEmpty>
                      {groups.map((g) => (
                        <ComboboxItem key={g.id} value={String(g.id)}>
                          {g.name}（{g.appName}）
                        </ComboboxItem>
                      ))}
                    </ComboboxList>
                  </ComboboxContent>
                </Combobox>
                <p className="text-muted-foreground text-xs">不选择任何执行器表示拥有全部权限</p>
              </div>
            )}
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
            <AlertDialogTitle>确认删除用户？</AlertDialogTitle>
            <AlertDialogDescription>将删除用户「{confirmDelete?.username}」，当前登录用户不可删除。</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={async () => {
              if (!confirmDelete) return
              const ret = await apiUserRemove(confirmDelete.id)
              if (assertOk(ret, '已删除')) { setConfirmDelete(null); load() }
            }}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={pwdOpen} onOpenChange={setPwdOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>修改当前用户密码</DialogTitle>
          </DialogHeader>
          <div className="grid gap-2">
            <Label>新密码</Label>
            <Input type="password" value={pwd} onChange={(e) => setPwd(e.target.value)} placeholder="至少 6 位" />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPwdOpen(false)}>取消</Button>
            <Button onClick={async () => {
              if (pwd.length < 6) return
              const ret = await apiUpdatePwd(pwd)
              if (assertOk(ret, '密码已修改')) { setPwdOpen(false); setPwd('') }
            }}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

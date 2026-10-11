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
import { Skeleton } from '@/components/ui/skeleton'
import { apiUserPageList, apiUserRemove, apiUserSave, apiUpdatePwd, assertOk, JobUser } from '@/api'

export default function UserPage() {
  const [list, setList] = useState<JobUser[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
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

  useEffect(() => { load() }, [load])

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
            <DialogDescription>普通用户的权限为可访问的执行器 ID，逗号分隔，留空表示全部</DialogDescription>
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
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">管理员</SelectItem>
                  <SelectItem value="0">普通用户</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>权限（执行器 ID）</Label>
              <Input value={editing?.permission || ''} onChange={(e) => setField('permission', e.target.value)}
                placeholder="逗号分隔，如 1,2；留空为全部" />
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

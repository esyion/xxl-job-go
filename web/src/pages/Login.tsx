import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { GalleryVerticalEnd, Loader2 } from 'lucide-react'

import { cn } from '@/lib/utils'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import { apiLogin } from '@/api'

export default function Login() {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [userName, setUserName] = useState('admin')
  const [password, setPassword] = useState('')

  const onFinish = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!userName || !password) {
      toast.error('请输入账号和密码')
      return
    }
    setLoading(true)
    try {
      const ret = await apiLogin(userName, password)
      if (ret.code === 200) {
        toast.success('登录成功')
        navigate('/')
      } else {
        toast.error(ret.msg || '登录失败')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="bg-muted flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
      <div className="flex w-full max-w-sm flex-col gap-6">
        <a href="/" className="flex items-center gap-2 self-center font-medium">
          <div className="bg-primary text-primary-foreground flex aspect-square size-6 items-center justify-center rounded-md">
            <GalleryVerticalEnd className="size-4" />
          </div>
          XXL-JOB 任务调度中心
        </a>
        <div className={cn('flex flex-col gap-6')}>
          <Card>
            <CardHeader>
              <CardTitle>登录</CardTitle>
              <CardDescription>默认账号 admin / 123456</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={onFinish}>
                <div className="flex flex-col gap-6">
                  <div className="grid gap-3">
                    <Label htmlFor="userName">账号</Label>
                    <Input
                      id="userName"
                      placeholder="admin"
                      value={userName}
                      onChange={(e) => setUserName(e.target.value)}
                      required
                    />
                  </div>
                  <div className="grid gap-3">
                    <Label htmlFor="password">密码</Label>
                    <Input
                      id="password"
                      type="password"
                      placeholder="123456"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      required
                    />
                  </div>
                  <Button type="submit" className="w-full" disabled={loading}>
                    {loading && <Loader2 className="animate-spin" />}
                    登录
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}

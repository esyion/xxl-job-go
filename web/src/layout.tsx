import { useEffect, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { useNavigate } from 'react-router-dom'
import { PanelLeft, LogOut } from 'lucide-react'
import { toast } from 'sonner'

import { SidebarProvider, SidebarInset, SidebarTrigger } from '@/components/ui/sidebar'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { AppSidebar } from '@/components/app-sidebar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { apiLogout, apiUserInfo } from '@/api'

export default function Layout() {
  const navigate = useNavigate()
  const location = useLocation()
  const [userName, setUserName] = useState('')
  const [isAdmin, setIsAdmin] = useState(false)

  useEffect(() => {
    apiUserInfo().then((ret) => {
      if (ret.code === 200) {
        setUserName(ret.content.userName)
        setIsAdmin(ret.content.role === 1)
      }
    })
  }, [])

  const title =
    { '/': '运行报表', '/jobinfo': '任务管理', '/jobgroup': '执行器管理', '/joblog': '调度日志', '/user': '用户管理' }[
      location.pathname
    ] || 'XXL-JOB'

  const doLogout = async () => {
    await apiLogout()
    toast.success('退出成功')
    navigate('/login')
  }

  return (
    <SidebarProvider>
      <AppSidebar isAdmin={isAdmin} />
      <SidebarInset>
        <header className="flex h-14 shrink-0 items-center gap-2 border-b">
          <div className="flex flex-1 items-center gap-2 px-3">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
            <span className="text-sm font-medium">{title}</span>
          </div>
          <div className="px-3">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm">
                  {userName || '-'}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>{isAdmin ? '管理员' : '普通用户'}</DropdownMenuLabel>
                <DropdownMenuItem onClick={doLogout}>
                  <LogOut />
                  退出登录
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>
        <div className="flex flex-1 flex-col gap-4 p-4">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}

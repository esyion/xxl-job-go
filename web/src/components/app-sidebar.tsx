import {
  GalleryVerticalEnd,
  LayoutDashboard,
  Network,
  ScrollText,
  UsersRound,
} from 'lucide-react'
import { NavLink } from 'react-router-dom'

import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from '@/components/ui/sidebar'

const nav = [
  { title: '运行报表', url: '/', icon: LayoutDashboard, adminOnly: false },
  { title: '任务管理', url: '/jobinfo', icon: GalleryVerticalEnd, adminOnly: false },
  { title: '执行器管理', url: '/jobgroup', icon: Network, adminOnly: true },
  { title: '调度日志', url: '/joblog', icon: ScrollText, adminOnly: false },
  { title: '用户管理', url: '/user', icon: UsersRound, adminOnly: true },
]

export function AppSidebar({ isAdmin, ...props }: { isAdmin?: boolean } & React.ComponentProps<typeof Sidebar>) {
  return (
    <Sidebar {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <a href="/">
                <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                  <GalleryVerticalEnd className="size-4" />
                </div>
                <div className="flex flex-col gap-0.5 leading-none">
                  <span className="font-medium">XXL-JOB</span>
                  <span className="">任务调度中心</span>
                </div>
              </a>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>导航</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {nav
                .filter((item) => !item.adminOnly || isAdmin)
                .map((item) => (
                  <SidebarMenuItem key={item.url}>
                    <SidebarMenuButton asChild tooltip={item.title}>
                      <NavLink to={item.url}>
                        <item.icon />
                        <span>{item.title}</span>
                      </NavLink>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarRail />
    </Sidebar>
  )
}

import { useEffect, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { Layout as AntLayout, Menu, Dropdown, message } from 'antd'
import {
  DashboardOutlined, PartitionOutlined, ProfileOutlined, UserOutlined, LogoutOutlined,
} from '@ant-design/icons'
import { apiLogout, apiUserInfo } from './api'

const { Sider, Header, Content } = AntLayout

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

  const selected = '/' + (location.pathname.split('/')[1] || '')

  const items = [
    { key: '/', icon: <DashboardOutlined />, label: '运行报表' },
    { key: '/jobinfo', icon: <PartitionOutlined />, label: '任务管理' },
    ...(isAdmin ? [{ key: '/jobgroup', icon: <ProfileOutlined />, label: '执行器管理' }] : []),
    { key: '/joblog', icon: <ProfileOutlined />, label: '调度日志' },
    ...(isAdmin ? [{ key: '/user', icon: <UserOutlined />, label: '用户管理' }] : []),
  ]

  const doLogout = async () => {
    await apiLogout()
    message.success('退出成功')
    navigate('/login')
  }

  return (
    <AntLayout style={{ minHeight: '100vh' }}>
      <Sider theme="dark" width={200}>
        <div style={{ color: '#fff', fontSize: 18, fontWeight: 700, padding: '18px 24px' }}>XXL-JOB</div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selected]}
          items={items}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <AntLayout>
        <Header style={{
          background: '#fff', padding: '0 24px', display: 'flex',
          justifyContent: 'flex-end', alignItems: 'center', borderBottom: '1px solid #f0f0f0',
        }}>
          <Dropdown
            menu={{
              items: [
                { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', onClick: doLogout },
              ],
            }}
          >
            <span style={{ cursor: 'pointer' }}>
              <UserOutlined /> {userName || '-'}
            </span>
          </Dropdown>
        </Header>
        <Content style={{ margin: 16 }}>
          <Outlet />
        </Content>
      </AntLayout>
    </AntLayout>
  )
}

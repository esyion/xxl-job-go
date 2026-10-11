import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Form, Input, message } from 'antd'
import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { apiLogin } from '../api'

export default function Login() {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)

  const onFinish = async (values: { userName: string; password: string }) => {
    setLoading(true)
    try {
      const ret = await apiLogin(values.userName, values.password)
      if (ret.code === 200) {
        message.success('登录成功')
        navigate('/')
      } else {
        message.error(ret.msg || '登录失败')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      height: '100vh', display: 'flex', justifyContent: 'center', alignItems: 'center',
      background: 'linear-gradient(135deg, #1c92d2 0%, #f2fcfe 100%)',
    }}>
      <Card title="XXL-JOB 任务调度中心" style={{ width: 380 }} headStyle={{ textAlign: 'center', fontSize: 18 }}>
        <Form onFinish={onFinish} initialValues={{ userName: 'admin' }}>
          <Form.Item name="userName" rules={[{ required: true, message: '请输入账号' }]}>
            <Input prefix={<UserOutlined />} placeholder="账号（默认 admin）" size="large" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码（默认 123456）" size="large" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block size="large" loading={loading}>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}

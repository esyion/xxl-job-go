import { Navigate, Route, Routes } from 'react-router-dom'
import Layout from '@/layout'
import Login from '@/pages/Login'
import Dashboard from '@/pages/Dashboard'
import JobInfo from '@/pages/JobInfo'
import JobGroup from '@/pages/JobGroup'
import JobLog from '@/pages/JobLog'
import User from '@/pages/User'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<Dashboard />} />
        <Route path="jobinfo" element={<JobInfo />} />
        <Route path="jobgroup" element={<JobGroup />} />
        <Route path="joblog" element={<JobLog />} />
        <Route path="user" element={<User />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

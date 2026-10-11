import axios from 'axios'
import { toast } from 'sonner'

const http = axios.create({ timeout: 60_000 })

// 统一处理登录态失效
http.interceptors.response.use((resp) => {
  const data = resp.data
  if (data && typeof data === 'object' && data.code === 500 && typeof data.msg === 'string' && data.msg.includes('登录态')) {
    window.location.href = '/login'
  }
  return resp
})

export interface ReturnT<T = any> {
  code: number
  msg: string
  content: T
}

export interface PageResult<T = any> {
  recordsTotal: number
  data: T[]
}

export interface JobInfo {
  id: number
  jobGroup: number
  name: string
  author: string
  alarmEmail: string
  scheduleType: string
  scheduleConf: string
  misfireStrategy: string
  executorRouteStrategy: string
  executorHandler: string
  executorParam: string
  executorBlockStrategy: string
  executorTimeout: number
  executorFailRetryCount: number
  glueType: string
  glueSource?: string
  glueRemark: string
  childJobId: string
  triggerStatus: number
  triggerLastTime: number
  triggerNextTime: number
}

export interface JobGroup {
  id: number
  appName: string
  name: string
  addressType: number
  addressList: string
  updateTime: string
}

export interface JobLog {
  id: number
  jobGroup: number
  jobId: number
  executorAddress: string
  executorHandler: string
  executorParam: string
  executorShardingParam: string
  executorFailRetryCount: number
  triggerTime: string
  triggerCode: number
  triggerMsg: string
  handleTime: string
  handleCode: number
  handleMsg: string
  alarmStatus: number
}

export interface JobUser {
  id: number
  username: string
  password?: string
  role: number
  permission: string
}

export interface LogReport {
  triggerDay: string
  runningCount: number
  sucCount: number
  failCount: number
}

async function post<T = any>(url: string, body?: any): Promise<ReturnT<T>> {
  const resp = await http.post(url, body || {})
  return resp.data
}

async function get<T = any>(url: string, params?: any): Promise<ReturnT<T>> {
  const resp = await http.get(url, { params })
  return resp.data
}

async function postPage<T = any>(url: string, body?: any): Promise<PageResult<T>> {
  const resp = await http.post(url, body || {})
  return resp.data
}

// ============ 登录 ============
export const apiLogin = (userName: string, password: string) => post('/auth/login', { userName, password })
export const apiLogout = () => post('/auth/logout')
export const apiUserInfo = () => get<{ userName: string; role: number }>('/auth/user')
export const apiUpdatePwd = (password: string) => post('/auth/updatePwd', { password })

// ============ 任务 ============
export const apiJobPageList = (params: any) => postPage<JobInfo>('/jobinfo/pageList', { ...params, length: params.length || 10 })
export const apiJobAdd = (job: Partial<JobInfo>) => post('/jobinfo/add', job)
export const apiJobUpdate = (job: Partial<JobInfo> & { id: number }) => post('/jobinfo/update', job)
export const apiJobRemove = (id: number) => post('/jobinfo/remove', { id })
export const apiJobStart = (id: number) => post('/jobinfo/start', { id })
export const apiJobStop = (id: number) => post('/jobinfo/stop', { id })
export const apiJobTrigger = (id: number, executorParam?: string, addressList?: string) =>
  post('/jobinfo/trigger', { id, executorParam, addressList })
export const apiNextTriggerTime = (scheduleType: string, scheduleConf: string) =>
  get<string[]>('/jobinfo/nextTriggerTime', { scheduleType, scheduleConf })
export const apiGlueSave = (jobId: number, glueSource: string, glueRemark: string) =>
  post('/jobcode/save', { jobId, glueSource, glueRemark })

// ============ 执行器 ============
export const apiGroupPageList = (params: any) => postPage<JobGroup>('/jobgroup/pageList', { ...params, length: params.length || 10 })
export const apiGroupSave = (g: Partial<JobGroup> & { accessToken?: string }) => post('/jobgroup/save', g)
export const apiGroupRemove = (id: number) => post('/jobgroup/remove', { id })

// ============ 日志 ============
export const apiLogPageList = (params: any) => postPage<JobLog>('/joblog/pageList', { ...params, length: params.length || 10 })
export const apiLogDetailCat = (logId: number, fromLineNum: number) =>
  post<{ fromLineNum: number; toLineNum: number; logContent: string; isEnd: boolean }>('/joblog/logDetailCat', { logId, fromLineNum })
export const apiLogKill = (id: number) => post('/joblog/logKill', { id })
export const apiLogClear = (jobGroup: number, jobId: number, type: number) =>
  post('/joblog/clearLog', { jobGroup, jobId, type })

// ============ 用户 ============
export const apiUserPageList = (params: any) => postPage<JobUser>('/user/pageList', { ...params, length: params.length || 10 })
export const apiUserSave = (u: Partial<JobUser>) => post('/user/save', u)
export const apiUserRemove = (id: number) => post('/user/remove', { id })

// ============ 报表 ============
export const apiChartInfo = () =>
  get<{ reportList: LogReport[]; jobInfoCount: number; runningJobCount: number }>('/index/chartInfo')

// ============ 工具 ============
export function assertOk(ret: ReturnT, okMsg?: string): boolean {
  if (ret.code === 200) {
    if (okMsg) toast.success(okMsg)
    return true
  }
  toast.error(ret.msg || '操作失败')
  return false
}

export const ROUTE_OPTIONS = [
  'FIRST', 'LAST', 'ROUND', 'RANDOM', 'CONSISTENT_HASH',
  'LEAST_FREQUENTLY_USED', 'LEAST_RECENTLY_USED', 'FAILOVER', 'BUSYOVER', 'SHARDING_BROADCAST',
]

export const GLUE_OPTIONS = ['BEAN', 'GLUE_SHELL', 'GLUE_PYTHON', 'GLUE_NODEJS', 'GLUE_PHP', 'GLUE_POWERSHELL']

export const BLOCK_OPTIONS = [
  { value: 'SERIAL_EXECUTION', label: '单机串行' },
  { value: 'DISCARD_LATER', label: '丢弃后续调度' },
  { value: 'COVER_EARLY', label: '覆盖之前调度' },
]

export const MISFIRE_OPTIONS = [
  { value: 'DO_NOTHING', label: '忽略' },
  { value: 'FIRE_ONCE_NOW', label: '立即执行一次' },
]

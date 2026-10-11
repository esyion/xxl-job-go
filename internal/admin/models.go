package admin

import (
	"strings"
	"time"
)

// ============ 8 张表模型，字段与 tables_xxl_job.sql 完全对齐 ============

type JobGroup struct {
	ID          int       `json:"id"`
	AppName     string    `json:"appName"`
	Name        string    `json:"name"`
	AddressType int       `json:"addressType"` // 0=自动注册 1=手动录入
	AddressList string    `json:"addressList"`
	AccessToken string    `json:"-"`
	UpdateTime  time.Time `json:"updateTime"`
}

// RegistryList 返回地址列表（自动注册时来自 registry 刷新进来的 address_list）。
func (g *JobGroup) RegistryList() []string {
	var list []string
	for _, a := range strings.Split(g.AddressList, ",") {
		if a = strings.TrimSpace(a); a != "" {
			list = append(list, a)
		}
	}
	return list
}

type JobRegistry struct {
	ID            int64     `json:"id"`
	RegistryGroup string    `json:"registryGroup"`
	RegistryKey   string    `json:"registryKey"`
	RegistryValue string    `json:"registryValue"`
	UpdateTime    time.Time `json:"updateTime"`
}

type JobInfo struct {
	ID        int    `json:"id"`
	JobGroup  int    `json:"jobGroup"`
	Name      string `json:"name"`
	Author    string `json:"author"`
	AlarmEmail string `json:"alarmEmail"`

	ScheduleType   string `json:"scheduleType"`   // NONE / CRON / FIX_RATE
	ScheduleConf   string `json:"scheduleConf"`
	MisfireStrategy string `json:"misfireStrategy"` // DO_NOTHING / FIRE_ONCE_NOW

	ExecutorRouteStrategy   string `json:"executorRouteStrategy"`
	ExecutorHandler         string `json:"executorHandler"`
	ExecutorParam           string `json:"executorParam"`
	ExecutorBlockStrategy   string `json:"executorBlockStrategy"`
	ExecutorTimeout         int    `json:"executorTimeout"`
	ExecutorFailRetryCount  int    `json:"executorFailRetryCount"`

	GlueType      string    `json:"glueType"`
	GlueSource    string    `json:"glueSource"`
	GlueRemark    string    `json:"glueRemark"`
	GlueUpdatetime time.Time `json:"glueUpdatetime"`
	ChildJobID    string    `json:"childJobId"`

	TriggerStatus   int   `json:"triggerStatus"` // 0-停止 1-运行
	TriggerLastTime int64 `json:"triggerLastTime"`
	TriggerNextTime int64 `json:"triggerNextTime"`

	AddTime    time.Time `json:"addTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type JobLog struct {
	ID           int64  `json:"id"`
	JobGroup     int    `json:"jobGroup"`
	JobID        int    `json:"jobId"`
	ExecutorAddress string `json:"executorAddress"`
	ExecutorHandler string `json:"executorHandler"`
	ExecutorParam   string `json:"executorParam"`
	ExecutorShardingParam string `json:"executorShardingParam"`
	ExecutorFailRetryCount int `json:"executorFailRetryCount"`

	TriggerTime time.Time `json:"triggerTime"`
	TriggerCode int       `json:"triggerCode"`
	TriggerMsg  string    `json:"triggerMsg"`

	HandleTime time.Time `json:"handleTime"`
	HandleCode int       `json:"handleCode"`
	HandleMsg  string    `json:"handleMsg"`

	AlarmStatus int `json:"alarmStatus"`
}

type JobLogGlue struct {
	ID         int       `json:"id"`
	JobID      int       `json:"jobId"`
	GlueType   string    `json:"glueType"`
	GlueSource string    `json:"glueSource"`
	GlueRemark string    `json:"glueRemark"`
	AddTime    time.Time `json:"addTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type JobLogReport struct {
	ID           int       `json:"id"`
	TriggerDay   time.Time `json:"triggerDay"`
	RunningCount int       `json:"runningCount"`
	SucCount     int       `json:"sucCount"`
	FailCount    int       `json:"failCount"`
	UpdateTime   time.Time `json:"updateTime"`
}

type JobUser struct {
	ID         int    `json:"id"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Role       int    `json:"role"` // 0-普通用户 1-管理员
	Permission string `json:"permission"`
}

// ============ 常量 ============

const (
	TriggerStatusStopped = 0
	TriggerStatusRunning = 1

	RoleNormal = 0
	RoleAdmin  = 1
)

// Misfire 策略
const (
	MisfireDoNothing    = "DO_NOTHING"
	MisfireFireOnceNow  = "FIRE_ONCE_NOW"
)

// 调度类型
const (
	ScheduleNone    = "NONE"
	ScheduleCron    = "CRON"
	ScheduleFixRate = "FIX_RATE"
)

// 触发类型，对应 TriggerTypeEnum
type TriggerType string

const (
	TriggerCron   TriggerType = "CRON"
	TriggerManual TriggerType = "MANUAL"
	TriggerRetry  TriggerType = "RETRY"
	TriggerParent TriggerType = "PARENT"
	TriggerApi    TriggerType = "API"
)

func (t TriggerType) Title() string {
	switch t {
	case TriggerCron:
		return "Cron触发"
	case TriggerManual:
		return "手动触发"
	case TriggerRetry:
		return "失败重试触发"
	case TriggerParent:
		return "父任务触发"
	case TriggerApi:
		return "API触发"
	}
	return string(t)
}

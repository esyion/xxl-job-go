package xxl

// 通信协议常量，与 xxl-job-core Const 对齐。
const (
	HeaderAccessToken = "XXL-JOB-ACCESS-TOKEN"
	HeaderAppname     = "XXL-JOB-APPNAME"

	BeatTimeoutSec  = 30  // 心跳周期(秒)
	DeadTimeoutSec  = 90  // 注册失效阈值(秒) = BeatTimeout*3
	HTTPTimeoutSec  = 10
	RollLogSize     = 1000 // 滚动日志单次读取行数
	HandleMsgMaxLen = 50000
)

// ============ admin -> executor 请求 DTO ============

type TriggerRequest struct {
	JobID                int    `json:"jobId"`
	ExecutorHandler      string `json:"executorHandler"`
	ExecutorParams       string `json:"executorParams"`
	ExecutorBlockStrategy string `json:"executorBlockStrategy"`
	ExecutorTimeout      int    `json:"executorTimeout"`

	LogID       int64 `json:"logId"`
	LogDateTime int64 `json:"logDateTime"`

	GlueType     string `json:"glueType"`
	GlueSource   string `json:"glueSource"`
	GlueUpdatetime int64 `json:"glueUpdatetime"`

	BroadcastIndex int `json:"broadcastIndex"`
	BroadcastTotal int `json:"broadcastTotal"`
}

type IdleBeatRequest struct {
	JobID int `json:"jobId"`
}

type KillRequest struct {
	JobID int `json:"jobId"`
}

type LogRequest struct {
	LogID       int64 `json:"logId"`
	LogDateTime int64 `json:"logDateTime"`
	FromLineNum int   `json:"fromLineNum"`
}

type LogData struct {
	FromLineNum int    `json:"fromLineNum"`
	ToLineNum   int    `json:"toLineNum"`
	LogContent  string `json:"logContent"`
	IsEnd       bool   `json:"isEnd"`
}

// ============ executor -> admin 请求 DTO ============

type CallbackData struct {
	LogID       int64  `json:"logId"`
	LogDateTime int64  `json:"logDateTime"`
	HandleCode  int    `json:"handleCode"`
	HandleMsg   string `json:"handleMsg"`
}

type CallbackRequest struct {
	CallbackList []CallbackData `json:"callbackList"`
}

type RegistryRequest struct {
	RegistryGroup string `json:"registryGroup"`
	RegistryKey   string `json:"registryKey"`
	RegistryValue string `json:"registryValue"`
}

// ============ 阻塞策略 / GLUE 类型枚举 ============

const (
	BlockSerialExecution = "SERIAL_EXECUTION"
	BlockDiscardLater    = "DISCARD_LATER"
	BlockCoverEarly      = "COVER_EARLY"
)

const (
	GlueBean      = "BEAN"
	GlueGroovy    = "GLUE_GROOVY"
	GlueShell     = "GLUE_SHELL"
	GluePython    = "GLUE_PYTHON"
	GlueNodejs    = "GLUE_NODEJS"
	GluePhp       = "GLUE_PHP"
	GluePowershell = "GLUE_POWERSHELL"
)

func IsScriptGlue(glueType string) bool {
	switch glueType {
	case GlueShell, GluePython, GlueNodejs, GluePhp, GluePowershell:
		return true
	}
	return false
}

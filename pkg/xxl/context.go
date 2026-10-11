package xxl

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Context 对应 Java 的 XxlJobContext + XxlJobHelper，单次任务执行内传递。
type Context struct {
	JobID         int
	JobParam      string
	LogID         int64
	LogDateTime   int64
	logFileName   string
	BroadcastIndex int
	BroadcastTotal int

	mu         sync.Mutex
	handleCode int    // 0=未设置, 200=成功, 500=失败, 502=超时
	handleMsg  string
}

func newContext(req *TriggerRequest, logFileName string) *Context {
	return &Context{
		JobID:          req.JobID,
		JobParam:       req.ExecutorParams,
		LogID:          req.LogID,
		LogDateTime:    req.LogDateTime,
		logFileName:    logFileName,
		BroadcastIndex: req.BroadcastIndex,
		BroadcastTotal: req.BroadcastTotal,
	}
}

// Log 写执行日志（落盘到执行日志文件 + stdout）。
func (c *Context) Log(args ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	msg := fmt.Sprint(args...)
	line := fmt.Sprintf("%s [%d] %s\n", time.Now().Format("2006-01-02 15:04:05"), c.JobID, msg)
	appendToFile(c.logFileName, line)
	fmt.Print(line)
}

// Logf 格式化写执行日志。
func (c *Context) Logf(format string, args ...interface{}) {
	c.Log(fmt.Sprintf(format, args...))
}

// LogError 写异常堆栈日志。
func (c *Context) LogError(err error) {
	c.Log(fmt.Sprintf("<br>----------- xxl-job job execute error -----------<br>%v", err))
}

// HandleSuccess 标记执行成功。
func (c *Context) HandleSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handleCode = CodeSuccess
}

// HandleSuccessMsg 标记执行成功并携带消息。
func (c *Context) HandleSuccessMsg(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handleCode = CodeSuccess
	c.handleMsg = msg
}

// HandleFail 标记执行失败。
func (c *Context) HandleFail(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handleCode = CodeFail
	c.handleMsg = msg
}

// HandleTimeout 标记执行超时。
func (c *Context) HandleTimeout(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handleCode = CodeTimeout
	c.handleMsg = msg
}

func (c *Context) result() (int, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.handleCode, c.handleMsg
}

// appendToFile 追加写入日志文件（自动建目录）。
func appendToFile(fileName, content string) {
	if fileName == "" {
		return
	}
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(content)
}

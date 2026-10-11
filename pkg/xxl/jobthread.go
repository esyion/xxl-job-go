package xxl

import (
	"sync"
	"sync/atomic"
	"time"
)

// JobHandler 对应 Java IJobHandler；可选实现 Init()/Destroy() 的接口会被识别。
type JobHandler interface {
	Execute(ctx *Context)
}

// JobFunc 函数式 handler 适配器。
type JobFunc func(ctx *Context)

func (f JobFunc) Execute(ctx *Context) { f(ctx) }

type fullHandler interface {
	JobHandler
	Init()
	Destroy()
}

// jobThread 对应 Java JobThread：每个 jobId 一个 goroutine + 触发队列。
type jobThread struct {
	ex         *Executor
	jobID      int
	handlerName    string // BEAN 模式 handler 名（用于变更检测）
	glueUpdatetime int64 // 脚本/GROOVY 模式源码版本（用于变更检测）
	handler    JobHandler

	queue   chan *TriggerRequest
	logIDs  sync.Map // map[int64]struct{}，防同一 logId 重复触发
	toStop  atomic.Bool
	running atomic.Bool
	stopReason string
}

func newJobThread(ex *Executor, jobID int, handler JobHandler, handlerName string, glueUpdatetime int64) *jobThread {
	return &jobThread{
		ex:             ex,
		jobID:          jobID,
		handler:        handler,
		handlerName:    handlerName,
		glueUpdatetime: glueUpdatetime,
		queue:          make(chan *TriggerRequest, 4096),
	}
}

// pushTriggerQueue 入队一次触发，logId 去重。
func (t *jobThread) pushTriggerQueue(req *TriggerRequest) Response[string] {
	if _, loaded := t.logIDs.LoadOrStore(req.LogID, struct{}{}); loaded {
		return Fail[string]("repeat trigger job, logId:" + time.Now().Format("20060102150405"))
	}
	select {
	case t.queue <- req:
		return SuccessMsg[string]("success")
	default:
		t.logIDs.Delete(req.LogID)
		return Fail[string]("trigger queue is full, jobId:" + itoa(req.JobID))
	}
}

// isRunningOrHasQueue 阻塞策略判断依据。
func (t *jobThread) isRunningOrHasQueue() bool {
	return t.running.Load() || len(t.queue) > 0
}

// toStopWithReason 停止线程（安全重复调用）。
func (t *jobThread) toStopWithReason(reason string) {
	t.stopReason = reason
	t.toStop.Store(true)
}

// run 主循环：取触发请求 → 执行 → 回调；空闲 30 个周期(3s)自动回收。
func (t *jobThread) run() {
	if h, ok := t.handler.(fullHandler); ok {
		func() {
			defer func() { recover() }()
			h.Init()
		}()
	}

	idleTimes := 0
	for !t.toStop.Load() {
		select {
		case req := <-t.queue:
			t.running.Store(true)
			idleTimes = 0
			t.logIDs.Delete(req.LogID)
			t.executeOnce(req)
			t.running.Store(false)
		case <-time.After(3 * time.Second):
			idleTimes++
			if idleTimes > 30 && len(t.queue) == 0 {
				t.ex.removeJobThread(t.jobID, "excutor idle times over limit.")
				goto out
			}
		}
	}
out:
	// 队列中未执行的任务，直接回调失败
	for {
		select {
		case req := <-t.queue:
			t.ex.callbackChan <- CallbackData{
				LogID:       req.LogID,
				LogDateTime: req.LogDateTime,
				HandleCode:  CodeFail,
				HandleMsg:   t.stopReason + " [job not executed, in the job queue, killed.]",
			}
		default:
			goto destroy
		}
	}
destroy:
	if h, ok := t.handler.(fullHandler); ok {
		func() {
			defer func() { recover() }()
			h.Destroy()
		}()
	}
	exLog.Printf("xxl-job JobThread stoped, jobId:%d", t.jobID)
}

// executeOnce 执行一次任务（含超时控制与结果回调）。
func (t *jobThread) executeOnce(req *TriggerRequest) {
	logFileName := t.ex.appender.MakeLogFileName(req.LogDateTime, req.LogID)
	ctx := newContext(req, logFileName)

	ctx.Log("----------- xxl-job job execute start -----------\n----------- Param:" + ctx.JobParam)

	done := make(chan struct{})
	if req.ExecutorTimeout > 0 {
		go func() {
			defer func() { recover() }()
			defer close(done)
			t.handler.Execute(ctx)
		}()
		select {
		case <-done:
		case <-time.After(time.Duration(req.ExecutorTimeout) * time.Second):
			ctx.Log("----------- xxl-job job execute timeout")
			ctx.HandleTimeout("job execute timeout ")
			// Go 无法强杀业务 goroutine，超时后放弃等待（goroutine 自行结束）
		}
	} else {
		func() {
			defer func() {
				if r := recover(); r != nil {
					ctx.HandleFail(panicText(r))
				}
			}()
			t.handler.Execute(ctx)
		}()
	}

	code, msg := ctx.result()
	if code <= 0 {
		code, msg = CodeFail, "job handle result lost."
	}
	if len(msg) > HandleMsgMaxLen {
		msg = msg[:HandleMsgMaxLen] + "..."
	}
	ctx.Logf("----------- xxl-job job execute end(finish) -----------\n----------- Result: handleCode=%d, handleMsg = %s", code, msg)

	if t.toStop.Load() {
		t.ex.pushCallback(CallbackData{
			LogID: req.LogID, LogDateTime: req.LogDateTime,
			HandleCode: CodeFail, HandleMsg: t.stopReason + " [job running, killed]",
		})
		return
	}
	t.ex.pushCallback(CallbackData{
		LogID: req.LogID, LogDateTime: req.LogDateTime,
		HandleCode: code, HandleMsg: msg,
	})
}

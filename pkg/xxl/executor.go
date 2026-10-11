package xxl

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"xxljob-go/pkg/logx"
)

func itoa(i int) string        { return fmt.Sprintf("%d", i) }
func panicText(r any) string   { return fmt.Sprintf("job handler panic: %v", r) }

// ExecutorConfig 执行器配置，对应 XxlJobExecutor 的属性。
type ExecutorConfig struct {
	Appname         string   // 执行器 AppName（必填，与 admin 端执行器配置一致）
	AccessToken     string   // 通信令牌（默认 default_token）
	Address         string   // "ip:port" 形式注册地址；留空则自动探测
	Port            int      // 内嵌服务监听端口（Address 为空时必填），默认 9999
	AdminAddresses  []string // 调度中心地址列表
	LogPath         string   // 执行日志目录
	LogRetentionDays int     // 日志保留天数，-1 永久
	GlueEnabled     bool     // 是否支持 GLUE 任务
}

type Executor struct {
	cfg      ExecutorConfig
	appender *FileAppender

	handlers   map[string]JobHandler
	handlerMu  sync.RWMutex
	jobThreads sync.Map // map[int]*jobThread

	adminClients []*adminClient

	callbackChan chan CallbackData
	callbackStop chan struct{}
	callbackWG   sync.WaitGroup

	registryStop chan struct{}
	registryWG   sync.WaitGroup

	httpServer   *http.Server
	httpListener net.Listener
	serverWG     sync.WaitGroup

	addressOnce sync.Once
	addressVal  string
}

// NewExecutor 创建执行器。
func NewExecutor(cfg ExecutorConfig) *Executor {
	if cfg.AccessToken == "" {
		cfg.AccessToken = "default_token"
	}
	if cfg.Port == 0 && cfg.Address == "" {
		cfg.Port = 9999
	}
	if cfg.LogRetentionDays == 0 {
		cfg.LogRetentionDays = 30
	}
	return &Executor{
		cfg:          cfg,
		appender:     NewFileAppender(cfg.LogPath),
		handlers:     map[string]JobHandler{},
		callbackChan: make(chan CallbackData, 4096),
		callbackStop: make(chan struct{}),
		registryStop: make(chan struct{}),
	}
}

// RegHandler 注册函数式任务 handler。
func (e *Executor) RegHandler(name string, fn JobFunc) {
	e.RegJobHandler(name, fn)
}

// RegJobHandler 注册任务 handler（可实现 Init/Destroy 可选接口）。
func (e *Executor) RegJobHandler(name string, h JobHandler) {
	e.handlerMu.Lock()
	defer e.handlerMu.Unlock()
	e.handlers[name] = h
}

// Address 返回注册地址（http://ip:port），自动探测本机 IP。
func (e *Executor) Address() string {
	e.addressOnce.Do(func() {
		if e.cfg.Address != "" {
			e.addressVal = e.cfg.Address
			return
		}
		_, port, err := net.SplitHostPort(e.httpListenerAddr())
		if err != nil {
			port = itoa(e.cfg.Port)
		}
		e.addressVal = "http://" + net.JoinHostPort(localIP(), port)
	})
	return e.addressVal
}

func (e *Executor) httpListenerAddr() string {
	if e.httpListener != nil {
		return e.httpListener.Addr().String()
	}
	return fmt.Sprintf("0.0.0.0:%d", e.cfg.Port)
}

// Run 启动执行器（非阻塞）：注册、回调、日志清理、内嵌服务。
func (e *Executor) Run() error {
	if e.cfg.Appname == "" {
		return fmt.Errorf("executor appname is required")
	}
	if len(e.cfg.AdminAddresses) == 0 {
		return fmt.Errorf("executor adminAddresses is required")
	}
	httpClient := &http.Client{Timeout: HTTPTimeoutSec * time.Second}
	for _, addr := range e.cfg.AdminAddresses {
		e.adminClients = append(e.adminClients, &adminClient{
			address:     addr,
			appname:     e.cfg.Appname,
			accessToken: e.cfg.AccessToken,
			httpClient:  httpClient,
		})
	}

	if err := e.startEmbedServer(); err != nil {
		return err
	}
	e.startRegistryWorker()
	e.startCallbackWorker()
	e.startLogFileCleanWorker()
	logx.Infof(">>>>>>>>>>> xxl-job executor init success, appname:%s, address:%s", e.cfg.Appname, e.Address())
	return nil
}

// Stop 优雅停止：反注册、停回调、停服务。
func (e *Executor) Stop() {
	close(e.registryStop)
	e.registryWG.Wait()

	// 停止所有任务线程
	e.jobThreads.Range(func(key, value any) bool {
		t := value.(*jobThread)
		t.toStopWithReason("executor shutdown")
		return true
	})

	close(e.callbackStop)
	e.callbackWG.Wait()

	if e.httpServer != nil {
		_ = e.httpServer.Close()
	}
	e.serverWG.Wait()
	logx.Info(">>>>>>>>>>> xxl-job executor destroy success")
}

// startLogFileCleanWorker 每天清理过期日志目录。
func (e *Executor) startLogFileCleanWorker() {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			e.appender.CleanExpired(e.cfg.LogRetentionDays)
		}
	}()
}

func (e *Executor) pushCallback(data CallbackData) {
	select {
	case e.callbackChan <- data:
	default:
		logx.Infof("xxl-job callback queue full, logId:%d dropped", data.LogID)
	}
}

func (e *Executor) loadJobThread(jobID int) *jobThread {
	if v, ok := e.jobThreads.Load(jobID); ok {
		return v.(*jobThread)
	}
	return nil
}

// registJobThread 创建并启动任务线程。
func (e *Executor) registJobThread(jobID int, handler JobHandler, handlerName string, glueUpdatetime int64, removeOldReason string) *jobThread {
	t := newJobThread(e, jobID, handler, handlerName, glueUpdatetime)
	e.jobThreads.Store(jobID, t)
	go t.run()
	if removeOldReason != "" {
		logx.Infof(">>>>>>>>>>> xxl-job, running job thread remove, reason:%s", removeOldReason)
	}
	return t
}

// removeJobThread 移除并停止任务线程。
func (e *Executor) removeJobThread(jobID int, removeOldReason string) {
	if v, loaded := e.jobThreads.LoadAndDelete(jobID); loaded {
		v.(*jobThread).toStopWithReason(removeOldReason)
	}
}

// ============ ExecutorBiz 实现，对应 Java ExecutorBizImpl ============

func (e *Executor) beat() Response[string] {
	return Success[string]("")
}

func (e *Executor) idleBeat(req *IdleBeatRequest) Response[string] {
	if t := e.loadJobThread(req.JobID); t != nil && t.isRunningOrHasQueue() {
		return Fail[string]("job thread is running or has trigger queue.")
	}
	return Success[string]("")
}

// trigger 处理调度请求：解析 handler、处理阻塞策略、入队。
func (e *Executor) trigger(req *TriggerRequest) Response[string] {
	jobThread := e.loadJobThread(req.JobID)
	var handler JobHandler
	if jobThread != nil {
		handler = jobThread.handler
	}
	removeOldReason := ""
	glueType := req.GlueType

	// 非 BEAN 的 GLUE 需要执行器开启支持
	if glueType != GlueBean && !e.cfg.GlueEnabled {
		return Of[string](CodeFail, "fail, current glue type ["+glueType+"] not supported.")
	}

	switch {
	case glueType == GlueBean:
		e.handlerMu.RLock()
		newHandler, found := e.handlers[req.ExecutorHandler]
		e.handlerMu.RUnlock()
		if !found {
			return Of[string](CodeFail, "job handler ["+req.ExecutorHandler+"] not found.")
		}
		// handler 变更：杀旧建新（Go 函数值不可比较，用注册名判断变更）
		if jobThread != nil && jobThread.handlerName != req.ExecutorHandler {
			removeOldReason = "change jobhandler or glue type, and terminate the old job thread."
			e.removeJobThread(req.JobID, removeOldReason)
			jobThread, handler = nil, nil
		}
		if handler == nil {
			handler = newHandler
		}

	case IsScriptGlue(glueType):
		// 脚本源码变更：杀旧建新
		if jobThread != nil {
			sh, ok := handler.(*scriptJobHandler)
			if !ok || sh.glueUpdatetime != req.GlueUpdatetime {
				removeOldReason = "change script or glue type, and terminate the old job thread."
				e.removeJobThread(req.JobID, removeOldReason)
				jobThread, handler = nil, nil
			}
		}
		if handler == nil {
			handler = &scriptJobHandler{
				jobID:          req.JobID,
				glueUpdatetime: req.GlueUpdatetime,
				glueSource:     req.GlueSource,
				glueType:       req.GlueType,
				logPath:        e.appender.basePath,
			}
		}

	default:
		return Of[string](CodeFail, "glueType["+glueType+"] is not valid.")
	}

	// 阻塞策略
	if jobThread != nil {
		switch req.ExecutorBlockStrategy {
		case BlockDiscardLater:
			if jobThread.isRunningOrHasQueue() {
				return Of[string](CodeFail, "block strategy effect："+BlockDiscardLater)
			}
		case BlockCoverEarly:
			if jobThread.isRunningOrHasQueue() {
				removeOldReason = "block strategy effect：" + BlockCoverEarly
				e.removeJobThread(req.JobID, removeOldReason)
				jobThread = nil
			}
		default:
			// SERIAL_EXECUTION：直接排队
		}
	}

	// 新建线程（新任务或旧线程已失效）
	if jobThread == nil {
		handlerName := ""
		if glueType == GlueBean {
			handlerName = req.ExecutorHandler
		}
		jobThread = e.registJobThread(req.JobID, handler, handlerName, req.GlueUpdatetime, removeOldReason)
	}

	return jobThread.pushTriggerQueue(req)
}

func (e *Executor) kill(req *KillRequest) Response[string] {
	if t := e.loadJobThread(req.JobID); t != nil {
		e.removeJobThread(req.JobID, "scheduling center kill job.")
		return Success[string]("")
	}
	return SuccessMsg[string]("job thread already killed.")
}

func (e *Executor) log(req *LogRequest) Response[*LogData] {
	fileName := e.appender.MakeLogFileName(req.LogDateTime, req.LogID)
	return Success[*LogData](e.appender.ReadLog(fileName, req.FromLineNum))
}

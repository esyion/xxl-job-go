package admin

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"xxljob-go/pkg/xxl"
)

// triggerPool 对应 JobTriggerPoolHelper：fast/slow 双工作池，
// 1 分钟内超时(>500ms)超过 10 次的任务降级到 slow 池。
type triggerPool struct {
	store  *Store
	client *executorClient
	svc    *Service // 触发失败重试/子任务触发依赖回调处理

	fast chan func()
	slow chan func()

	minute          int64
	jobTimeoutCount sync.Map // jobID -> *int32

	stopOnce sync.Once
	stop     chan struct{}
	wg       sync.WaitGroup
}

func newTriggerPool(store *Store, client *executorClient, svc *Service) *triggerPool {
	return &triggerPool{
		store:  store,
		client: client,
		svc:    svc,
		fast:   make(chan func(), 2000),
		slow:   make(chan func(), 5000),
		stop:   make(chan struct{}),
	}
}

func (p *triggerPool) start(fastWorkers, slowWorkers int) {
	for i := 0; i < fastWorkers; i++ {
		p.wg.Add(1)
		go p.worker(p.fast)
	}
	for i := 0; i < slowWorkers; i++ {
		p.wg.Add(1)
		go p.worker(p.slow)
	}
}

func (p *triggerPool) worker(ch chan func()) {
	defer p.wg.Done()
	for {
		select {
		case fn := <-ch:
			p.runGuarded(fn)
		case <-p.stop:
			return
		}
	}
}

func (p *triggerPool) runGuarded(fn func()) {
	defer func() { recover() }()
	fn()
}

func (p *triggerPool) stopPool() {
	p.stopOnce.Do(func() { close(p.stop) })
	p.wg.Wait()
}

// trigger 提交一次触发（异步）。
// failRetryCount >=0 用传入值，<0 用任务配置；shardingParam 非空用于重试单分片；
// executorParam 非空覆盖任务参数；addressList 非空覆盖执行器地址。
func (p *triggerPool) trigger(jobID int, triggerType TriggerType,
	failRetryCount int, shardingParam, executorParam, addressList string) {

	pool := p.fast
	if v, ok := p.jobTimeoutCount.Load(jobID); ok && atomic.LoadInt32(v.(*int32)) > 10 {
		pool = p.slow
	}

	select {
	case pool <- func() {
		start := time.Now()
		defer func() {
			p.countTimeout(jobID, start)
		}()
		p.processTrigger(jobID, triggerType, failRetryCount, shardingParam, executorParam, addressList)
	}:
	default:
		adLog.Printf(">>>>>>>>>>> xxl-job, admin trigger pool execute too fast, jobId:%d", jobID)
	}
}

// countTimeout 每分钟轮转的超时计数。
func (p *triggerPool) countTimeout(jobID int, start time.Time) {
	minute := time.Now().Unix() / 60
	old := atomic.LoadInt64(&p.minute)
	if old != minute && atomic.CompareAndSwapInt64(&p.minute, old, minute) {
		p.jobTimeoutCount.Range(func(k, v any) bool {
			p.jobTimeoutCount.Delete(k)
			return true
		})
	}
	if time.Since(start) > 500*time.Millisecond {
		v, _ := p.jobTimeoutCount.LoadOrStore(jobID, new(int32))
		atomic.AddInt32(v.(*int32), 1)
	}
}

// processTrigger 对应 Java JobTrigger.trigger。
func (p *triggerPool) processTrigger(jobID int, triggerType TriggerType,
	failRetryCount int, shardingParam, executorParam, addressList string) {

	jobInfo, err := p.store.LoadJobInfo(jobID)
	if err != nil {
		adLog.Printf(">>>>>>>>>>>> trigger fail, jobId invalid，jobId=%d", jobID)
		return
	}
	if executorParam != "" {
		jobInfo.ExecutorParam = executorParam
	}
	finalFailRetryCount := jobInfo.ExecutorFailRetryCount
	if failRetryCount >= 0 {
		finalFailRetryCount = failRetryCount
	}
	group, err := p.store.LoadJobGroup(jobInfo.JobGroup)
	if err != nil {
		adLog.Printf(">>>>>>>>>>>> trigger fail, jobGroup invalid，jobId=%d", jobID)
		return
	}

	if addressList != "" {
		group.AddressType = 1
		group.AddressList = strings.TrimSpace(addressList)
	}

	regList := group.RegistryList()

	// 分片参数解析（重试时传入 "index/total"）
	var shard [2]int
	hasShard := false
	if shardingParam != "" {
		var idx, total int
		if _, err := fmt.Sscanf(shardingParam, "%d/%d", &idx, &total); err == nil && total > 0 {
			shard = [2]int{idx, total}
			hasShard = true
		}
	}

	if jobInfo.ExecutorRouteStrategy == RouteShardingBroadcast && len(regList) > 0 && !hasShard {
		for i := range regList {
			p.doProcessTrigger(group, jobInfo, finalFailRetryCount, triggerType, time.Now(), i, len(regList))
		}
	} else {
		idx, total := 0, 1
		if hasShard {
			idx, total = shard[0], shard[1]
		}
		p.doProcessTrigger(group, jobInfo, finalFailRetryCount, triggerType, time.Now(), idx, total)
	}
}

// doProcessTrigger 单次触发全流程：落日志 → 路由 → 远程触发 → 回写日志。
func (p *triggerPool) doProcessTrigger(group *JobGroup, jobInfo *JobInfo, finalFailRetryCount int,
	triggerType TriggerType, triggerTime time.Time, index, total int) {

	isSharding := jobInfo.ExecutorRouteStrategy == RouteShardingBroadcast
	shardingParam := ""
	if isSharding {
		shardingParam = fmt.Sprintf("%d/%d", index, total)
	}

	// 1、save log-id
	jobLog := &JobLog{
		JobGroup:    jobInfo.JobGroup,
		JobID:       jobInfo.ID,
		TriggerTime: triggerTime,
	}
	logID, err := p.store.InsertJobLog(jobLog)
	if err != nil {
		adLog.Printf(">>>>>>>>>>> xxl-job trigger fail, save log error, jobId:%d, err:%v", jobInfo.ID, err)
		return
	}
	jobLog.ID = logID

	// 2、init trigger-param
	glueUpdatetime := int64(0)
	if !jobInfo.GlueUpdatetime.IsZero() {
		glueUpdatetime = jobInfo.GlueUpdatetime.UnixMilli()
	}
	triggerParam := &xxl.TriggerRequest{
		JobID:                 jobInfo.ID,
		ExecutorHandler:       jobInfo.ExecutorHandler,
		ExecutorParams:        jobInfo.ExecutorParam,
		ExecutorBlockStrategy: jobInfo.ExecutorBlockStrategy,
		ExecutorTimeout:       jobInfo.ExecutorTimeout,
		LogID:                 logID,
		LogDateTime:           triggerTime.UnixMilli(),
		GlueType:              jobInfo.GlueType,
		GlueSource:            jobInfo.GlueSource,
		GlueUpdatetime:        glueUpdatetime,
		BroadcastIndex:        index,
		BroadcastTotal:        total,
	}

	// 3、init address
	regList := group.RegistryList()
	var address string
	var routeResult xxl.Response[string]
	if len(regList) > 0 {
		if isSharding {
			if index < len(regList) {
				address = regList[index]
			} else {
				address = regList[0]
			}
		} else {
			routeResult = route(jobInfo.ExecutorRouteStrategy, triggerParam, group, regList, p.client)
			if routeResult.IsSuccess() {
				address = routeResult.Data
			}
		}
	} else {
		routeResult = xxl.Fail[string]("调度中心获取不到执行器地址")
	}

	// 4、trigger remote executor
	var triggerResult xxl.Response[string]
	if address != "" {
		triggerResult = p.doTrigger(triggerParam, address, group)
	} else {
		triggerResult = xxl.Fail[string]("Address Router Fail.")
	}

	// 5、collection trigger info
	addrTypeDesc := "自动注册"
	if group.AddressType == 1 {
		addrTypeDesc = "手动录入"
	}
	triggerMsg := fmt.Sprintf("调度类型：%s", triggerType.Title())
	triggerMsg += fmt.Sprintf("<br>调度机器：%s", localIPDesc())
	triggerMsg += fmt.Sprintf("<br>执行器-注册方式：%s", addrTypeDesc)
	triggerMsg += fmt.Sprintf("<br>执行器-地址列表：%v", regList)
	triggerMsg += fmt.Sprintf("<br>路由策略：%s", jobInfo.ExecutorRouteStrategy)
	if isSharding {
		triggerMsg += fmt.Sprintf("(%s)", shardingParam)
	}
	triggerMsg += fmt.Sprintf("<br>阻塞处理策略：%s", jobInfo.ExecutorBlockStrategy)
	triggerMsg += fmt.Sprintf("<br>任务超时时间：%d", jobInfo.ExecutorTimeout)
	triggerMsg += fmt.Sprintf("<br>失败重试次数：%d", finalFailRetryCount)

	triggerMsg += "<br><br><span style=\"color:#00c0ef;\" > >>>>>>>>>>>触发<<<<<<<<<<< </span><br>"
	triggerMsg += "<br>执行器地址："
	if address != "" {
		triggerMsg += address
	} else if !routeResult.IsSuccess() {
		triggerMsg += "address route fail, " + routeResult.Msg
	} else {
		triggerMsg += "address route fail."
	}
	if jobInfo.ExecutorHandler != "" {
		triggerMsg += fmt.Sprintf("<br>JobHandler：%s", jobInfo.ExecutorHandler)
	}
	triggerMsg += fmt.Sprintf("<br>任务参数：%s", jobInfo.ExecutorParam)
	triggerMsg += "<br>触发结果："
	if triggerResult.IsSuccess() {
		triggerMsg += "success"
	} else {
		triggerMsg += "error, " + triggerResult.Msg
	}

	// 6、save log trigger-info
	jobLog.ExecutorAddress = address
	jobLog.ExecutorHandler = jobInfo.ExecutorHandler
	jobLog.ExecutorParam = jobInfo.ExecutorParam
	jobLog.ExecutorShardingParam = shardingParam
	jobLog.ExecutorFailRetryCount = finalFailRetryCount
	jobLog.TriggerCode = triggerResult.Code
	jobLog.TriggerMsg = triggerMsg
	if err := p.store.UpdateJobLogTriggerInfo(jobLog); err != nil {
		adLog.Printf(">>>>>>>>>>> xxl-job trigger fail, update log error, logId:%d, err:%v", logID, err)
	}
}

// doTrigger 远程触发执行器。
func (p *triggerPool) doTrigger(req *xxl.TriggerRequest, address string, group *JobGroup) xxl.Response[string] {
	result := p.client.trigger(address, group.AppName, group.AccessToken, req)
	msg := fmt.Sprintf("触发：<br>address：%s<br>code：%d<br>msg：%s", address, result.Code, result.Msg)
	result.Msg = msg
	return result
}

package admin

import (
	"sync"

	"xxljob-go/pkg/xxl"
)

// Service admin 总装：数据层 + 调度器 + 触发池 + 后台协程。
type Service struct {
	store       *Store
	scheduler   *scheduler
	triggerPool *triggerPool
	alarmer     alarmer

	callbackCh chan xxl.CallbackData
	stop       chan struct{}
	wg         sync.WaitGroup
}

func NewService(store *Store, smtp SMTPConfig) *Service {
	exeClient := newExecutorClient()
	s := &Service{
		store:      store,
		alarmer:    &emailAlermer{cfg: smtp},
		callbackCh: make(chan xxl.CallbackData, 2048),
		stop:       make(chan struct{}),
	}
	s.triggerPool = newTriggerPool(store, exeClient, s)
	s.scheduler = newScheduler(store, s.triggerPool)
	return s
}

// Start 启动全部后台线程。
func (s *Service) Start() {
	s.triggerPool.start(32, 16)
	s.scheduler.start()
	s.startRegistryWorker()
	s.startCallbackWorker()
	s.startFailAlarmMonitor()
	s.startLostMonitor()
	s.startReportWorker()
	adLog.Printf(">>>>>>>>>>> xxl-job admin service started.")
}

// Stop 优雅停止。
func (s *Service) Stop() {
	close(s.stop)
	s.scheduler.stopScheduler()
	s.wg.Wait()
	s.triggerPool.stopPool()
	adLog.Printf(">>>>>>>>>>> xxl-job admin service stopped.")
}

// TriggerPool 暴露触发能力给 REST/openapi。
func (s *Service) TriggerPool() *triggerPool { return s.triggerPool }

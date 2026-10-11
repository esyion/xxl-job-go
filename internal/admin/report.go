package admin

import (
	"fmt"
	"time"
)

// startReportWorker 日志报表聚合（对应 JobLogReportHelper）：
// 每分钟刷新今日统计，启动时补算昨日。
func (s *Service) startReportWorker() {
	s.refreshReport(time.Now().AddDate(0, 0, -1))
	s.refreshReport(time.Now())

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		lastDay := time.Now().Day()
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				if now.Day() != lastDay {
					// 跨天：补算昨日
					s.refreshReport(now.AddDate(0, 0, -1))
					lastDay = now.Day()
				}
				s.refreshReport(now)
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *Service) refreshReport(day time.Time) {
	running, suc, fail, err := s.store.LogReportCount(day)
	if err != nil {
		pkgLog.Error(fmt.Sprintf(">>>>>>>>>>> xxl-job log report count error:%v", err))
		return
	}
	if err := s.store.UpsertLogReport(day, int(running), int(suc), int(fail)); err != nil {
		pkgLog.Error(fmt.Sprintf(">>>>>>>>>>> xxl-job log report upsert error:%v", err))
	}
}

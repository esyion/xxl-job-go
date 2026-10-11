package admin

import (
	"strings"
	"time"

	"xxljob-go/pkg/xxl"
)

// startFailAlarmMonitor 失败告警监控（对应 JobFailAlarmMonitorHelper）：
// 失败日志 → 失败重试 + 邮件告警，alarm_status 做并发互斥。
func (s *Service) startFailAlarmMonitor() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.monitorFailLogs()
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *Service) monitorFailLogs() {
	ids, err := s.store.FindFailJobLogIDs(1000)
	if err != nil {
		adLog.Printf(">>>>>>>>>>> xxl-job fail-alarm-monitor find error:%v", err)
		return
	}
	for _, id := range ids {
		// 0 -> -1 抢占锁定
		if affected, _ := s.store.UpdateAlarmStatus(id, 0, -1); affected == 0 {
			continue
		}

		l, err := s.store.LoadJobLog(id)
		if err != nil {
			_, _ = s.store.UpdateAlarmStatus(id, -1, 0)
			continue
		}

		// 1、失败重试（触发失败或执行失败，且还有重试次数）
		if l.ExecutorFailRetryCount > 0 {
			s.triggerPool.trigger(l.JobID, TriggerRetry, l.ExecutorFailRetryCount-1,
				l.ExecutorShardingParam, l.ExecutorParam, "")
		}

		// 2、告警
		newAlarmStatus := 1 // 无需告警
		info, err := s.store.LoadJobInfo(l.JobID)
		if err == nil && strings.TrimSpace(info.AlarmEmail) != "" {
			ok := s.alarmer.alarm(info, l)
			if ok {
				newAlarmStatus = 2
			} else {
				newAlarmStatus = 3
			}
		}
		_, _ = s.store.UpdateAlarmStatus(id, -1, newAlarmStatus)
	}
}

// startLostMonitor 结果丢失监控：触发成功但 10 分钟未回调 → 标记失败并告警。
func (s *Service) startLostMonitor() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.monitorLostLogs()
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *Service) monitorLostLogs() {
	olderThan := time.Now().Add(-10 * time.Minute)
	ids, err := s.store.FindLostJobLogIDs(olderThan, 100)
	if err != nil {
		adLog.Printf(">>>>>>>>>>> xxl-job lost-monitor find error:%v", err)
		return
	}
	for _, id := range ids {
		if affected, _ := s.store.UpdateAlarmStatus(id, 0, -1); affected == 0 {
			continue
		}
		l, err := s.store.LoadJobLog(id)
		if err != nil {
			continue
		}
		l.HandleTime = time.Now()
		l.HandleCode = xxl.CodeFail
		l.HandleMsg = "job result lost (调度中心长时间未收到执行器回调)"
		s.complete(l)

		newAlarmStatus := 1
		if info, err := s.store.LoadJobInfo(l.JobID); err == nil && strings.TrimSpace(info.AlarmEmail) != "" {
			if s.alarmer.alarm(info, l) {
				newAlarmStatus = 2
			} else {
				newAlarmStatus = 3
			}
		}
		_, _ = s.store.UpdateAlarmStatus(id, -1, newAlarmStatus)
	}
}

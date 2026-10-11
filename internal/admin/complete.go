package admin

import (
	"strconv"
	"strings"
	"time"

	"xxljob-go/pkg/xxl"
)

// handleCallback 执行结果回调入口（openapi 调用），异步处理。
func (s *Service) handleCallback(list []xxl.CallbackData) xxl.Response[string] {
	if len(list) == 0 {
		return xxl.Fail[string]("invalid request, requestBody empty.")
	}
	for _, item := range list {
		select {
		case s.callbackCh <- item:
		default:
			// 回调积压时同步处理，避免丢失结果
			s.doCallback(item)
		}
	}
	return xxl.Success[string]("")
}

// startCallbackWorker 回调处理协程（对应 JobCompleteHelper 的 callbackThreadPool）。
func (s *Service) startCallbackWorker() {
	for i := 0; i < 16; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for {
				select {
				case item := <-s.callbackCh:
					s.doCallback(item)
				case <-s.stop:
					return
				}
			}
		}()
	}
}

// doCallback 单条回调：更新执行结果 + 触发子任务（对应 JobCompleter）。
func (s *Service) doCallback(item xxl.CallbackData) {
	log1, err := s.store.LoadJobLog(item.LogID)
	if err != nil {
		adLog.Printf(">>>>>>>>>>> xxl-job callback error, log item not found, logId:%d", item.LogID)
		return
	}
	if log1.HandleCode > 0 {
		return // 重复回调，忽略（避免重复触发子任务等）
	}

	// 合并历史消息
	handleMsg := log1.HandleMsg
	if handleMsg != "" {
		handleMsg += "<br>"
	}
	handleMsg += item.HandleMsg

	log1.HandleTime = time.Now()
	log1.HandleCode = item.HandleCode
	log1.HandleMsg = handleMsg
	s.complete(log1)
}

// complete 子任务触发 + 落库（对应 JobCompleter.complete）。
func (s *Service) complete(l *JobLog) {
	// 1、执行成功 → 触发子任务
	var childMsg string
	if l.HandleCode == xxl.CodeSuccess {
		if info, err := s.store.LoadJobInfo(l.JobID); err == nil && info.ChildJobID != "" {
			childMsg = "<br><br><span style=\"color:#00c0ef;\" > >>>>>>>>>>>触发子任务<<<<<<<<<<< </span><br>"
			for _, idStr := range strings.Split(info.ChildJobID, ",") {
				childID, err := strconv.Atoi(strings.TrimSpace(idStr))
				if err != nil || childID <= 0 {
					childMsg += "任务ID[" + idStr + "]非法"
					continue
				}
				if childID == l.JobID {
					continue // 忽略自引用
				}
				s.triggerPool.trigger(childID, TriggerParent, -1, "", "", "")
				childMsg += "子任务[" + idStr + "]触发成功"
			}
		}
	}
	if childMsg != "" {
		l.HandleMsg += childMsg
	}

	// 2、text 最大 64kb，避免超长
	if len(l.HandleMsg) > 15000 {
		l.HandleMsg = l.HandleMsg[:15000]
	}

	// 3、落库
	if err := s.store.UpdateJobLogHandleInfo(l); err != nil {
		adLog.Printf(">>>>>>>>>>> xxl-job complete update log error, logId:%d, err:%v", l.ID, err)
	}
}

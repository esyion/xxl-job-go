package admin

import (
	"fmt"
	"database/sql"
	"sync"
	"time"

	"xxljob-go/pkg/cronexpr"
)

const preReadMs = 5000 // 调度预读窗口

// GenerateNextTriggerTime 对应 ScheduleTypeEnum.getScheduleType().generateNextTriggerTime。
// 返回零值表示无法生成下次时间（任务将被停止）。
func GenerateNextTriggerTime(j *JobInfo, from time.Time) (time.Time, error) {
	switch j.ScheduleType {
	case ScheduleCron:
		expr, err := cronexpr.Parse(j.ScheduleConf)
		if err != nil {
			return time.Time{}, err
		}
		next := expr.Next(from)
		if next.IsZero() {
			return time.Time{}, nil // 4 年内无匹配
		}
		return next, nil
	case ScheduleFixRate:
		var rate int
		if _, err := parsePositiveInt(j.ScheduleConf, &rate); err != nil || rate < 1 {
			return time.Time{}, errBadScheduleConf
		}
		return from.Add(time.Duration(rate) * time.Second), nil
	default: // NONE
		return time.Time{}, nil
	}
}

type scheduler struct {
	store   *Store
	trigger *triggerPool

	ringMu   sync.Mutex
	ringData map[int][]int // 秒 -> []jobId 时间轮

	stop    chan struct{}
	stopped chan struct{}
}

func newScheduler(store *Store, trigger *triggerPool) *scheduler {
	return &scheduler{
		store:    store,
		trigger:  trigger,
		ringData: map[int][]int{},
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

func (s *scheduler) start() {
	go s.scheduleLoop()
	go s.ringLoop()
	pkgLog.Info(">>>>>>>>>>> init xxl-job admin scheduler success.")
}

func (s *scheduler) stopScheduler() {
	close(s.stop)
	select {
	case <-s.stopped:
	case <-time.After(15 * time.Second):
	}
}

// scheduleLoop 调度主循环：事务 + 行锁 + 预读 5s + 时间轮。
func (s *scheduler) scheduleLoop() {
	defer func() { s.stopped <- struct{}{} }()

	// 对齐整秒启动
	alignSecond()

	preReadCount := 6000

	for {
		select {
		case <-s.stop:
			pkgLog.Info(">>>>>>>>>>> xxl-job, admin JobScheduleHelper#scheduleThread stop")
			return
		default:
		}

		start := time.Now()
		preReadSuc := true

		tx, err := s.store.DB().Begin()
		if err == nil {
			err = s.store.ScheduleLock(tx)
		}
		if err != nil {
			pkgLog.Error(fmt.Sprintf(">>>>>>>>>>> xxl-job, JobScheduleHelper#scheduleThread error:%v", err))
			sleepToNextCycle(preReadSuc)
			continue
		}

		nowMs := time.Now().UnixMilli()
		list, err := s.store.ScheduleJobQueryTx(tx, nowMs+preReadMs, preReadCount)
		if err != nil {
			_ = tx.Rollback()
			pkgLog.Error(fmt.Sprintf(">>>>>>>>>>> xxl-job, JobScheduleHelper#scheduleThread error:%v", err))
			sleepToNextCycle(preReadSuc)
			continue
		}

		if len(list) > 0 {
			for _, j := range list {
				switch {
				case nowMs > j.TriggerNextTime+preReadMs:
					// 过期超过 5s：misfire 策略处理
					s.handleMisfire(j.ID, j.MisfireStrategy)
					s.refreshNextTriggerTime(j, time.Now())
				case nowMs >= j.TriggerNextTime:
					// 过期 5s 内：立即触发
					s.trigger.trigger(j.ID, TriggerCron, -1, "", "", "")
					s.refreshNextTriggerTime(j, time.Now())
					if j.TriggerStatus == TriggerStatusRunning && nowMs+preReadMs > j.TriggerNextTime {
						// 下次触发仍在 5s 窗口内：预读推入时间轮
						ringSecond := int((j.TriggerNextTime / 1000) % 60)
						s.pushTimeRing(ringSecond, j.ID)
						s.refreshNextTriggerTime(j, time.UnixMilli(j.TriggerNextTime))
					}
				default:
					// 未来 5s 内：推入时间轮
					ringSecond := int((j.TriggerNextTime / 1000) % 60)
					s.pushTimeRing(ringSecond, j.ID)
					s.refreshNextTriggerTime(j, time.UnixMilli(j.TriggerNextTime))
				}
				_ = s.store.ScheduleUpdate(tx, j)
			}
		} else {
			preReadSuc = false
		}

		_ = tx.Commit()

		cost := time.Since(start)
		if cost < time.Second {
			sleepToNextCycle(preReadSuc)
		}
	}
}

// ringLoop 时间轮循环：每秒处理当前刻度 + 回看前 2 个刻度（防漏），刻度内去重（防重）。
func (s *scheduler) ringLoop() {
	for {
		select {
		case <-s.stop:
			pkgLog.Info(">>>>>>>>>>> xxl-job, JobScheduleHelper#ringThread stop")
			return
		case <-time.After(time.Second):
		}
		alignSecond()

		nowSecond := time.Now().Second()
		var jobIDs []int
		s.ringMu.Lock()
		for i := 0; i <= 2; i++ {
			sec := (nowSecond + 60 - i) % 60
			item := s.ringData[sec]
			if len(item) == 0 {
				continue
			}
			delete(s.ringData, sec)
			seen := map[int]struct{}{}
			for _, id := range item {
				if _, dup := seen[id]; dup {
					pkgLog.Warn(fmt.Sprintf(">>>>>>>>>>> xxl-job, time-ring found job repeat beat : %d = %v", sec, item))
					continue
				}
				seen[id] = struct{}{}
				jobIDs = append(jobIDs, id)
			}
		}
		s.ringMu.Unlock()

		for _, id := range jobIDs {
			s.trigger.trigger(id, TriggerCron, -1, "", "", "")
		}
	}
}

func (s *scheduler) pushTimeRing(ringSecond int, jobID int) {
	s.ringMu.Lock()
	s.ringData[ringSecond] = append(s.ringData[ringSecond], jobID)
	s.ringMu.Unlock()
}

// handleMisfire 调度过期策略。
func (s *scheduler) handleMisfire(jobID int, strategy string) {
	if strategy == MisfireFireOnceNow {
		s.trigger.trigger(jobID, TriggerCron, -1, "", "", "")
	}
	// DO_NOTHING：跳过本次
}

// refreshNextTriggerTime 刷新下次触发时间；生成失败则停止任务。
func (s *scheduler) refreshNextTriggerTime(j *JobInfo, from time.Time) {
	next, err := GenerateNextTriggerTime(j, from)
	switch {
	case err != nil:
		j.TriggerStatus = TriggerStatusStopped
		j.TriggerLastTime, j.TriggerNextTime = 0, 0
		pkgLog.Error("refreshNextValidTime error for job", "jobId", j.ID, "scheduleType", j.ScheduleType, "scheduleConf", j.ScheduleConf, "err", err)
	case next.IsZero():
		j.TriggerStatus = TriggerStatusStopped
		j.TriggerLastTime, j.TriggerNextTime = 0, 0
		pkgLog.Warn("refreshNextValidTime fail for job", "jobId", j.ID, "scheduleType", j.ScheduleType, "scheduleConf", j.ScheduleConf)
	default:
		j.TriggerStatus = -1 // -1: scheduleUpdate 不更新状态列
		j.TriggerLastTime = j.TriggerNextTime
		j.TriggerNextTime = next.UnixMilli()
	}
}

func alignSecond() {
	now := time.Now()
	time.Sleep(time.Duration(1000-now.Nanosecond()/1e6) * time.Millisecond)
}

func sleepToNextCycle(preReadSuc bool) {
	base := time.Second
	if !preReadSuc {
		base = preReadMs * time.Millisecond
	}
	now := time.Now()
	elapsed := now.Sub(now.Truncate(time.Second))
	if d := base - elapsed; d > 0 {
		time.Sleep(d)
	}
}

var _ = sql.ErrNoRows

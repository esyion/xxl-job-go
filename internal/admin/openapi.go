package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"xxljob-go/pkg/xxl"
)

// handleOpenAPI 对应 Java OpenApiController：POST /api/{uri}，accessToken+appname 鉴权。
func (s *Service) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	uri := r.PathValue("uri")

	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }

	if r.Method != http.MethodPost {
		write(xxl.Fail[string]("invalid request, HttpMethod not support."))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil || len(body) == 0 {
		write(xxl.Fail[string]("invalid request, requestBody empty."))
		return
	}

	accessToken := r.Header.Get(xxl.HeaderAccessToken)
	appname := r.Header.Get(xxl.HeaderAppname)
	if accessToken == "" || appname == "" {
		write(xxl.Fail[string]("invalid request, accessToken or appname is empty."))
		return
	}
	group, err := s.store.LoadGroupByAppName(appname)
	if err != nil || group.AccessToken != accessToken {
		write(xxl.Fail[string]("invalid request, accessToken or appname invalid."))
		return
	}

	switch uri {
	case "callback":
		var req xxl.CallbackRequest
		if err := json.Unmarshal(body, &req); err != nil {
			write(xxl.Fail[string]("invalid request body."))
			return
		}
		write(s.handleCallback(req.CallbackList))
	case "registry":
		var req xxl.RegistryRequest
		if err := json.Unmarshal(body, &req); err != nil {
			write(xxl.Fail[string]("invalid request body."))
			return
		}
		if err := s.store.RegistrySave(req.RegistryGroup, req.RegistryKey, req.RegistryValue); err != nil {
			write(xxl.Fail[string](err.Error()))
			return
		}
		write(xxl.Success[string](""))
	case "registryRemove":
		var req xxl.RegistryRequest
		if err := json.Unmarshal(body, &req); err != nil {
			write(xxl.Fail[string]("invalid request body."))
			return
		}
		if err := s.store.RegistryRemove(req.RegistryGroup, req.RegistryKey, req.RegistryValue); err != nil {
			write(xxl.Fail[string](err.Error()))
			return
		}
		write(xxl.Success[string](""))

	case "addJob":
		var req addJobRequest
		if err := json.Unmarshal(body, &req); err != nil {
			write(xxl.Fail[string]("invalid request body."))
			return
		}
		write(s.apiAddJob(&req))
	case "updateJob":
		var req updateJobRequest
		if err := json.Unmarshal(body, &req); err != nil {
			write(xxl.Fail[string]("invalid request body."))
			return
		}
		write(s.apiUpdateJob(&req))
	case "removeJob", "startJob", "stopJob":
		var req struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(body, &req); err != nil || req.ID <= 0 {
			write(xxl.Fail[string]("invalid request body."))
			return
		}
		switch uri {
		case "removeJob":
			write(s.apiOperateJob(req.ID, "remove"))
		case "startJob":
			write(s.apiOperateJob(req.ID, "start"))
		default:
			write(s.apiOperateJob(req.ID, "stop"))
		}
	case "triggerJob":
		var req struct {
			ID            int    `json:"id"`
			ExecutorParam string `json:"executorParam"`
			AddressList   string `json:"addressList"`
		}
		if err := json.Unmarshal(body, &req); err != nil || req.ID <= 0 {
			write(xxl.Fail[string]("invalid request body."))
			return
		}
		s.TriggerPool().trigger(req.ID, TriggerApi, -1, "", req.ExecutorParam, req.AddressList)
		write(xxl.Success[string](""))

	default:
		write(xxl.Fail[string]("invalid request, uri-mapping(" + uri + ") not found."))
	}
}

// ============ 开放 API 的任务管理（对应 AdminJobBizImpl） ============

type addJobRequest struct {
	JobGroup               int    `json:"jobGroup"`
	Name                   string `json:"name"`
	Author                 string `json:"author"`
	AlarmEmail             string `json:"alarmEmail"`
	ScheduleType           string `json:"scheduleType"`
	ScheduleConf           string `json:"scheduleConf"`
	MisfireStrategy        string `json:"misfireStrategy"`
	ExecutorRouteStrategy  string `json:"executorRouteStrategy"`
	ExecutorHandler        string `json:"executorHandler"`
	ExecutorParam          string `json:"executorParam"`
	ExecutorBlockStrategy  string `json:"executorBlockStrategy"`
	ExecutorTimeout        int    `json:"executorTimeout"`
	ExecutorFailRetryCount int    `json:"executorFailRetryCount"`
	GlueType               string `json:"glueType"`
	GlueSource             string `json:"glueSource"`
	GlueRemark             string `json:"glueRemark"`
	ChildJobID             string `json:"childJobId"`
}

type updateJobRequest struct {
	ID int `json:"id"`
	addJobRequest
}

func (s *Service) apiAddJob(req *addJobRequest) xxl.Response[string] {
	j := &JobInfo{
		JobGroup: req.JobGroup, Name: req.Name, Author: req.Author, AlarmEmail: req.AlarmEmail,
		ScheduleType: req.ScheduleType, ScheduleConf: req.ScheduleConf, MisfireStrategy: req.MisfireStrategy,
		ExecutorRouteStrategy: req.ExecutorRouteStrategy, ExecutorHandler: req.ExecutorHandler,
		ExecutorParam: req.ExecutorParam, ExecutorBlockStrategy: req.ExecutorBlockStrategy,
		ExecutorTimeout: req.ExecutorTimeout, ExecutorFailRetryCount: req.ExecutorFailRetryCount,
		GlueType: req.GlueType, GlueSource: req.GlueSource, GlueRemark: req.GlueRemark,
		GlueUpdatetime: time.Now(), ChildJobID: req.ChildJobID,
	}
	if next, err := GenerateNextTriggerTime(j, time.Now()); err == nil && !next.IsZero() {
		j.TriggerNextTime = next.UnixMilli()
	}
	id, err := s.store.InsertJobInfo(j)
	if err != nil {
		return xxl.Fail[string](err.Error())
	}
	return xxl.Success[string](strconv.FormatInt(id, 10))
}

func (s *Service) apiUpdateJob(req *updateJobRequest) xxl.Response[string] {
	j, err := s.store.LoadJobInfo(req.ID)
	if err != nil {
		return xxl.Fail[string]("job not found, id:" + strconv.Itoa(req.ID))
	}
	j.JobGroup, j.Name, j.Author, j.AlarmEmail = req.JobGroup, req.Name, req.Author, req.AlarmEmail
	j.ScheduleType, j.ScheduleConf, j.MisfireStrategy = req.ScheduleType, req.ScheduleConf, req.MisfireStrategy
	j.ExecutorRouteStrategy, j.ExecutorHandler, j.ExecutorParam = req.ExecutorRouteStrategy, req.ExecutorHandler, req.ExecutorParam
	j.ExecutorBlockStrategy, j.ExecutorTimeout, j.ExecutorFailRetryCount = req.ExecutorBlockStrategy, req.ExecutorTimeout, req.ExecutorFailRetryCount
	j.GlueType, j.GlueSource, j.GlueRemark = req.GlueType, req.GlueSource, req.GlueRemark
	if j.GlueSource != "" {
		j.GlueUpdatetime = time.Now()
	}
	j.ChildJobID = req.ChildJobID
	if j.TriggerStatus == TriggerStatusRunning {
		if next, err := GenerateNextTriggerTime(j, time.Now()); err == nil {
			j.TriggerNextTime = next.UnixMilli()
		}
	}
	if err := s.store.UpdateJobInfo(j); err != nil {
		return xxl.Fail[string](err.Error())
	}
	return xxl.Success[string]("")
}

func (s *Service) apiOperateJob(id int, op string) xxl.Response[string] {
	j, err := s.store.LoadJobInfo(id)
	if err != nil {
		return xxl.Fail[string]("job not found, id:" + strconv.Itoa(id))
	}
	switch op {
	case "remove":
		if j.TriggerStatus == TriggerStatusRunning {
			return xxl.Fail[string]("请先停止任务再删除")
		}
		if err := s.store.DeleteJobInfo(id); err != nil {
			return xxl.Fail[string](err.Error())
		}
	case "start":
		next, err := GenerateNextTriggerTime(j, time.Now())
		if err != nil || next.IsZero() {
			return xxl.Fail[string]("任务调度时间非法或已无下次触发时间")
		}
		if err := s.store.UpdateJobTriggerStatusWithNext(id, TriggerStatusRunning, next.UnixMilli()); err != nil {
			return xxl.Fail[string](err.Error())
		}
	case "stop":
		if err := s.store.UpdateJobTriggerStatusWithNext(id, TriggerStatusStopped, 0); err != nil {
			return xxl.Fail[string](err.Error())
		}
	}
	return xxl.Success[string]("")
}

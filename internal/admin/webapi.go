package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"xxljob-go/pkg/xxl"
)

// ============ 会话 ============

const sessionCookie = "XXL_JOB_LOGIN_IDENTITY"

var (
	sessionMu sync.Mutex
	sessions  = map[string]*JobUser{} // token -> user
)

func hashPassword(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func newSessionToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func saveSession(u *JobUser) string {
	token := newSessionToken()
	sessionMu.Lock()
	sessions[token] = u
	sessionMu.Unlock()
	return token
}

func loadSession(r *http.Request) *JobUser {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	sessionMu.Lock()
	u := sessions[c.Value]
	sessionMu.Unlock()
	return u
}

func clearSession(w http.ResponseWriter) {
	if c, err := (&http.Request{Header: http.Header{}}).Cookie(sessionCookie); false || err != nil {
		_ = c
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
}

// ============ 通用响应 ============

// ReturnT 与原版管理台接口响应一致：{code, msg, content}
type ReturnT struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Content any    `json:"content"`
}

func returnOK(content any) ReturnT  { return ReturnT{Code: 200, Msg: nil2empty(), Content: content} }
func returnFail(msg string) ReturnT { return ReturnT{Code: 500, Msg: msg} }

func nil2empty() string { return "" }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	_ = json.NewEncoder(w).Encode(v)
}

// parseBody 解析 JSON body 到 out；空 body 时保持零值。
func parseBody(r *http.Request, out any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(out); err != nil {
		if err.Error() == "EOF" {
			return nil
		}
		return err
	}
	return nil
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// ============ 鉴权中间件 ============

func (s *Service) requireAuth(next func(w http.ResponseWriter, r *http.Request, u *JobUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := loadSession(r)
		if u == nil {
			writeJSON(w, returnFail("登录态失效，请重新登录"))
			return
		}
		next(w, r, u)
	}
}

// ============ 登录 ============

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserName string `json:"userName"`
		Password string `json:"password"`
	}
	if err := parseBody(r, &req); err != nil {
		writeJSON(w, returnFail("参数错误"))
		return
	}
	if req.UserName == "" || req.Password == "" {
		writeJSON(w, returnFail("请输入账号和密码"))
		return
	}
	u, err := s.store.LoadUserByName(strings.TrimSpace(req.UserName))
	if err != nil || u.Password != hashPassword(req.Password) {
		writeJSON(w, returnFail("账号或密码错误"))
		return
	}
	token := saveSession(u)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSession(w)
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	u := loadSession(r)
	if u == nil {
		writeJSON(w, returnFail("未登录"))
		return
	}
	writeJSON(w, returnOK(map[string]any{
		"userName": u.Username,
		"role":     u.Role,
	}))
}

func (s *Service) handleUpdatePwd(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		Password string `json:"password"`
	}
	if err := parseBody(r, &req); err != nil || len(req.Password) < 6 {
		writeJSON(w, returnFail("密码至少 6 位"))
		return
	}
	u2, _ := s.store.LoadUser(u.ID)
	u2.Password = hashPassword(req.Password)
	if err := s.store.UpdateUser(u2); err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, returnOK(nil))
}

// ============ 权限辅助（对应原版 JobGroupPermissionUtil） ============

// normalizeQueryGroup 普通用户强制 jobGroup 限定在授权分组内：
// 未选/越权时回落到第一个授权分组（原版行为）；无任何授权则返回 false（空结果）。
func normalizeQueryGroup(u *JobUser, jobGroup int) (int, bool) {
	if u.IsAdmin() {
		return jobGroup, true
	}
	groups := u.PermissionGroups()
	if len(groups) == 0 {
		return 0, false
	}
	if jobGroup > 0 && u.HasGroupPermission(jobGroup) {
		return jobGroup, true
	}
	return groups[0], true
}

// requireGroupPermission 校验用户对分组的操作权限，无权则写拒绝响应。
func requireGroupPermission(w http.ResponseWriter, u *JobUser, groupID int) bool {
	if u.HasGroupPermission(groupID) {
		return true
	}
	writeJSON(w, returnFail("您没有该执行器的操作权限"))
	return false
}

// ============ 任务管理 ============

func (s *Service) handleJobPageList(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		Start           int    `json:"start"`
		Length          int    `json:"length"`
		JobGroup        int    `json:"jobGroup"`
		Name            string `json:"name"`
		ExecutorHandler string `json:"executorHandler"`
		TriggerStatus   int    `json:"triggerStatus"`
	}
	if err := parseBody(r, &req); err != nil {
		writeJSON(w, returnFail("参数错误"))
		return
	}
	if req.Length <= 0 {
		req.Length = 10
	}
	// permission check
	jobGroup, ok := normalizeQueryGroup(u, req.JobGroup)
	if !ok {
		writeJSON(w, map[string]any{"recordsTotal": 0, "data": []any{}})
		return
	}
	list, total, err := s.store.PageJobInfo(jobGroup, req.Name, req.ExecutorHandler, req.TriggerStatus, req.Start, req.Length)
	if err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, map[string]any{"recordsTotal": total, "data": list})
}

func (s *Service) handleJobSave(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req addJobRequest
	if err := parseBody(r, &req); err != nil {
		writeJSON(w, returnFail("参数错误"))
		return
	}
	if req.JobGroup <= 0 || req.Name == "" {
		writeJSON(w, returnFail("请填写执行器和任务名称"))
		return
	}
	// permission check
	if !requireGroupPermission(w, u, req.JobGroup) {
		return
	}
	if req.ScheduleType == "" {
		req.ScheduleType = ScheduleNone
	}
	j := &JobInfo{
		JobGroup: req.JobGroup, Name: req.Name, Author: req.Author, AlarmEmail: req.AlarmEmail,
		ScheduleType: req.ScheduleType, ScheduleConf: req.ScheduleConf, MisfireStrategy: req.MisfireStrategy,
		ExecutorRouteStrategy: req.ExecutorRouteStrategy, ExecutorHandler: req.ExecutorHandler,
		ExecutorParam: req.ExecutorParam, ExecutorBlockStrategy: req.ExecutorBlockStrategy,
		ExecutorTimeout: req.ExecutorTimeout, ExecutorFailRetryCount: req.ExecutorFailRetryCount,
		GlueType: req.GlueType, GlueSource: req.GlueSource, GlueRemark: req.GlueRemark,
		GlueUpdatetime: time.Now(), ChildJobID: req.ChildJobID,
	}
	if j.GlueType == "" {
		j.GlueType = "BEAN"
	}
	if next, err := GenerateNextTriggerTime(j, time.Now()); err == nil && !next.IsZero() {
		j.TriggerNextTime = next.UnixMilli()
	}
	if _, err := s.store.InsertJobInfo(j); err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleJobUpdate(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req updateJobRequest
	if err := parseBody(r, &req); err != nil {
		writeJSON(w, returnFail("参数错误"))
		return
	}
	j, err := s.store.LoadJobInfo(req.ID)
	if err != nil {
		writeJSON(w, returnFail("任务不存在"))
		return
	}
	// permission check：原任务所在分组与目标分组都需有权限
	if !requireGroupPermission(w, u, j.JobGroup) || !requireGroupPermission(w, u, req.JobGroup) {
		return
	}
	j.JobGroup, j.Name, j.Author, j.AlarmEmail = req.JobGroup, req.Name, req.Author, req.AlarmEmail
	j.ScheduleType, j.ScheduleConf, j.MisfireStrategy = req.ScheduleType, req.ScheduleConf, req.MisfireStrategy
	j.ExecutorRouteStrategy, j.ExecutorHandler, j.ExecutorParam = req.ExecutorRouteStrategy, req.ExecutorHandler, req.ExecutorParam
	j.ExecutorBlockStrategy, j.ExecutorTimeout, j.ExecutorFailRetryCount = req.ExecutorBlockStrategy, req.ExecutorTimeout, req.ExecutorFailRetryCount
	j.GlueType, j.GlueSource, j.GlueRemark = req.GlueType, req.GlueSource, req.GlueRemark
	if req.GlueSource != "" {
		j.GlueUpdatetime = time.Now()
	}
	j.ChildJobID = req.ChildJobID
	if j.TriggerStatus == TriggerStatusRunning {
		next, err := GenerateNextTriggerTime(j, time.Now())
		if err != nil || next.IsZero() {
			writeJSON(w, returnFail("调度配置非法或已无下次触发时间"))
			return
		}
		j.TriggerNextTime = next.UnixMilli()
	}
	if err := s.store.UpdateJobInfo(j); err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleJobRemove(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		ID int `json:"id"`
	}
	_ = parseBody(r, &req)
	j, err := s.store.LoadJobInfo(req.ID)
	if err != nil {
		writeJSON(w, returnFail("任务不存在"))
		return
	}
	// permission check
	if !requireGroupPermission(w, u, j.JobGroup) {
		return
	}
	if j.TriggerStatus == TriggerStatusRunning {
		writeJSON(w, returnFail("请先停止任务再删除"))
		return
	}
	if err := s.store.DeleteJobInfo(req.ID); err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleJobStartStop(w http.ResponseWriter, r *http.Request, u *JobUser, start bool) {
	var req struct {
		ID int `json:"id"`
	}
	_ = parseBody(r, &req)
	j, err := s.store.LoadJobInfo(req.ID)
	if err != nil {
		writeJSON(w, returnFail("任务不存在"))
		return
	}
	// permission check
	if !requireGroupPermission(w, u, j.JobGroup) {
		return
	}
	if start {
		next, err := GenerateNextTriggerTime(j, time.Now())
		if err != nil || next.IsZero() {
			writeJSON(w, returnFail("调度配置非法或已无下次触发时间"))
			return
		}
		if err := s.store.UpdateJobTriggerStatusWithNext(req.ID, TriggerStatusRunning, next.UnixMilli()); err != nil {
			writeJSON(w, returnFail(err.Error()))
			return
		}
	} else {
		if err := s.store.UpdateJobTriggerStatusWithNext(req.ID, TriggerStatusStopped, 0); err != nil {
			writeJSON(w, returnFail(err.Error()))
			return
		}
	}
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleJobTrigger(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		ID            int    `json:"id"`
		ExecutorParam string `json:"executorParam"`
		AddressList   string `json:"addressList"`
	}
	_ = parseBody(r, &req)
	if req.ID <= 0 {
		writeJSON(w, returnFail("任务ID非法"))
		return
	}
	// permission check
	if j, err := s.store.LoadJobInfo(req.ID); err != nil {
		writeJSON(w, returnFail("任务不存在"))
		return
	} else if !requireGroupPermission(w, u, j.JobGroup) {
		return
	}
	s.TriggerPool().trigger(req.ID, TriggerManual, -1, "", req.ExecutorParam, req.AddressList)
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleNextTriggerTime(w http.ResponseWriter, r *http.Request) {
	scheduleType := r.URL.Query().Get("scheduleType")
	scheduleConf := r.URL.Query().Get("scheduleConf")
	j := &JobInfo{ScheduleType: scheduleType, ScheduleConf: scheduleConf}
	if _, err := GenerateNextTriggerTime(j, time.Now()); err != nil {
		writeJSON(w, returnFail("cron/fixed-rate 非法：" + err.Error()))
		return
	}
	var times []string
	t := time.Now()
	for i := 0; i < 5; i++ {
		next, err := GenerateNextTriggerTime(j, t)
		if err != nil || next.IsZero() {
			break
		}
		times = append(times, next.Format("2006-01-02 15:04:05"))
		t = next
	}
	if len(times) == 0 {
		writeJSON(w, returnFail("无法生成下次触发时间"))
		return
	}
	writeJSON(w, returnOK(times))
}

// ============ 执行器管理 ============

func (s *Service) handleGroupPageList(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		Start   int    `json:"start"`
		Length  int    `json:"length"`
		AppName string `json:"appName"`
	}
	_ = parseBody(r, &req)
	if req.Length <= 0 {
		req.Length = 10
	}
	// permission check：普通用户只返回授权分组（作为页面下拉数据源）
	list, total, err := s.store.ListJobGroups(req.AppName, 0, 100000)
	if err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	if !u.IsAdmin() {
		perm := map[int]bool{}
		for _, g := range u.PermissionGroups() {
			perm[g] = true
		}
		filtered := list[:0]
		for _, g := range list {
			if perm[g.ID] {
				filtered = append(filtered, g)
			}
		}
		list = filtered
		total = int64(len(list))
	}
	// 内存分页（分组数量小）
	start, end := req.Start, req.Start+req.Length
	if start > len(list) {
		start = len(list)
	}
	if end > len(list) {
		end = len(list)
	}
	writeJSON(w, map[string]any{"recordsTotal": total, "data": list[start:end]})
}

func (s *Service) handleGroupSave(w http.ResponseWriter, r *http.Request, u *JobUser) {
	if u.Role != RoleAdmin {
		writeJSON(w, returnFail("仅管理员可操作执行器"))
		return
	}
	var req struct {
		ID          int    `json:"id"`
		AppName     string `json:"appName"`
		Name        string `json:"name"`
		AddressType int    `json:"addressType"`
		AddressList string `json:"addressList"`
		AccessToken string `json:"accessToken"`
	}
	if err := parseBody(r, &req); err != nil {
		writeJSON(w, returnFail("参数错误"))
		return
	}
	if req.AppName == "" || req.Name == "" {
		writeJSON(w, returnFail("请填写 AppName 和名称"))
		return
	}
	if req.AccessToken == "" {
		req.AccessToken = "default_token"
	}
	g := &JobGroup{
		ID: req.ID, AppName: req.AppName, Name: req.Name,
		AddressType: req.AddressType, AddressList: req.AddressList, AccessToken: req.AccessToken,
	}
	if req.ID > 0 {
		if err := s.store.UpdateJobGroup(g); err != nil {
			writeJSON(w, returnFail(err.Error()))
			return
		}
	} else {
		if _, err := s.store.InsertJobGroup(g); err != nil {
			writeJSON(w, returnFail(err.Error()))
			return
		}
	}
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleGroupRemove(w http.ResponseWriter, r *http.Request, u *JobUser) {
	if u.Role != RoleAdmin {
		writeJSON(w, returnFail("仅管理员可操作执行器"))
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	_ = parseBody(r, &req)
	var jobCount int64
	s.store.DB().QueryRow(`SELECT COUNT(*) FROM xxl_job_info WHERE job_group=?`, req.ID).Scan(&jobCount)
	if jobCount > 0 {
		writeJSON(w, returnFail("该执行器下存在任务，禁止删除"))
		return
	}
	if err := s.store.DeleteJobGroup(req.ID); err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, returnOK(nil))
}

// ============ 日志 ============

func (s *Service) handleLogPageList(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		Start       int    `json:"start"`
		Length      int    `json:"length"`
		JobGroup    int    `json:"jobGroup"`
		JobID       int    `json:"jobId"`
		LogStatus   int    `json:"logStatus"` // -1 全部 1 成功 2 失败 3 进行中
		TimeFrom    string `json:"filterTimeFrom"`
		TimeTo      string `json:"filterTimeTo"`
	}
	if err := parseBody(r, &req); err != nil {
		writeJSON(w, returnFail("参数错误"))
		return
	}
	if req.Length <= 0 {
		req.Length = 10
	}
	// permission check
	jobGroup, ok := normalizeQueryGroup(u, req.JobGroup)
	if !ok {
		writeJSON(w, map[string]any{"recordsTotal": 0, "data": []any{}})
		return
	}
	var from, to time.Time
	if req.TimeFrom != "" {
		from, _ = time.ParseInLocation("2006-01-02 15:04:05", req.TimeFrom, time.Local)
	}
	if req.TimeTo != "" {
		to, _ = time.ParseInLocation("2006-01-02 15:04:05", req.TimeTo, time.Local)
	}
	list, total, err := s.store.PageJobLog(jobGroup, req.JobID, from, to, req.LogStatus, req.Start, req.Length)
	if err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, map[string]any{"recordsTotal": total, "data": list})
}

// handleLogDetailCat 滚动读取执行日志（admin -> 执行器 /log 代理）。
func (s *Service) handleLogDetailCat(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		LogID       int64 `json:"logId"`
		FromLineNum int   `json:"fromLineNum"`
	}
	_ = parseBody(r, &req)
	l, err := s.store.LoadJobLog(req.LogID)
	if err != nil || l.ExecutorAddress == "" {
		writeJSON(w, returnFail("日志不存在或执行器地址为空"))
		return
	}
	// permission check
	if !requireGroupPermission(w, u, l.JobGroup) {
		return
	}
	group, err := s.store.LoadJobGroup(l.JobGroup)
	if err != nil {
		writeJSON(w, returnFail("执行器不存在"))
		return
	}
	client := newExecutorClient()
	result := client.log(l.ExecutorAddress, group.AppName, group.AccessToken, &xxl.LogRequest{
		LogID:       l.ID,
		LogDateTime: l.TriggerTime.UnixMilli(),
		FromLineNum: req.FromLineNum,
	})
	if !result.IsSuccess() {
		writeJSON(w, returnFail("读取执行日志失败："+result.Msg))
		return
	}
	writeJSON(w, returnOK(result.Data))
}

// handleLogKill 终止运行中的任务（admin -> 执行器 /kill 代理）。
func (s *Service) handleLogKill(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		ID int64 `json:"id"`
	}
	_ = parseBody(r, &req)
	l, err := s.store.LoadJobLog(req.ID)
	if err != nil || l.ExecutorAddress == "" {
		writeJSON(w, returnFail("日志不存在或执行器地址为空"))
		return
	}
	// permission check
	if !requireGroupPermission(w, u, l.JobGroup) {
		return
	}
	if l.TriggerCode != 200 {
		writeJSON(w, returnFail("任务没有调度成功，无法终止"))
		return
	}
	group, _ := s.store.LoadJobGroup(l.JobGroup)
	client := newExecutorClient()
	result := client.kill(l.ExecutorAddress, group.AppName, group.AccessToken, l.JobID)
	if !result.IsSuccess() {
		writeJSON(w, returnFail(result.Msg))
		return
	}
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleLogClear(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		JobGroup int `json:"jobGroup"`
		JobID    int `json:"jobId"`
		Type     int `json:"type"` // 1 清理一个月前 2 清理一年前 3 清理一千条前 4 全部
	}
	_ = parseBody(r, &req)
	var before time.Time
	switch req.Type {
	case 1:
		before = time.Now().AddDate(0, -1, 0)
	case 2:
		before = time.Now().AddDate(-1, 0, 0)
	default:
		before = time.Now()
	}
	affected, err := s.store.ClearLog(req.JobGroup, req.JobID, before, req.Type, req.Type == 4)
	if err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, returnOK(fmt.Sprintf("清理日志数量：%d", affected)))
}

// ============ 用户管理 ============

func (s *Service) handleUserPageList(w http.ResponseWriter, r *http.Request, u *JobUser) {
	if u.Role != RoleAdmin {
		writeJSON(w, returnFail("仅管理员可查看用户列表"))
		return
	}
	var req struct {
		Start    int    `json:"start"`
		Length   int    `json:"length"`
		Username string `json:"username"`
		Role     int    `json:"role"`
	}
	_ = parseBody(r, &req)
	if req.Length <= 0 {
		req.Length = 10
	}
	list, total, err := s.store.PageUsers(req.Username, req.Role, req.Start, req.Length)
	if err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	for _, item := range list {
		item.Password = "" // 不回传密码哈希
	}
	writeJSON(w, map[string]any{"recordsTotal": total, "data": list})
}

func (s *Service) handleUserSave(w http.ResponseWriter, r *http.Request, u *JobUser) {
	if u.Role != RoleAdmin {
		writeJSON(w, returnFail("仅管理员可操作用户"))
		return
	}
	var req struct {
		ID         int    `json:"id"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Role       int    `json:"role"`
		Permission string `json:"permission"`
	}
	if err := parseBody(r, &req); err != nil || req.Username == "" {
		writeJSON(w, returnFail("请填写用户名"))
		return
	}
	if req.ID == 0 && len(req.Password) < 6 {
		writeJSON(w, returnFail("密码至少 6 位"))
		return
	}
	if req.ID > 0 {
		old, err := s.store.LoadUser(req.ID)
		if err != nil {
			writeJSON(w, returnFail("用户不存在"))
			return
		}
		old.Username, old.Role, old.Permission = req.Username, req.Role, req.Permission
		if req.Password != "" {
			old.Password = hashPassword(req.Password)
		}
		if err := s.store.UpdateUser(old); err != nil {
			writeJSON(w, returnFail(err.Error()))
			return
		}
	} else {
		nu := &JobUser{Username: req.Username, Password: hashPassword(req.Password), Role: req.Role, Permission: req.Permission}
		if err := s.store.InsertUser(nu); err != nil {
			writeJSON(w, returnFail(err.Error()))
			return
		}
	}
	writeJSON(w, returnOK(nil))
}

func (s *Service) handleUserRemove(w http.ResponseWriter, r *http.Request, u *JobUser) {
	if u.Role != RoleAdmin {
		writeJSON(w, returnFail("仅管理员可操作用户"))
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	_ = parseBody(r, &req)
	if req.ID == u.ID {
		writeJSON(w, returnFail("禁止删除当前登录用户"))
		return
	}
	if err := s.store.DeleteUser(req.ID); err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	writeJSON(w, returnOK(nil))
}

// ============ 报表 ============

func (s *Service) handleChartInfo(w http.ResponseWriter, r *http.Request, u *JobUser) {
	from := time.Now().AddDate(0, 0, -6)
	reports, err := s.store.ListLogReports(
		time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.Local),
		time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local))
	if err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	var jobInfoCount, runningCount int64
	s.store.DB().QueryRow(`SELECT COUNT(*) FROM xxl_job_info`).Scan(&jobInfoCount)
	s.store.DB().QueryRow(`SELECT COUNT(*) FROM xxl_job_info WHERE trigger_status=1`).Scan(&runningCount)
	writeJSON(w, returnOK(map[string]any{
		"reportList":     reports,
		"jobInfoCount":   jobInfoCount,
		"runningJobCount": runningCount,
	}))
}

// ============ GLUE 源码 ============

func (s *Service) handleGlueSave(w http.ResponseWriter, r *http.Request, u *JobUser) {
	var req struct {
		JobID      int    `json:"jobId"`
		GlueSource string `json:"glueSource"`
		GlueRemark string `json:"glueRemark"`
	}
	if err := parseBody(r, &req); err != nil || req.JobID <= 0 {
		writeJSON(w, returnFail("参数错误"))
		return
	}
	j, err := s.store.LoadJobInfo(req.JobID)
	if err != nil {
		writeJSON(w, returnFail("任务不存在"))
		return
	}
	j.GlueSource, j.GlueRemark, j.GlueUpdatetime = req.GlueSource, req.GlueRemark, time.Now()
	if err := s.store.UpdateJobInfo(j); err != nil {
		writeJSON(w, returnFail(err.Error()))
		return
	}
	_ = s.store.InsertJobLogGlue(&JobLogGlue{JobID: req.JobID, GlueType: j.GlueType, GlueSource: req.GlueSource, GlueRemark: req.GlueRemark})
	writeJSON(w, returnOK(nil))
}

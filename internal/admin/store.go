package admin

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Store 封装全部数据库访问（对应 admin 各 Mapper）。
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) DB() *sql.DB { return s.db }

var ErrNotFound = errors.New("not found")

// ============ xxl_job_info ============

const jobInfoCols = `id, job_group, name, COALESCE(author,''), COALESCE(alarm_email,''), schedule_type,
COALESCE(schedule_conf,''), misfire_strategy, COALESCE(executor_route_strategy,''), COALESCE(executor_handler,''),
COALESCE(executor_param,''), COALESCE(executor_block_strategy,''), executor_timeout, executor_fail_retry_count,
glue_type, COALESCE(glue_source,''), COALESCE(glue_remark,''), glue_updatetime, COALESCE(child_jobid,''),
trigger_status, trigger_last_time, trigger_next_time, add_time, update_time`

func scanJobInfo(row interface{ Scan(...any) error }) (*JobInfo, error) {
	var j JobInfo
	var glueUpd, addT, updT sql.NullTime
	err := row.Scan(&j.ID, &j.JobGroup, &j.Name, &j.Author, &j.AlarmEmail, &j.ScheduleType, &j.ScheduleConf,
		&j.MisfireStrategy, &j.ExecutorRouteStrategy, &j.ExecutorHandler, &j.ExecutorParam, &j.ExecutorBlockStrategy,
		&j.ExecutorTimeout, &j.ExecutorFailRetryCount, &j.GlueType, &j.GlueSource, &j.GlueRemark, &glueUpd,
		&j.ChildJobID, &j.TriggerStatus, &j.TriggerLastTime, &j.TriggerNextTime, &addT, &updT)
	if err != nil {
		return nil, err
	}
	j.GlueUpdatetime, j.AddTime, j.UpdateTime = glueUpd.Time, addT.Time, updT.Time
	return &j, nil
}

func (s *Store) LoadJobInfo(id int) (*JobInfo, error) {
	row := s.db.QueryRow(`SELECT `+jobInfoCols+` FROM xxl_job_info WHERE id = ?`, id)
	j, err := scanJobInfo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return j, err
}

// querier 兼容 *sql.DB 与 *sql.Tx。
type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// ScheduleJobQuery 调度扫描：取 next_time 在 maxNextTime 内的运行中任务。
func (s *Store) ScheduleJobQuery(maxNextTime int64, count int) ([]*JobInfo, error) {
	return s.ScheduleJobQueryTx(s.db, maxNextTime, count)
}

func (s *Store) ScheduleJobQueryTx(q querier, maxNextTime int64, count int) ([]*JobInfo, error) {
	rows, err := q.Query(`SELECT `+jobInfoCols+` FROM xxl_job_info
		WHERE trigger_status = 1 AND trigger_next_time <= ?
		ORDER BY id ASC LIMIT ?`, maxNextTime, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*JobInfo
	for rows.Next() {
		j, err := scanJobInfo(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, j)
	}
	return list, rows.Err()
}

// ScheduleUpdate 刷新触发时间（调度事务内）。
// triggerStatus < 0 表示不更新状态列；WHERE trigger_status=1 防止复活已停止任务。
func (s *Store) ScheduleUpdate(tx *sql.Tx, j *JobInfo) error {
	if j.TriggerStatus >= 0 {
		_, err := tx.Exec(`UPDATE xxl_job_info SET trigger_last_time=?, trigger_next_time=?, trigger_status=? WHERE id=? AND trigger_status=1`,
			j.TriggerLastTime, j.TriggerNextTime, j.TriggerStatus, j.ID)
		return err
	}
	_, err := tx.Exec(`UPDATE xxl_job_info SET trigger_last_time=?, trigger_next_time=? WHERE id=? AND trigger_status=1`,
		j.TriggerLastTime, j.TriggerNextTime, j.ID)
	return err
}

func (s *Store) InsertJobInfo(j *JobInfo) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO xxl_job_info(job_group, name, author, alarm_email, schedule_type, schedule_conf,
misfire_strategy, executor_route_strategy, executor_handler, executor_param, executor_block_strategy,
executor_timeout, executor_fail_retry_count, glue_type, glue_source, glue_remark, glue_updatetime, child_jobid,
trigger_status, trigger_last_time, trigger_next_time, add_time, update_time)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		j.JobGroup, j.Name, j.Author, j.AlarmEmail, j.ScheduleType, j.ScheduleConf,
		j.MisfireStrategy, j.ExecutorRouteStrategy, j.ExecutorHandler, j.ExecutorParam, j.ExecutorBlockStrategy,
		j.ExecutorTimeout, j.ExecutorFailRetryCount, j.GlueType, j.GlueSource, j.GlueRemark, j.GlueUpdatetime, j.ChildJobID,
		j.TriggerStatus, j.TriggerLastTime, j.TriggerNextTime, time.Now(), time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateJobInfo(j *JobInfo) error {
	_, err := s.db.Exec(`UPDATE xxl_job_info SET job_group=?, name=?, author=?, alarm_email=?, schedule_type=?, schedule_conf=?,
misfire_strategy=?, executor_route_strategy=?, executor_handler=?, executor_param=?, executor_block_strategy=?,
executor_timeout=?, executor_fail_retry_count=?, glue_type=?, glue_source=?, glue_remark=?, glue_updatetime=?, child_jobid=?,
update_time=? WHERE id=?`,
		j.JobGroup, j.Name, j.Author, j.AlarmEmail, j.ScheduleType, j.ScheduleConf,
		j.MisfireStrategy, j.ExecutorRouteStrategy, j.ExecutorHandler, j.ExecutorParam, j.ExecutorBlockStrategy,
		j.ExecutorTimeout, j.ExecutorFailRetryCount, j.GlueType, j.GlueSource, j.GlueRemark, j.GlueUpdatetime, j.ChildJobID,
		time.Now(), j.ID)
	return err
}

func (s *Store) DeleteJobInfo(id int) error {
	_, err := s.db.Exec(`DELETE FROM xxl_job_info WHERE id=?`, id)
	return err
}

// UpdateJobTriggerStatusWithNext 启动/停止任务并显式设置下次触发时间。
func (s *Store) UpdateJobTriggerStatusWithNext(id int, status int, nextTimeMs int64) error {
	_, err := s.db.Exec(`UPDATE xxl_job_info SET trigger_status=?, trigger_next_time=? WHERE id=?`, status, nextTimeMs, id)
	return err
}

// PageJobInfo 任务分页查询。
func (s *Store) PageJobInfo(jobGroup int, name, executorHandler string, triggerStatus int, start, length int) ([]*JobInfo, int64, error) {
	where, args := "1=1", []any{}
	if jobGroup > 0 {
		where += " AND job_group=?"
		args = append(args, jobGroup)
	}
	if name != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+name+"%")
	}
	if executorHandler != "" {
		where += " AND executor_handler LIKE ?"
		args = append(args, "%"+executorHandler+"%")
	}
	if triggerStatus >= 0 {
		where += " AND trigger_status=?"
		args = append(args, triggerStatus)
	}
	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM xxl_job_info WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := "SELECT " + jobInfoCols + " FROM xxl_job_info WHERE " + where + " ORDER BY id DESC LIMIT ?, ?"
	rows, err := s.db.Query(q, append(args, start, length)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []*JobInfo
	for rows.Next() {
		j, err := scanJobInfo(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, j)
	}
	return list, total, rows.Err()
}

// ============ xxl_job_group ============

func (s *Store) LoadJobGroup(id int) (*JobGroup, error) {
	row := s.db.QueryRow(`SELECT id, app_name, name, address_type, COALESCE(address_list,''), COALESCE(access_token,''), update_time FROM xxl_job_group WHERE id=?`, id)
	var g JobGroup
	err := row.Scan(&g.ID, &g.AppName, &g.Name, &g.AddressType, &g.AddressList, &g.AccessToken, &g.UpdateTime)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &g, err
}

// LoadGroupByAppName openapi 鉴权用。
func (s *Store) LoadGroupByAppName(appname string) (*JobGroup, error) {
	row := s.db.QueryRow(`SELECT id, app_name, name, address_type, COALESCE(address_list,''), COALESCE(access_token,''), update_time FROM xxl_job_group WHERE app_name=?`, appname)
	var g JobGroup
	err := row.Scan(&g.ID, &g.AppName, &g.Name, &g.AddressType, &g.AddressList, &g.AccessToken, &g.UpdateTime)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &g, err
}

func (s *Store) ListJobGroups(appname string, start, length int) ([]*JobGroup, int64, error) {
	where, args := "1=1", []any{}
	if appname != "" {
		where += " AND app_name LIKE ?"
		args = append(args, "%"+appname+"%")
	}
	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM xxl_job_group WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT id, app_name, name, address_type, COALESCE(address_list,''), COALESCE(access_token,''), update_time
		FROM xxl_job_group WHERE `+where+` ORDER BY id ASC LIMIT ?, ?`, append(args, start, length)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []*JobGroup
	for rows.Next() {
		var g JobGroup
		if err := rows.Scan(&g.ID, &g.AppName, &g.Name, &g.AddressType, &g.AddressList, &g.AccessToken, &g.UpdateTime); err != nil {
			return nil, 0, err
		}
		list = append(list, &g)
	}
	return list, total, rows.Err()
}

func (s *Store) InsertJobGroup(g *JobGroup) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO xxl_job_group(app_name, name, address_type, address_list, access_token, update_time)
		VALUES(?,?,?,?,?,now())`, g.AppName, g.Name, g.AddressType, g.AddressList, g.AccessToken)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateJobGroup(g *JobGroup) error {
	_, err := s.db.Exec(`UPDATE xxl_job_group SET app_name=?, name=?, address_type=?, address_list=?, access_token=?, update_time=now() WHERE id=?`,
		g.AppName, g.Name, g.AddressType, g.AddressList, g.AccessToken, g.ID)
	return err
}

func (s *Store) DeleteJobGroup(id int) error {
	_, err := s.db.Exec(`DELETE FROM xxl_job_group WHERE id=?`, id)
	return err
}

// ============ xxl_job_registry ============

func (s *Store) RegistrySave(group, key, value string) error {
	_, err := s.db.Exec(`INSERT INTO xxl_job_registry(registry_group, registry_key, registry_value, update_time)
		VALUES(?,?,?,now())
		ON DUPLICATE KEY UPDATE update_time=now()`, group, key, value)
	return err
}

func (s *Store) RegistryRemove(group, key, value string) error {
	_, err := s.db.Exec(`DELETE FROM xxl_job_registry WHERE registry_group=? AND registry_key=? AND registry_value=?`, group, key, value)
	return err
}

// RegistryRemoveDead 清理失联注册（超 DEAD_TIMEOUT 未心跳）。
func (s *Store) RegistryRemoveDead(deadBefore time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM xxl_job_registry WHERE update_time < ?`, deadBefore)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RegistryFindAll 存活注册列表。
func (s *Store) RegistryFindAll(after time.Time) ([]*JobRegistry, error) {
	rows, err := s.db.Query(`SELECT id, registry_group, registry_key, registry_value, update_time
		FROM xxl_job_registry WHERE update_time > ?`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*JobRegistry
	for rows.Next() {
		var r JobRegistry
		if err := rows.Scan(&r.ID, &r.RegistryGroup, &r.RegistryKey, &r.RegistryValue, &r.UpdateTime); err != nil {
			return nil, err
		}
		list = append(list, &r)
	}
	return list, rows.Err()
}

// UpdateGroupAddressListByAuto 自动注册组地址刷新。
func (s *Store) UpdateGroupAddressListByAuto(key string, addressList string) error {
	_, err := s.db.Exec(`UPDATE xxl_job_group SET address_list=? WHERE app_name=? AND address_type=0`, addressList, key)
	return err
}

// ============ xxl_job_lock ============

// ScheduleLock 事务内取调度锁（SELECT ... FOR UPDATE）。
func (s *Store) ScheduleLock(tx *sql.Tx) error {
	var name string
	return tx.QueryRow(`SELECT lock_name FROM xxl_job_lock WHERE lock_name = 'schedule_lock' FOR UPDATE`).Scan(&name)
}

// ============ xxl_job_log ============

const jobLogCols = `id, job_group, job_id, executor_address, executor_handler, executor_param,
executor_sharding_param, executor_fail_retry_count, trigger_time, trigger_code, trigger_msg,
handle_time, handle_code, handle_msg, alarm_status`

func scanJobLog(row interface{ Scan(...any) error }) (*JobLog, error) {
	var l JobLog
	var addr, handler, param, shardParam, triggerMsg, handleMsg sql.NullString
	var triggerTime, handleTime sql.NullTime
	err := row.Scan(&l.ID, &l.JobGroup, &l.JobID, &addr, &handler, &param,
		&shardParam, &l.ExecutorFailRetryCount, &triggerTime, &l.TriggerCode, &triggerMsg,
		&handleTime, &l.HandleCode, &handleMsg, &l.AlarmStatus)
	if err != nil {
		return nil, err
	}
	l.ExecutorAddress, l.ExecutorHandler, l.ExecutorParam = addr.String, handler.String, param.String
	l.ExecutorShardingParam, l.TriggerMsg, l.HandleMsg = shardParam.String, triggerMsg.String, handleMsg.String
	l.TriggerTime, l.HandleTime = triggerTime.Time, handleTime.Time
	return &l, nil
}

func (s *Store) InsertJobLog(l *JobLog) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO xxl_job_log(job_group, job_id, trigger_time, trigger_code, trigger_msg, handle_code, alarm_status)
		VALUES(?,?,?,?,?,?,0)`, l.JobGroup, l.JobID, l.TriggerTime, 0, "", 0)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateJobLogTriggerInfo(l *JobLog) error {
	_, err := s.db.Exec(`UPDATE xxl_job_log SET executor_address=?, executor_handler=?, executor_param=?,
executor_sharding_param=?, executor_fail_retry_count=?, trigger_code=?, trigger_msg=? WHERE id=?`,
		l.ExecutorAddress, l.ExecutorHandler, l.ExecutorParam, l.ExecutorShardingParam,
		l.ExecutorFailRetryCount, l.TriggerCode, l.TriggerMsg, l.ID)
	return err
}

func (s *Store) UpdateJobLogHandleInfo(l *JobLog) error {
	_, err := s.db.Exec(`UPDATE xxl_job_log SET handle_time=?, handle_code=?, handle_msg=? WHERE id=?`,
		l.HandleTime, l.HandleCode, l.HandleMsg, l.ID)
	return err
}

func (s *Store) LoadJobLog(id int64) (*JobLog, error) {
	row := s.db.QueryRow(`SELECT `+jobLogCols+` FROM xxl_job_log WHERE id=?`, id)
	l, err := scanJobLog(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return l, err
}

// PageJobLog 日志分页：filterLogStatus 1=成功 2=失败 3=进行中。
func (s *Store) PageJobLog(jobGroup, jobId int, filterTimeFrom, filterTimeTo time.Time, logStatus int, start, length int) ([]*JobLog, int64, error) {
	where, args := "1=1", []any{}
	if jobGroup > 0 {
		where += " AND job_group=?"
		args = append(args, jobGroup)
	}
	if jobId > 0 {
		where += " AND job_id=?"
		args = append(args, jobId)
	}
	if !filterTimeFrom.IsZero() {
		where += " AND trigger_time >= ?"
		args = append(args, filterTimeFrom)
	}
	if !filterTimeTo.IsZero() {
		where += " AND trigger_time <= ?"
		args = append(args, filterTimeTo)
	}
	switch logStatus {
	case 1:
		where += " AND handle_code = 200"
	case 2:
		where += " AND (handle_code = 500 OR trigger_code != 200)"
	case 3:
		where += " AND trigger_code = 200 AND handle_code = 0"
	}
	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM xxl_job_log WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT `+jobLogCols+` FROM xxl_job_log WHERE `+where+` ORDER BY id DESC LIMIT ?, ?`,
		append(args, start, length)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []*JobLog
	for rows.Next() {
		l, err := scanJobLog(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, l)
	}
	return list, total, rows.Err()
}

// FindFailJobLogIDs 失败且未告警完成的日志（告警监控用）。
func (s *Store) FindFailJobLogIDs(limit int) ([]int64, error) {
	rows, err := s.db.Query(`SELECT id FROM xxl_job_log
		WHERE !(trigger_code=200 AND handle_code=0)
		  AND (alarm_status=0 OR alarm_status=-1)
		ORDER BY id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpdateAlarmStatus CAS 更新告警状态。
func (s *Store) UpdateAlarmStatus(id int64, oldStatus, newStatus int) (int64, error) {
	res, err := s.db.Exec(`UPDATE xxl_job_log SET alarm_status=? WHERE id=? AND alarm_status=?`, newStatus, id, oldStatus)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// FindLostJobLogIDs 触发成功但长时间未回调的"丢失"日志。
func (s *Store) FindLostJobLogIDs(olderThan time.Time, limit int) ([]int64, error) {
	rows, err := s.db.Query(`SELECT id FROM xxl_job_log
		WHERE trigger_code=200 AND handle_code=0 AND trigger_time < ?
		ORDER BY id ASC LIMIT ?`, olderThan, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ClearLog 按类型清理日志：type 1=按时间 2=按任务 3=全部。
func (s *Store) ClearLog(jobGroup, jobId int, clearBefore time.Time, clearType int, clearAllInGroup bool) (int64, error) {
	where, args := "1=1", []any{}
	if clearType == 1 {
		where += " AND trigger_time <= ?"
		args = append(args, clearBefore)
	} else if clearType == 2 {
		where += " AND job_id=?"
		args = append(args, jobId)
	}
	if clearType != 3 && !clearAllInGroup {
		// 保持与原版一致：非"一键清理"时限定分组
		if jobGroup > 0 {
			where += " AND job_group=?"
			args = append(args, jobGroup)
		}
	}
	if clearType == 3 {
		if clearAllInGroup && jobGroup > 0 {
			where = "job_group=?"
			args = []any{jobGroup}
		} else {
			where, args = "1=1", args[:0]
		}
	}
	res, err := s.db.Exec(`DELETE FROM xxl_job_log WHERE `+where, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ============ xxl_job_log_report ============

// LogReportCount 某日统计（闭开区间：day 00:00 <= trigger_time < day+1 00:00）。
func (s *Store) LogReportCount(day time.Time) (running, suc, fail int64, err error) {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	row := s.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN trigger_code=200 AND handle_code=0 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN handle_code=200 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN !(trigger_code=200 AND handle_code=0) AND handle_code != 200 THEN 1 ELSE 0 END),0)
		FROM xxl_job_log WHERE trigger_time >= ? AND trigger_time < ?`, dayStart, dayEnd)
	err = row.Scan(&running, &suc, &fail)
	return
}

func (s *Store) UpsertLogReport(day time.Time, running, suc, fail int) error {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	_, err := s.db.Exec(`INSERT INTO xxl_job_log_report(trigger_day, running_count, suc_count, fail_count, update_time)
		VALUES(?,?,?,?,now())
		ON DUPLICATE KEY UPDATE running_count=?, suc_count=?, fail_count=?, update_time=now()`,
		dayStart, running, suc, fail, running, suc, fail)
	return err
}

func (s *Store) ListLogReports(from, to time.Time) ([]*JobLogReport, error) {
	rows, err := s.db.Query(`SELECT id, trigger_day, running_count, suc_count, fail_count, update_time
		FROM xxl_job_log_report WHERE trigger_day IS NOT NULL AND trigger_day BETWEEN ? AND ? ORDER BY trigger_day ASC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*JobLogReport
	for rows.Next() {
		var r JobLogReport
		if err := rows.Scan(&r.ID, &r.TriggerDay, &r.RunningCount, &r.SucCount, &r.FailCount, &r.UpdateTime); err != nil {
			return nil, err
		}
		list = append(list, &r)
	}
	return list, rows.Err()
}

// ============ xxl_job_user ============

func (s *Store) LoadUserByName(username string) (*JobUser, error) {
	row := s.db.QueryRow(`SELECT id, username, password, role, COALESCE(permission,'') FROM xxl_job_user WHERE username=?`, username)
	var u JobUser
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Permission)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (s *Store) LoadUser(id int) (*JobUser, error) {
	row := s.db.QueryRow(`SELECT id, username, password, role, COALESCE(permission,'') FROM xxl_job_user WHERE id=?`, id)
	var u JobUser
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Permission)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (s *Store) PageUsers(username string, role int, start, length int) ([]*JobUser, int64, error) {
	where, args := "1=1", []any{}
	if username != "" {
		where += " AND username LIKE ?"
		args = append(args, "%"+username+"%")
	}
	if role >= 0 {
		where += " AND role=?"
		args = append(args, role)
	}
	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM xxl_job_user WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT id, username, password, role, COALESCE(permission,'') FROM xxl_job_user WHERE `+
		where+` ORDER BY id ASC LIMIT ?, ?`, append(args, start, length)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []*JobUser
	for rows.Next() {
		var u JobUser
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Permission); err != nil {
			return nil, 0, err
		}
		list = append(list, &u)
	}
	return list, total, rows.Err()
}

func (s *Store) InsertUser(u *JobUser) error {
	_, err := s.db.Exec(`INSERT INTO xxl_job_user(username, password, role, permission) VALUES(?,?,?,?)`,
		u.Username, u.Password, u.Role, u.Permission)
	return err
}

func (s *Store) UpdateUser(u *JobUser) error {
	_, err := s.db.Exec(`UPDATE xxl_job_user SET username=?, password=?, role=?, permission=? WHERE id=?`,
		u.Username, u.Password, u.Role, u.Permission, u.ID)
	return err
}

func (s *Store) DeleteUser(id int) error {
	_, err := s.db.Exec(`DELETE FROM xxl_job_user WHERE id=?`, id)
	return err
}

// ============ xxl_job_logglue ============

func (s *Store) InsertJobLogGlue(g *JobLogGlue) error {
	_, err := s.db.Exec(`INSERT INTO xxl_job_logglue(job_id, glue_type, glue_source, glue_remark, add_time, update_time)
		VALUES(?,?,?,?,now(),now())`, g.JobID, g.GlueType, g.GlueSource, g.GlueRemark)
	return err
}

func (s *Store) FindJobLogGlue(jobID int, limit int) ([]*JobLogGlue, error) {
	rows, err := s.db.Query(`SELECT id, job_id, COALESCE(glue_type,''), COALESCE(glue_source,''), COALESCE(glue_remark,''), add_time, update_time
		FROM xxl_job_logglue WHERE job_id=? ORDER BY id DESC LIMIT ?`, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*JobLogGlue
	for rows.Next() {
		var g JobLogGlue
		if err := rows.Scan(&g.ID, &g.JobID, &g.GlueType, &g.GlueSource, &g.GlueRemark, &g.AddTime, &g.UpdateTime); err != nil {
			return nil, err
		}
		list = append(list, &g)
	}
	return list, rows.Err()
}

// ============ 帮助函数 ============

var errBadScheduleConf = errors.New("schedule_conf 非法")

func parsePositiveInt(s string, out *int) (bool, error) {
	n := 0
	for _, c := range strings.TrimSpace(s) {
		if c < '0' || c > '9' {
			return false, errBadScheduleConf
		}
		n = n*10 + int(c-'0')
		if n > 1<<30 {
			return false, errBadScheduleConf
		}
	}
	if s == "" {
		return false, errBadScheduleConf
	}
	*out = n
	return true, nil
}

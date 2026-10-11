package admin

import (
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

// SMTPConfig 邮件告警配置。
type SMTPConfig struct {
	Host     string // smtp 服务器
	Port     int    // 端口，默认 25
	Username string
	Password string
	From     string // 发件人，默认 Username
	StartTLS bool   // 是否需要 STARTTLS
}

type alarmer interface {
	alarm(info *JobInfo, log1 *JobLog) bool
}

type emailAlermer struct {
	cfg SMTPConfig
}

func (e *emailAlermer) alarm(info *JobInfo, log1 *JobLog) bool {
	if e.cfg.Host == "" {
		return false
	}
	port := e.cfg.Port
	if port == 0 {
		port = 25
	}
	from := e.cfg.From
	if from == "" {
		from = e.cfg.Username
	}

	subject := "XXL-JOB 任务告警"
	body := fmt.Sprintf(`<h4>调度中心任务告警</h4>
<table border="1" cellpadding="6">
<tr><td>任务ID</td><td>%d</td></tr>
<tr><td>任务名称</td><td>%s</td></tr>
<tr><td>执行器</td><td>%s</td></tr>
<tr><td>JobHandler</td><td>%s</td></tr>
<tr><td>执行机器</td><td>%s</td></tr>
<tr><td>触发时间</td><td>%s</td></tr>
<tr><td>调度结果</td><td>%s</td></tr>
<tr><td>执行结果</td><td>%s</td></tr>
<tr><td>失败次数</td><td>%d</td></tr>
</table>`,
		info.ID, info.Name, strconv.Itoa(info.JobGroup), info.ExecutorHandler, log1.ExecutorAddress,
		log1.TriggerTime.Format("2006-01-02 15:04:05"),
		statusText(log1.TriggerCode), statusText(log1.HandleCode), log1.ExecutorFailRetryCount)

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + info.AlarmEmail,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", e.cfg.Host, port)
	var auth smtp.Auth
	if e.cfg.Username != "" {
		auth = smtp.PlainAuth("", e.cfg.Username, e.cfg.Password, e.cfg.Host)
	}
	err := smtp.SendMail(addr, auth, from, strings.Split(info.AlarmEmail, ","), []byte(msg))
	if err != nil {
		pkgLog.Error(fmt.Sprintf(">>>>>>>>>>> xxl-job job alarm email send error:%v", err))
		return false
	}
	return true
}

func statusText(code int) string {
	switch code {
	case 0:
		return "无"
	case 200:
		return "<span style=\"color: green\">成功</span>"
	case 502:
		return "<span style=\"color: orange\">超时</span>"
	default:
		return "<span style=\"color: red\">失败</span>"
	}
}

type noopAlermer struct{}

func (noopAlermer) alarm(*JobInfo, *JobLog) bool { return false }

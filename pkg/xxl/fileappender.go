package xxl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileAppender 对应 Java XxlJobFileAppender：按天分目录、按 logId 分文件的执行日志。
type FileAppender struct {
	basePath string
}

func NewFileAppender(basePath string) *FileAppender {
	if basePath == "" {
		basePath = filepath.Join(".", "logs", "xxl-job")
	}
	return &FileAppender{basePath: basePath}
}

// CallbackLogPath 回调失败重试文件目录。
func (a *FileAppender) CallbackLogPath() string {
	return filepath.Join(a.basePath, "callback")
}

// MakeLogFileName 生成日志文件名：basePath/yyyy-MM-dd/{logId}.log
func (a *FileAppender) MakeLogFileName(logDateTime int64, logID int64) string {
	day := unixToDay(logDateTime)
	dayDir := filepath.Join(a.basePath, day)
	_ = os.MkdirAll(dayDir, 0755)
	return filepath.Join(dayDir, fmt.Sprintf("%d.log", logID))
}

// ReadLog 从 fromLineNum(1-based) 起读取日志，单次最多 RollLogSize 行。
// 返回的 toLineNum 为本次读取到的最后一行行号；isEnd 表示已到文件末尾。
func (a *FileAppender) ReadLog(fileName string, fromLineNum int) *LogData {
	if fromLineNum < 1 {
		fromLineNum = 1
	}
	data := &LogData{FromLineNum: fromLineNum, ToLineNum: 0, IsEnd: true}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return data // 文件不存在视为已读完
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	if fromLineNum > len(lines) {
		return data
	}
	to := fromLineNum + RollLogSize
	isEnd := true
	if to > len(lines) {
		to = len(lines) + 1
	} else {
		isEnd = false
	}
	data.ToLineNum = to - 1
	data.LogContent = strings.Join(lines[fromLineNum-1:to-1], "\n")
	data.IsEnd = isEnd
	return data
}

func unixToDay(ms int64) string { return time.UnixMilli(ms).Format("2006-01-02") }
func timeNow() time.Time        { return time.Now() }

// CleanExpired 清理 retainDays 天之前的按天日志目录。
func (a *FileAppender) CleanExpired(retainDays int) {
	if retainDays < 1 {
		return
	}
	entries, err := os.ReadDir(a.basePath)
	if err != nil {
		return
	}
	limit := timeNow().AddDate(0, 0, -retainDays).Format("2006-01-02")
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || len(name) != 10 || !strings.HasPrefix(name, "20") {
			continue
		}
		if name < limit {
			_ = os.RemoveAll(filepath.Join(a.basePath, name))
		}
	}
}

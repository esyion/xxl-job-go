// Package logx 基于官方 log/slog 的统一日志封装。
// 默认 text 格式输出到 stdout；设置 XXL_JOB_LOG_FORMAT=json 切换 JSON
// （容器/采集场景友好）。支持 XXL_JOB_LOG_LEVEL 控制级别（debug/info/warn/error）。
package logx

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var logger = newLogger()

func newLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: levelFromEnv()}
	var h slog.Handler = slog.NewTextHandler(os.Stdout, opts)
	if strings.EqualFold(os.Getenv("XXL_JOB_LOG_FORMAT"), "json") {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

func levelFromEnv() slog.Level {
	switch strings.ToLower(os.Getenv("XXL_JOB_LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// L 返回底层 logger（需要自定义 Handler 时使用）。
func L() *slog.Logger { return logger }

func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }

// Printf 风格便捷方法（内部格式化后走 slog）。
func Debugf(format string, args ...any) { logger.Debug(fmt.Sprintf(format, args...)) }
func Infof(format string, args ...any)  { logger.Info(fmt.Sprintf(format, args...)) }
func Warnf(format string, args ...any)  { logger.Warn(fmt.Sprintf(format, args...)) }
func Errorf(format string, args ...any) { logger.Error(fmt.Sprintf(format, args...)) }

// Fatalf 打印后退出进程（启动期致命错误用）。
func Fatalf(format string, args ...any) {
	logger.Error("fatal: " + fmt.Sprintf(format, args...))
	os.Exit(1)
}

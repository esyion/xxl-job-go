package admin

import (
	"log"
	"os"
)

// Logf / LogFatalf 供 cmd 层使用的日志出口。
var exportedLog = log.New(os.Stdout, "[xxl-job-admin] ", log.LstdFlags|log.Lmsgprefix)

func Logf(format string, args ...any)     { exportedLog.Printf(format, args...) }
func LogFatalf(format string, args ...any) { exportedLog.Fatalf(format, args...) }

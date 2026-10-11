package admin

import (
	"fmt"

	"xxljob-go/pkg/logx"
)

// pkgLog 模块级 logger，挂到 logx 的统一 handler 上。
var pkgLog = logx.L().With("module", "admin")

// Logf / LogFatalf 供 cmd 层使用的日志出口。
func Logf(format string, args ...any)      { logx.Info(fmt.Sprintf(format, args...), "module", "admin") }
func LogFatalf(format string, args ...any) { logx.Fatalf(format, args...) }

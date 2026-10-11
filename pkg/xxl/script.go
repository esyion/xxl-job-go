package xxl

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// scriptJobHandler 对应 Java ScriptJobHandler：GLUE 脚本任务。
type scriptJobHandler struct {
	jobID          int
	glueUpdatetime int64
	glueSource     string
	glueType       string
	logPath        string
}

var glueCmd = map[string]struct {
	cmd  string
	ext  string
}{
	GlueShell:      {"/bin/bash", ".sh"},
	GluePython:     {"python3", ".py"},
	GlueNodejs:     {"node", ".js"},
	GluePhp:        {"php", ".php"},
	GluePowershell: {"pwsh", ".ps1"},
}

func (h *scriptJobHandler) Execute(ctx *Context) {
	g, ok := glueCmd[h.glueType]
	if !ok {
		ctx.HandleFail("glueType[" + h.glueType + "] is not valid.")
		return
	}
	if _, err := exec.LookPath(g.cmd); err != nil {
		ctx.HandleFail(fmt.Sprintf("command %q not found on executor", g.cmd))
		return
	}

	glueDir := filepath.Join(h.logPath, "gluesource")
	_ = os.MkdirAll(glueDir, 0755)
	scriptFile := filepath.Join(glueDir, fmt.Sprintf("%d-%d%s", h.jobID, h.glueUpdatetime, g.ext))
	if err := os.WriteFile(scriptFile, []byte(h.glueSource), 0644); err != nil {
		ctx.HandleFail("write glue source fail: " + err.Error())
		return
	}

	// 与 Java ScriptUtil 一致的参数顺序：脚本文件、日志文件、jobId、任务参数
	cmdArgs := []string{scriptFile, ctx.logFileName, fmt.Sprintf("%d", ctx.JobID), ctx.JobParam}
	cmd := exec.Command(g.cmd, cmdArgs...)

	logFile, err := os.OpenFile(ctx.logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		defer logFile.Close()
		cmd.Stdout = io.MultiWriter(logFile, os.Stdout)
		cmd.Stderr = io.MultiWriter(logFile, os.Stderr)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		ctx.HandleFail(strings.TrimSpace(fmt.Sprintf("exit code(%d), %v", cmd.ProcessState.ExitCode(), err)))
		return
	}
	ctx.HandleSuccess()
}

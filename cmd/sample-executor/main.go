// xxl-job 执行器示例（Go 版）：演示 BEAN 任务与分片广播任务的写法。
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xxljob-go/pkg/xxl"
)

func main() {
	executor := xxl.NewExecutor(xxl.ExecutorConfig{
		Appname:        "xxl-job-executor-sample",
		AccessToken:    "default_token",
		Port:           9999,
		Address:        "http://127.0.0.1:9999", // 本机调试用；生产多机部署可留空自动探测
		AdminAddresses: []string{"http://127.0.0.1:8080"},
		LogPath:        "./logs/xxl-job",
	})

	// 示例：普通 BEAN 任务
	executor.RegHandler("demoJobHandler", func(ctx *xxl.Context) {
		ctx.Log("示例任务执行中... param=" + ctx.JobParam)
		time.Sleep(2 * time.Second)
		ctx.Log("示例任务执行完毕")
		ctx.HandleSuccess()
	})

	// 示例：分片广播任务
	executor.RegHandler("shardingJobHandler", func(ctx *xxl.Context) {
		ctx.Logf("分片参数：当前分片=%d, 总分片=%d", ctx.BroadcastIndex, ctx.BroadcastTotal)
		ctx.HandleSuccess()
	})

	// 示例：返回失败结果的任务（可测试失败重试与告警）
	executor.RegHandler("failJobHandler", func(ctx *xxl.Context) {
		ctx.Log("这个任务永远失败")
		ctx.HandleFail("业务执行失败示例")
	})

	if err := executor.Run(); err != nil {
		fmt.Println("执行器启动失败:", err)
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	executor.Stop()
}

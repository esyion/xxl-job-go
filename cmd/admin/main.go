// xxl-job-admin（Go 版）：调度中心。
//
// 配置通过环境变量：
//   XXL_JOB_ADMIN_PORT        监听端口，默认 8080
//   XXL_JOB_DB_DSN            MySQL DSN（xxl-job 库）
//   XXL_JOB_STATIC_DIR        React 构建产物目录（web/dist），默认 ./web/dist
//   XXL_JOB_SMTP_HOST/PORT/USERNAME/PASSWORD/FROM  告警邮箱（可选）
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"xxljob-go/internal/admin"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	port := env("XXL_JOB_ADMIN_PORT", "8080")
	dsn := os.Getenv("XXL_JOB_DB_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/xxl_job?charset=utf8mb4&parseTime=true&loc=Local"
	}
	staticDir := env("XXL_JOB_STATIC_DIR", "./web/dist")

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		admin.LogFatalf("连接数据库失败，请检查 XXL_JOB_DB_DSN：%v", err)
	}

	smtpPort, _ := strconv.Atoi(env("XXL_JOB_SMTP_PORT", "25"))
	store := admin.NewStore(db)
	svc := admin.NewService(store, admin.SMTPConfig{
		Host:     env("XXL_JOB_SMTP_HOST", ""),
		Port:     smtpPort,
		Username: env("XXL_JOB_SMTP_USERNAME", ""),
		Password: env("XXL_JOB_SMTP_PASSWORD", ""),
		From:     env("XXL_JOB_SMTP_FROM", ""),
	})
	svc.Start()

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           svc.HTTPHandler(staticDir),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		admin.Logf("xxl-job-admin 启动完成: http://127.0.0.1:%s/  (默认账号 admin / 123456)", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			admin.LogFatalf("HTTP 服务启动失败：%v", err)
		}
	}()

	// 优雅停止
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	admin.Logf("正在停止 xxl-job-admin ...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	svc.Stop()
	admin.Logf("xxl-job-admin 已停止")
}

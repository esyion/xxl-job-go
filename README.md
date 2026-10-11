# xxl-job-go

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8)](https://go.dev)
[![React](https://img.shields.io/badge/React-18-61DAFB)](https://react.dev)
[![License](https://img.shields.io/badge/License-GPL--3.0-blue)](./LICENSE)

[XXL-JOB](https://github.com/xuxueli/xxl-job)（v3.5.x）的 **Go + React** 复刻实现：调度中心、执行器 SDK、管理控制台三位一体，协议与表结构同原版 1:1 对齐，可直接与官方 Java 执行器（xxl-job-core 3.x）混合组网。

- **调度中心（admin）**：Go 实现，调度双循环（数据库事务行锁 + 预读 5s + 秒级时间轮）、fast/slow 双触发池、10 种路由策略、misfire 策略、失败重试、子任务、邮件告警、日志报表，多节点集群部署依旧靠 `xxl_job_lock` 行锁互斥
- **执行器 SDK（pkg/xxl）**：Go 实现，内嵌 HTTP 服务、每任务一 goroutine 的触发队列、阻塞策略（串行/丢弃/覆盖）、执行日志落盘与滚动读取、GLUE 脚本任务（Shell/Python/NodeJS/PHP/PowerShell）、注册心跳、回调失败落盘重试
- **管理台（web）**：React + Vite + Ant Design + ECharts：登录、运行报表、任务管理（CRUD/启停/执行一次/GLUE 编辑/cron 预览）、执行器管理、调度日志（终端风格滚动日志/终止任务/清理）、用户管理
- **通信协议**：HTTP + JSON，字段与 `xxl-job-core` openapi DTO 完全一致（执行器侧 `/trigger` `/beat` `/idleBeat` `/kill` `/log`，admin 侧 `/api/{callback|registry|addJob|...}`），鉴权为 `XXL-JOB-ACCESS-TOKEN` + `XXL-JOB-APPNAME` 请求头
- **数据库**：沿用官方 8 张表，开箱即用

## 快速开始

```bash
# 1. 启动 MySQL（docker）
docker compose up -d
docker exec -i xxl-job-go-mysql mysql -uroot -proot --default-character-set=utf8mb4 xxl_job < doc/tables_xxl_job.sql

# 2. 构建并启动调度中心（默认 8080，账号 admin / 123456）
go build -o bin/admin ./cmd/admin
XXL_JOB_DB_DSN='root:root@tcp(127.0.0.1:3306)/xxl_job?charset=utf8mb4&parseTime=true&loc=Local' \
XXL_JOB_STATIC_DIR=./web/dist ./bin/admin

# 3. 启动示例执行器（端口 9999，每 30s 向 admin 注册心跳）
go build -o bin/sample-executor ./cmd/sample-executor
./bin/sample-executor
```

> 前端管理台随 admin 一起部署（admin 直接托管 `web/dist` 静态文件），无需单独的前端进程。首次使用先构建前端：`cd web && npm install && npm run build`。

打开 http://127.0.0.1:8080 登录后：执行器管理页可看到 `xxl-job-executor-sample` 已自动注册；任务管理页有 3 个示例任务（BEAN / 分片广播 / GLUE-SHELL）。

### 环境变量（admin）

| 变量 | 默认 | 说明 |
|---|---|---|
| `XXL_JOB_ADMIN_PORT` | 8080 | 监听端口 |
| `XXL_JOB_DB_DSN` | root:root@tcp(127.0.0.1:3306)/xxl_job... | MySQL DSN |
| `XXL_JOB_STATIC_DIR` | ./web/dist | React 构建产物目录 |
| `XXL_JOB_SMTP_HOST/PORT/USERNAME/PASSWORD/FROM` | 空 | 失败告警邮箱（可选） |

## 业务代码接入执行器 SDK

```go
executor := xxl.NewExecutor(xxl.ExecutorConfig{
    Appname:        "my-app",                    // 与 admin 执行器配置的 AppName 一致
    AccessToken:    "default_token",             // 与执行器分组的 AccessToken 一致
    Port:           9999,                        // 内嵌服务端口
    AdminAddresses: []string{"http://127.0.0.1:8080"},
    LogPath:        "./logs/xxl-job",
    Address:        "",                          // 留空自动探测；本机调试可写 http://127.0.0.1:9999
})

executor.RegHandler("demoJobHandler", func(ctx *xxl.Context) {
    ctx.Log("参数：", ctx.JobParam)               // 写执行日志（admin 页面可滚动查看）
    ctx.Logf("分片：%d/%d", ctx.BroadcastIndex, ctx.BroadcastTotal)
    // ... 业务逻辑 ...
    ctx.HandleSuccess()                           // 或 ctx.HandleFail("原因")
})

if err := executor.Run(); err != nil { panic(err) }
defer executor.Stop()
```

GLUE 脚本任务（SHELL/PYTHON/NODEJS/PHP/POWERSHELL）直接在管理台编辑源码即可，无需重新部署。

### 非 Go 业务系统

协议与原版对齐，因此 Java 项目可直接使用官方 `xxl-job-core`（3.x）接入本调度中心；其他语言按协议实现 5 个服务端接口（beat/idleBeat/trigger/kill/log）+ 2 个上报调用（registry/callback）即可。

## 目录结构

```
├── cmd/
│   ├── admin/            # 调度中心入口
│   └── sample-executor/  # 示例执行器
├── pkg/
│   ├── xxl/              # 执行器 SDK（≈ xxl-job-core）
│   │   ├── dto.go        # 协议 DTO（字段名与 Java 一致）
│   │   ├── executor.go   # Executor 组装 + trigger/idleBeat/kill/log 业务
│   │   ├── jobthread.go  # 任务线程：队列/超时/回调/空闲回收
│   │   ├── server.go     # 内嵌 HTTP 服务（net/http）
│   │   ├── callback.go   # 回调队列 + 失败落盘重试
│   │   ├── registry.go   # 30s 心跳注册
│   │   ├── fileappender.go # 执行日志按天/logId 分文件 + 滚动读
│   │   └── script.go     # GLUE 脚本执行
│   └── cronexpr/         # Quartz 风格 cron（6/7 字段、L/W/# 语义）
├── internal/admin/       # 调度中心（≈ xxl-job-admin）
│   ├── schedule.go       # JobScheduleHelper：事务+行锁+时间轮
│   ├── trigger.go        # 触发池 + processTrigger
│   ├── route.go          # 10 种路由策略
│   ├── registryhelper.go # 注册清理与地址刷新
│   ├── complete.go       # 回调处理 + 子任务触发
│   ├── monitor.go        # 失败重试/告警 + 结果丢失监控
│   ├── report.go         # 日志报表聚合
│   ├── alarm.go          # 邮件告警
│   ├── openapi.go        # POST /api/{uri}（callback/registry/任务管理）
│   ├── webapi.go         # 管理台 REST
│   └── store.go          # 8 张表数据访问
├── web/                  # React 管理台（Vite + antd + echarts）
└── doc/tables_xxl_job.sql
```

## 开发模式

```bash
# 前端热更新开发（vite 代理到 8080）
cd web && npm install && npm run dev    # http://127.0.0.1:5173
cd web && npm run build                 # 构建到 web/dist，由 admin 直接托管
```

## 与原版的差异

- GLUE_GROOVY 不支持（Go 无 Groovy），脚本类 GLUE 完整支持
- 任务超时无法强杀 goroutine（Go 运行时限制），超时后放弃等待并按超时回调
- 登录为内存会话 + Cookie（原版 3.x 用 xxl-sso）；用户密码同样 SHA-256 存储
- 授权限的普通用户仅做了接口层角色校验，页面数据未按 permission 过滤

## 来源与致谢

本项目基于开源项目 [XXL-JOB](https://github.com/xuxueli/xxl-job)（v3.5.x，GPL-3.0）的架构与协议复刻而成，感谢原作者 [xuxueli（许雪里）](https://github.com/xuxueli) 的杰出工作。为保持许可一致性，本项目同样采用 [GPL-3.0](./LICENSE) 开源。

## License

[GPL-3.0](./LICENSE)，与上游 XXL-JOB 保持一致。

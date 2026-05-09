# Docker 生产级模板设计

## 背景

ncgo 当前生成的 Dockerfile 和 compose.yaml 仅适用于本地开发环境，缺少生产必需的健康检查、安全加固、资源限制等配置。同时用户手动执行 `docker compose` 命令，缺少 Makefile 统一入口。

## 目标

在 ncgo 生成的下游服务项目中：

1. 提供生产就绪的 Dockerfile（非 root、HEALTHCHECK、缓存优化）
2. 提供增强版 compose.yaml（healthcheck、restart、日志限制、环境变量参数化）
3. 在 Makefile 中添加 Docker 操作统一入口

**范围**：PostgreSQL、Redis、Polaris（配置中心，仅 micro 微服务 workspace 生成）。Kafka/ES/ClickHouse/etcd/Nacos 暂不改动。

## 架构

### Dockerfile 变更

| 新增项 | 说明 |
|--------|------|
| 模块缓存层 | 先 `COPY go.mod go.sum` → `go mod download`，再 `COPY . .` |
| 非 root 用户 | `adduser -u 1000 -G appgroup -D appuser` + `USER appuser` |
| HEALTHCHECK | 容器级 `wget --spider` 健康检查 |
| tzdata | Alpine 安装 `ca-certificates` 同时安装 `tzdata` |
| VERSION ARG | 构建参数注入 `-ldflags` 版本号 |

### compose.yaml 变更

| 新增项 | 说明 |
|--------|------|
| build args | `APP_NAME`、`PROFILE` 传入 Dockerfile |
| healthcheck | 应用服务、PostgreSQL、Redis、Polaris |
| depends_on condition | `condition: service_healthy` |
| restart 策略 | `unless-stopped` |
| 日志限制 | `json-file` + `max-size: 10m` + `max-file: 3` |
| 环境变量参数化 | 密码通过 `${VAR:-default}` 支持 `.env` 覆盖 |

### Makefile Docker Targets

| Target | 说明 |
|--------|------|
| `docker-build` | 构建镜像，支持 `PROFILE=prod` 和 `IMAGE_TAG=...` |
| `docker-up` | 启动所有服务 |
| `docker-down` | 停止并移除容器 |
| `docker-logs` | 流式查看应用日志 |
| `docker-shell` | 进入容器调试 |
| `docker-start` | 构建 + 启动 |
| `docker-rebuild` | 停止 → 构建 → 启动（完整重建） |

## 修改文件清单

| 文件 | 变更 |
|------|------|
| `internal/scaffold/micro/micro.go` | Micro workspace compose 添加 Polaris healthcheck/restart |
| `internal/scaffold/shared/container.go` | 单服务 compose 不生成配置中心 healthcheck/restart |
| `internal/assets/_data/hertz/layout.yaml` | Hertz Makefile 添加 Docker targets |
| `internal/assets/_data/kitex/kitex-template/makefile.yaml` | Kitex Makefile 添加 Docker targets |
| `internal/scaffold/mono/testdata/` | 更新 golden test fixtures |

## 风险

- Makefile 模板变更需同步更新 golden tests
- Dockerfile 变更可能影响现有使用 `compose up` 的用户（增加 build args 兼容）
- Polaris healthcheck/restart 仅在 micro workspace compose 中生成，单服务 compose 不生成

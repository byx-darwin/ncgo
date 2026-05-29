# 计划：`ncgo test rate-limit e2e` 一键限流测试

## 背景

当前 `ncgo test rate-limit` 拆分为 `seed` 和 `run` 两个独立命令，用户需要手动编排：

```bash
# 1. 启动 PostgreSQL
docker compose up -d postgres
# 2. 启动服务
make dev
# 3. 注入测试规则
ncgo test rate-limit seed
# 4. 执行压测
ncgo test rate-limit run --port 8080 --rate 200
```

需要一条 `e2e` 命令串联全流程，自动检测项目类型（单体/微服务）和限流配置（来源 + Redis 后端），完成从环境准备到结果分析的全链路验证。

## 命令设计

```
ncgo test rate-limit e2e [flags]
```

| 标志 | 默认值 | 说明 |
|------|--------|------|
| `--root` | `.` | 项目根目录 |
| `--host` | `localhost` | 服务地址 |
| `--port` | `8080` | HTTP 端口（可覆盖） |
| `--rate` | `200` | 压测 QPS |
| `--duration` | `10s` | 压测持续时间 |
| `--paths` | `/healthz,/` | 测试路径 |
| `--dsn` | 空 | PostgreSQL DSN（空则用 docker compose） |
| `--cleanup` | `true` | 完成后是否停止依赖 |
| `--report` | 空 | 输出报告文件路径（后缀 .md 或 .json） |
| `--output` | `text` | 输出格式：text 或 json |
| `--plan` | `false` | 等价于 `--output json`，不执行实际操作 |

## 项目类型检测逻辑

### 1. 检测部署模式（单体/微服务）

复用 `ai init` 已有的模式：

```
尝试 LoadWorkspace(ncgo.workspace)
    ↓ 成功                    ↓ 失败
  微服务模式                  尝试 Load(.ncgo/manifest.yaml)
    ↓                              ↓ 成功
  扫描 services[]                单体模式 (hertz/kitex)
  查找 rule-center               ↓
    ↓ 有                        ↓ 失败
  micro + rule-center            未知项目 → 报错
    ↓ 无
  micro 无 rule-center → 报错
```

### 2. 读取限流配置（conf.yaml）

从 `conf/dev/conf.yaml` 解析：

```yaml
rate_limit:
  source:
    type: config | database | rule_center  ← 规则来源
  backend: memory | redis                   ← 计数器后端
```

### 3. 依赖矩阵

| 来源 | 后端 | 需要启动 | 需要 seed |
|------|------|----------|-----------|
| config | memory | 仅服务本身 | 否（本地配置） |
| config | redis | Redis + 服务 | 否（本地配置） |
| database | memory | PostgreSQL + 服务 | 是 |
| database | redis | PostgreSQL + Redis + 服务 | 是 |
| rule_center | redis | PostgreSQL + Redis + rule-center + consumer | 是 |

## 流程编排

### 单体 config + memory（最简单）

```
[1/3] Starting service
[2/3] Running attack
[3/3] Analyzing results
```

### 单体 config + redis

```
[1/4] Starting redis (docker compose up -d redis)
[2/4] Starting service
[3/4] Running attack
[4/4] Analyzing results
```

### 单体 database + memory

```
[1/4] Starting postgres
[2/4] Starting service
[3/4] Seeding rules
[4/4] Running attack + analysis
```

### 单体 database + redis

```
[1/5] Starting postgres + redis
[2/5] Starting service
[3/5] Seeding rules
[4/5] Running attack
[5/5] Analyzing results
```

### 微服务 rule-center + redis

```
[1/6] Starting postgres + redis
[2/6] Starting rule-center
[3/6] Starting consumer (第一个 Hertz 服务)
[4/6] Seeding rules (通过 rule-center 的 PostgreSQL)
[5/6] Running attack (HTTP consumer) + gRPC verify (rule-center)
[6/6] Analyzing results
```

### 微服务无 rule-center

报错退出：`micro workspace has no rule-center service; e2e test requires rule-center`

## 实现文件

| 文件 | 变更 |
|------|------|
| `internal/scaffold/test/ratelimit/e2e.go` | 新增，e2e 编排逻辑 |
| `internal/scaffold/test/ratelimit/e2e_test.go` | 新增，编排逻辑测试 |
| `internal/cli/test.go` | 新增 `e2e` 子命令 |

## e2e.go 核心结构

```go
type E2EOptions struct {
    Root     string
    Host     string
    Port     int
    Rate     int
    Duration string
    Paths    []string
    DSN      string
    Cleanup  bool
    DryRun   bool
}

type E2EResult struct {
    Mode        string       // mono | micro
    Source      string       // config | database | rule_center
    Backend     string       // memory | redis
    Pass        bool         // true = 检测到限流
    TotalReqs   int          // 总请求数
    Status429   int          // 被限流响应数
    Status200   int          // 成功响应数
    StatusOther int          // 其他状态码数
    AvgLatency  time.Duration // 平均响应延迟
    P99Latency  time.Duration // P99 响应延迟
    StartedAt   time.Time    // e2e 开始时间
    Duration    time.Duration // 压测持续时间
    ReportPath  string       // 生成的报告文件路径
}

func E2E(ctx context.Context, opts E2EOptions) (*E2EResult, error)
```

流程步骤复用现有的 `Seed()` 和 `Run()` 函数，只做编排和结果分析：

- 读取 conf.yaml → 解析 source.type + backend
- 按需启动依赖：postgres / redis / rule-center / consumer
- 健康检查：轮询 HTTP 200 或超时（默认 30s，每 2s 重试）
- 调用 `Seed(ctx, SeedOptions{...})`（仅 database/rule_center 来源）
- 调用 `Run(ctx, RunOptions{...})` 并捕获 vegeta JSON 输出
- 解析 vegeta report JSON，统计 429/200 比例
- 可选 cleanup：`docker compose stop postgres redis ...`

## 健康检查

```go
func waitForReady(ctx context.Context, url string, interval time.Duration, timeout time.Duration) error {
    // GET url，每 interval 重试一次直到 timeout
    // 200 返回 nil，超时返回 error
}
```

## 结果分析

解析 vegeta JSON 输出（`vegeta attack -output results.json` + `vegeta report -type=json`），检查：

| 条件 | 判定 |
|------|------|
| 429 响应 > 0 且 < 100% | PASS — 限流生效且部分请求通过 |
| 429 响应 = 0 | FAIL — 限流未触发 |
| 429 响应 = 100% | WARN — 规则过严，所有请求被拒绝 |
| 服务不可达 | FAIL — 环境准备失败 |

## 测试报告

### 报告输出格式

```
ncgo test rate-limit e2e --report rate-limit-report.md
ncgo test rate-limit e2e --report rate-limit-report.json
```

根据文件后缀自动生成 Markdown 或 JSON 格式报告。

### Markdown 报告内容

```markdown
# 限流 E2E 测试报告

- **生成时间**: 2026-05-18T10:30:00Z
- **项目模式**: mono / micro
- **规则来源**: config / database / rule_center
- **计数器后端**: memory / redis
- **服务**: user-api (Hertz)

## 测试参数

| 参数 | 值 |
|------|-----|
| 目标地址 | http://localhost:8080 |
| 测试路径 | /healthz, / |
| 压测速率 | 200 rps |
| 持续时间 | 10s |
| 总请求数 | 2,000 |

## 结果

| 指标 | 值 |
|------|-----|
| 状态 | ✅ PASS / ❌ FAIL / ⚠️ WARN |
| 200 OK | 1,200 (60.0%) |
| 429 请求过多 | 800 (40.0%) |
| 其他错误 | 0 (0.0%) |
| 平均延迟 | 12ms |
| P99 延迟 | 45ms |

## 注入的规则

| 阶段 | 方法 | 路径 | 匹配类型 | 策略 | 窗口 | 最大请求 |
|------|------|------|----------|------|------|----------|
| pre_auth | * | * | exact | fixed_window | 60s | 10 |
| pre_auth | GET | /healthz | exact | fixed_window | 60s | 10 |
| grpc | * | * | exact | fixed_window | 60s | 10 |
```

## 验证

1. `go test ./internal/scaffold/test/ratelimit/... -count=1`
2. `go test ./internal/cli/... -count=1`
3. `go build ./... && go vet ./...`

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
[4/6] Seeding rules (via PostgreSQL in rule-center)
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
    Pass        bool         // true = rate limiting detected
    TotalReqs   int          // total requests sent
    Status429   int          // rate-limited responses
    Status200   int          // successful responses
    StatusOther int          // other status codes
    AvgLatency  time.Duration // average response latency
    P99Latency  time.Duration // p99 response latency
    StartedAt   time.Time    // when e2e started
    Duration    time.Duration // attack duration
    ReportPath  string       // path to generated report
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
    // GET url, retry every interval until timeout
    // Return nil on 200, error on timeout
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
# Rate Limit E2E Test Report

- **Generated at**: 2026-05-18T10:30:00Z
- **Project mode**: mono / micro
- **Rule source**: config / database / rule_center
- **Counter backend**: memory / redis
- **Service**: user-api (Hertz)

## Test Parameters

| Parameter | Value |
|-----------|-------|
| Target URL | http://localhost:8080 |
| Test paths | /healthz, / |
| Rate | 200 rps |
| Duration | 10s |
| Total requests | 2,000 |

## Results

| Metric | Value |
|--------|-------|
| Status | ✅ PASS / ❌ FAIL / ⚠️ WARN |
| 200 OK | 1,200 (60.0%) |
| 429 Too Many Requests | 800 (40.0%) |
| Other errors | 0 (0.0%) |
| Avg latency | 12ms |
| P99 latency | 45ms |

## Timeline

```
[████████████████████] 10s attack complete

  0s ──────────────────────────────────── 10s
  ██████████ 200 OK (200 rps target)
           ████████ 429 Rate Limited
```

## Rule Details

Rules seeded (database/rule_center mode only):

| Phase | Method | Path | Match Kind | Strategy | Window | Max Reqs |
|-------|--------|------|------------|----------|--------|----------|
| pre_auth | * | * | exact | fixed_window | 60s | 10 |
| pre_auth | GET | /healthz | exact | fixed_window | 60s | 10 |
| grpc | * | * | exact | fixed_window | 60s | 10 |

## Environment

- PostgreSQL: running via docker compose
- Redis: running via docker compose
- Rule-center: running (port 8888)
- Consumer: user-api running (port 8080)
```

### JSON 报告内容

```json
{
  "generatedAt": "2026-05-18T10:30:00Z",
  "mode": "mono",
  "source": "database",
  "backend": "redis",
  "service": "user-api",
  "testParams": {
    "targetUrl": "http://localhost:8080",
    "paths": ["/healthz", "/"],
    "rate": 200,
    "duration": "10s",
    "totalRequests": 2000
  },
  "results": {
    "status": "PASS",
    "status200": 1200,
    "status429": 800,
    "statusOther": 0,
    "avgLatencyMs": 12,
    "p99LatencyMs": 45
  },
  "rules": [
    {
      "phase": "pre_auth",
      "method": "*",
      "path": "*",
      "matchKind": "exact",
      "strategy": "fixed_window",
      "windowSeconds": 60,
      "maxRequests": 10
    }
  ],
  "environment": {
    "postgres": "running",
    "redis": "running",
    "ruleCenter": null,
    "consumer": "running"
  }
}
```

## 验证

1. `go test ./internal/scaffold/test/ratelimit/... -count=1`
2. `go test ./internal/cli/... -count=1`
3. `go build ./... && go vet ./...`

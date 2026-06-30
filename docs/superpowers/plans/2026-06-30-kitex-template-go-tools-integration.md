<!-- Issue: #N -->

# Kitex Template go-tools Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update kitex-template scaffold files to integrate go-tools packages (go-common/log, go-framework/config, go-framework/kitex/*) while preserving existing functionality and ncgo anchors.

**Architecture:** Modify 6 template YAML files under `internal/assets/_data/kitex/kitex-template/` in the specified order: main.yaml → conf.yaml → conf_dev.yaml → rpcerror.yaml → interceptor.yaml → server.yaml. Then update golden test snapshots and run integration validation. Changes are template-only (no Go logic changes in generators).

**Tech Stack:** Go 1.25+, go-tools (go-common/log, go-common/error, go-framework/config, go-framework/config/kitex, go-framework/kitex/*), OTel observability provider, pgx/v5（保留不变）

## Global Constraints

- 必须保留 ncgo anchors（`// ncgo:wire:*` 注释），确保 `ncgo add infra` 正常工作
- 不能使用 `go-framework/kitex/option`（`//go:build ignore` 阻止）
- 不能替换 `pgx/v5/pgxpool` 为 `go-middleware/db`（pgx 特有功能 Batch/CopyFrom/LISTEN）
- Config 字段类型变更（`int` 秒 → `time.Duration`）需要 config 文件格式同步更新
- 模板变更后必须更新 golden test 快照
- 所有 commit message 引用 Issue #N

## Issue 规划

**Issue 标题:** feat: integrate go-tools packages into kitex-template scaffold

**Issue 标签:** enhancement,kitex,go-tools,scaffold,priority:high

**Issue 描述:**
更新 kitex-template 脚手架模板，集成 go-tools 生态包（go-common/log 结构化日志、go-framework/config 配置加载、go-common/error 统一错误码、go-framework/kitex/middleware 访问日志、go-framework/kitex/observability OTel 追踪）。保留 ncgo anchors 和 pgx 依赖，不引入被 build ignore 禁用的 go-framework/kitex/option。Config 时间字段从 int 秒迁移到 time.Duration 字符串格式。

**验收标准:**
- [ ] 所有任务完成
- [ ] 测试通过（单元测试 + 集成测试 + golden tests）
- [ ] 代码审查通过
- [ ] 文档更新
- [ ] 覆盖率 > 80%

**关联:**
- 计划文件: `docs/superpowers/plans/2026-06-30-kitex-template-go-tools-integration.md`
- 设计文档: `docs/superpowers/specs/01-kitex-template-go-tools-integration.md`
- 里程碑: v2.0 kitex-template modernization

## File Structure

```
internal/assets/_data/kitex/kitex-template/
├── main.yaml                    # ✏️ 修改 — 集成 go-common/log
├── conf.yaml                    # ✏️ 修改 — 集成 go-framework/config.LoadYAML + kitexconfig.ServerConfig
├── conf_dev.yaml                # ✏️ 修改 — config 格式切换（int 秒 → time.Duration 字符串）
├── server.yaml                  # ✏️ 修改 — 集成 observability + 字段访问路径迁移
├── interceptor.yaml             # ✏️ 修改 — 集成 middleware.AccessLog()
├── interceptor_test.yaml        # ✏️ 修改 — AccessLog 测试更新（如有自定义实现）
├── rpcerror.yaml                # ✏️ 修改 — 集成 go-common/error + rpcerror.OopsStatusAdapter
├── rpcerror_test.yaml           # ✏️ 修改 — 错误码测试更新
├── data.yaml                    # 不变 — 保留 pgx/v5/pgxpool
├── handler.yaml                 # 不变
├── usecase.yaml                 # 不变
├── repository.yaml              # 不变
├── client.yaml                  # 不变
├── client_test.yaml             # 不变
├── makefile.yaml                # 不变
├── migration_*.yaml             # 不变
└── ratelimit_*.yaml             # 不变

internal/scaffold/mono/testdata/  # ✏️ 更新 golden test 快照
```

## Tasks

### Task 1: 创建 Issue

**Description:** 从 "Issue 规划" 部分提取信息，创建 Issue 并保存编号到 `.claude/gh-issue/current-issue.txt`。

- [ ] **Step 1: 运行 scripts/create-issue.sh**

```bash
bash /Users/byx/.claude/skills/writing-plans-with-issue/scripts/create-issue.sh docs/superpowers/plans/2026-06-30-kitex-template-go-tools-integration.md
```

- [ ] **Step 2: 验证 Issue 已创建**

```bash
cat .claude/gh-issue/current-issue.txt
gh issue view "$(cat .claude/gh-issue/current-issue.txt)"
```

### Task 2: 同步 Issue 状态为 in-progress

**Description:** 将 Issue 状态更新为 `status: in-progress`，表示开发已开始。

- [ ] **Step 1: 运行 scripts/sync-status.sh**

```bash
bash /Users/byx/.claude/skills/writing-plans-with-issue/scripts/sync-status.sh in-progress
```

- [ ] **Step 2: 确认**

```bash
echo "✅ Issue #$(cat .claude/gh-issue/current-issue.txt) 已标记为 in-progress"
```

### Task 3: 更新 main.yaml — 集成 go-common/log

**Description:** 修改 `main.yaml` 模板，将 `log` 标准库替换为 `go-common/log` 结构化日志初始化。

- [ ] **Step 1:** 在 `main.yaml` 中添加 `go-common/log` import（alias: `goclog`）
- [ ] **Step 2:** 在 `conf.Init()` 之后插入 `goclog.Init()` 调用块
- [ ] **Step 3:** 在 `main()` 函数末尾插入 `defer goclog.Close()`（`server.Run()` 之前）
- [ ] **Step 4:** 保留标准 `log` import（用于配置初始化失败时的致命日志）
- [ ] **Step 5:** 参考设计文档 4.1 节验证生成的代码

**Commit:** `feat(kitex-template): integrate go-common/log into main.yaml (#N)`

### Task 4: 更新 conf.yaml — 集成 go-framework/config 类型

**Description:** 修改 `conf.yaml` 模板，使用 `config.LoadYAML[T]()` 和 `kitexconfig.ServerConfig` 替换自定义配置加载。

- [ ] **Step 1:** 添加 `go-framework/config` 和 `go-framework/config/kitex` import
- [ ] **Step 2:** 将 `Config.Server` 字段类型改为 `kitexconfig.ServerConfig`
- [ ] **Step 3:** 添加 `Config.Debug`、`Config.Log`、`Config.Jaeger` 字段
- [ ] **Step 4:** 将 `Load()` 函数改为使用 `config.LoadYAML[Config](path)` 替换自定义 yaml 解析
- [ ] **Step 5:** 添加 `Validate()` 方法（空实现，预留校验扩展点）
- [ ] **Step 6:** 将 `Default()` 函数更新为使用 `kitexconfig.ServerConfig` 的新字段结构（RPC/Timeout/Registry 子结构体）
- [ ] **Step 7:** 将 `Get()` 单例函数更新为 `Get() *Config` 返回类型（如果之前不是）
- [ ] **Step 8:** 参考设计文档 4.2 节验证

**Commit:** `feat(kitex-template): integrate go-framework/config into conf.yaml (#N)`

### Task 5: 更新 conf_dev.yaml — 配置格式迁移

**Description:** 修改 `conf_dev.yaml` 模板，将配置格式从 `int` 秒迁移到 `time.Duration` 字符串。

- [ ] **Step 1:** 将 `server.name` → `server.registry.name`
- [ ] **Step 2:** 将 `server.addr` → `server.rpc.port`
- [ ] **Step 3:** 将 `server.read_write_timeout_seconds` → `server.timeout.read_write_timeout`（值从 `30` 改为 `"30s"`）
- [ ] **Step 4:** 将 `server.exit_wait_time_seconds` → `server.timeout.exit_wait_timeout`（值从 `10` 改为 `"10s"`）
- [ ] **Step 5:** 添加 `log:`、`database:` 配置块
- [ ] **Step 6:** 添加 `server.rpc.network: "tcp"` 字段
- [ ] **Step 7:** 参考设计文档 4.2 节 config file changes 部分验证

**Commit:** `feat(kitex-template): migrate config format to time.Duration (#N)`

### Task 6: 更新 rpcerror.yaml — 集成 go-common/error

**Description:** 修改 `rpcerror.yaml` 模板，使用 `go-common/error` 预定义错误码和 `rpcerror.OopsStatusAdapter`。

- [ ] **Step 1:** 添加 `go-common/error` 和 `go-framework/kitex/rpcerror` import
- [ ] **Step 2:** 将 `CodeInternalError` 等常量改为引用 `goerror.Code*` 预定义
- [ ] **Step 3:** 将 `ToBizError()` 实现改为使用 `rpcerror.OopsStatusAdapter{Err: err}`
- [ ] **Step 4:** 更新 `rpcerror_test.yaml` 中的错误码测试（如果测试引用了具体错误码数值）
- [ ] **Step 5:** 移除不再需要的自定义常量和 `samber/oops` 间接引用（如果 oops 只在 rpcerror 中使用）
- [ ] **Step 6:** 参考设计文档 4.5 节验证

**Commit:** `feat(kitex-template): integrate go-common/error and rpcerror adapter (#N)`

### Task 7: 更新 interceptor.yaml — 集成 middleware.AccessLog()

**Description:** 修改 `interceptor.yaml` 模板，将自定义 AccessLog 实现替换为 `go-framework/kitex/middleware.AccessLog()`。

- [ ] **Step 1:** 添加 `go-framework/kitex/middleware` import
- [ ] **Step 2:** 将 `AccessLog()` 函数体改为 `return middleware.AccessLog()`
- [ ] **Step 3:** 删除模板中自定义 AccessLog 实现代码
- [ ] **Step 4:** 更新 `interceptor_test.yaml` 中的 AccessLog 测试（如果存在自定义行为的测试）
- [ ] **Step 5:** 保留其他自定义拦截器（RequestID、CallerAllowlist、Recovery、RequestTimeout）不变
- [ ] **Step 6:** 参考设计文档 4.4 节验证

**Commit:** `feat(kitex-template): integrate middleware.AccessLog() into interceptor.yaml (#N)`

### Task 8: 更新 server.yaml — 集成 observability + 字段迁移

**Description:** 修改 `server.yaml` 模板，集成 OTel observability 提供者，更新 Config 字段访问路径。

- [ ] **Step 1:** 添加 `go-framework/kitex/observability` 和 `go-framework/config` import
- [ ] **Step 2:** 迁移 Config 字段访问：`cfg.Server.Addr` → `cfg.Server.RPC.Port`
- [ ] **Step 3:** 迁移 Config 字段访问：`cfg.Server.ReadWriteTimeoutSeconds` → `cfg.Server.Timeout.ReadWriteTimeout`
- [ ] **Step 4:** 迁移 Config 字段访问：`cfg.Server.ExitWaitTimeSeconds` → `cfg.Server.Timeout.ExitWaitTimeout`
- [ ] **Step 5:** 迁移 Config 字段访问：`cfg.Server.Name` → `cfg.Server.Registry.Name`
- [ ] **Step 6:** 在 `Run()` 函数中添加 OTel observability 初始化代码块（`if cfg.Jaeger != nil && cfg.Jaeger.Enable`）
- [ ] **Step 7:** 移除不再需要的 `durationSeconds()` 辅助函数对服务器配置字段的使用（保留用于 RPC 请求超时等其他位置）
- [ ] **Step 8:** 保留所有 ncgo anchors 不变（`// ncgo:wire:*`）
- [ ] **Step 9:** 保留 `go-framework/kitex/rpcerror` 和 `go-framework/kitex/middleware` 引用（已有）
- [ ] **Step 10:** 参考设计文档 4.3 节验证

**Commit:** `feat(kitex-template): integrate observability and migrate server config fields (#N)`

### Task 9: 更新 Golden Tests

**Description:** 更新 `internal/scaffold/mono/testdata/` 下的 golden test 快照以匹配模板变更。

- [ ] **Step 1:** 运行 golden test 更新命令

```bash
go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -update-golden -count=1
```

- [ ] **Step 2:** 检查 golden 快照 diff，确保只包含预期的 go-tools 集成变更
- [ ] **Step 3:** 确认 golden 快照中保留了所有 ncgo anchors
- [ ] **Step 4:** 确认 golden 快照中没有引入 `go-framework/kitex/option` import

**Commit:** `test: update golden snapshots for kitex go-tools integration (#N)`

### Task 10: 全量验证

**Description:** 运行完整的构建、测试和代码检查。

- [ ] **Step 1:** 构建检查

```bash
go build ./... && go build .
```

- [ ] **Step 2:** 代码检查

```bash
go vet ./...
```

- [ ] **Step 3:** 全量测试

```bash
go test ./... -count=1
```

- [ ] **Step 4:** Smoke test

```bash
./scripts/smoke.sh
```

- [ ] **Step 5:** 生成一个 Kitex 项目验证编译（手动验证）

```bash
go run . new --mode mono --kind kitex /tmp/test-kitex-gotools
cd /tmp/test-kitex-gotools && go build ./...
```

### Task 11: 收尾 — 本地合并后关闭 Issue

**Description:** 开发完成并本地合并到 base 分支后，push 并关闭 Issue。

> **注意：** 如果选择 PR 路径（Option 2），则不需要此任务 — `link-pr.sh` 会在 PR body 中包含 `Closes #N`，PR 合并时平台自动关闭 Issue。

- [ ] **Step 1: 确保已合并到 base 分支**

```bash
git branch --show-current  # 应该在 main/master 上
```

- [ ] **Step 2: 运行 scripts/finish-issue.sh（含验收 checkbox 同步）**

`finish-issue.sh` 会自动：
1. 将 Issue `## 验收标准` 下的 `- [ ]` 替换为 `- [x]`
2. Push base 分支
3. 关闭 Issue
4. 清理 local state

```bash
bash /Users/byx/.claude/skills/writing-plans-with-issue/scripts/finish-issue.sh
```

- [ ] **Step 3: 确认 Issue 已关闭且 checkbox 已打钩**

```bash
gh issue view "$(cat .claude/gh-issue/current-issue.txt 2>/dev/null || echo 'already cleaned')"
```

<!-- Issue: #N -->

# Hertz Template per-file yaml + go-tools Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `internal/assets/_data/hertz/hertz-template/*.yaml`（12 个模板文件），对齐 Kitex 的 per-file yaml + Go text/template 维护模式，集成 go-tools 包，并修改 layout.yaml 和 scaffold 代码以支持新的生成流水线。

**Architecture:** 新建 hertz-template 目录，编写 12 个模板文件（main、conf、server、data、usecase、repository、response、errcode、middleware、makefile、sqlc、conf_dev），精简 layout.yaml 中将被 hertz-template cover 的部分（删除 body），扩展 `writeHertzTemplate()` 复制模板到生成项目。分工：handler/router/pb/i18n 仍由 hz 生成，main/conf/server/DDD 层由 hertz-template 接管。

**Tech Stack:** Go 1.25+, go-tools（go-common/log、go-common/error、go-framework/config、go-framework/config/hertz、go-framework/hertz、go-framework/hertz/middleware、go-middleware/db）

**依赖:** `01-kitex-template-go-tools-integration`（已完成）

## Global Constraints

- 模板格式必须和 kitex-template 一致（per-file yaml + Go text/template）
- handler.go / router_gen.go / pb / i18n 保持由 hz 生成，不能覆盖
- `response.OK`/`response.Err` 包级函数签名必须兼容 `package.yaml` 中的 handler 模板
- `UseCase()`/`Repository` 等接口签名必须和现有 handler 模板兼容（初始化 + 调用方式）
- layout.yaml 精简时不能误删 hz 依赖的条目
- `go-framework/hertz/observability`（OTel）由 `NewHTTPServer` 内部处理，模板无需直接导入
- 所有 commit message 引用 Issue #N

## GitHub Issue 规划

**Issue 标题:** feat: create hertz-template with per-file yaml and go-tools integration

**Issue 标签:** enhancement,hertz,go-tools,scaffold,priority:high

**Issue 描述:**
新建 hertz-template 脚手架模板目录（internal/assets/_data/hertz/hertz-template/），包含 12 个 per-file yaml + Go text/template 格式的模板文件。集成 go-tools 生态包（go-common/log、go-framework/config/hertz、go-framework/hertz、go-framework/hertz/middleware、go-common/error、go-middleware/db）。同步精简 layout.yaml（删除将被 cover 的文件 body），扩展 writeHertzTemplate() 复制模板到生成项目。分工原则：handler/router/pb/i18n 仍由 hz 原生工具链生成，main/conf/server/DDL 层由 hertz-template 接管。

**验收标准:**
- [ ] 所有任务完成
- [ ] 测试通过（单元测试 + 集成测试 + golden tests）
- [ ] 代码审查通过
- [ ] 文档更新
- [ ] `ncgo new --mode mono --kind hertz --name test-svc` 生成可编译运行的项目
- [ ] 覆盖率 > 80%

**关联:**
- 计划文件: `docs/superpowers/plans/2026-06-30-hertz-template-go-tools-integration.md`
- 设计文档: `docs/superpowers/specs/02-hertz-template-design.md`
- 依赖: `docs/superpowers/specs/01-kitex-template-go-tools-integration.md`（已完成）
- 里程碑: v2.0 hertz-template modernization

## File Structure

```
internal/assets/_data/hertz/hertz-template/   # 🆕 新建目录
├── main_go.yaml              # 🆕 入口 — go-common/log 初始化
├── conf_go.yaml              # 🆕 配置 — config/hertz.ServerConfig
├── conf_dev_yaml.yaml        # 🆕 dev 配置样本
├── server_go.yaml            # 🆕 服务器 — hertz.NewHTTPServer
├── data_go.yaml              # 🆕 数据层 — go-middleware/db（条件: WithDatabase）
├── usecase_go.yaml           # 🆕 用例层 — loop_service, skip 策略
├── repository_go.yaml        # 🆕 仓库层 — loop_service, skip 策略（条件: WithDatabase）
├── response_go.yaml          # 🆕 响应 — hertz.Responder
├── errcode_go.yaml           # 🆕 错误码 — go-common/error
├── middleware_go.yaml        # 🆕 中间件 — hertz/middleware 重导出
├── makefile_yaml.yaml        # 🆕 Makefile
└── sqlc_yaml.yaml            # 🆕 sqlc 配置（条件: WithDatabase）

internal/assets/_data/hertz/layout.yaml        # ✏️ 修改 — 精简 body
internal/scaffold/mono/files.go                # ✏️ 修改 — writeHertzTemplate() 扩展
internal/scaffold/mono/testdata/               # ✏️ 更新 golden test 快照
```

## Tasks

### Task 1: 创建 Issue

**Description:** 从 "Issue 规划" 部分提取信息，创建 Issue 并保存编号到 `.claude/gh-issue/current-issue.txt`。

- [ ] **Step 1: 运行 scripts/create-issue.sh**

```bash
bash /Users/byx/.claude/skills/writing-plans-with-issue/scripts/create-issue.sh docs/superpowers/plans/2026-06-30-hertz-template-go-tools-integration.md
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

### Task 3: 创建 hertz-template 目录并编写核心模板（main + conf + conf_dev）

**Description:** 创建 `internal/assets/_data/hertz/hertz-template/` 目录，编写入口、配置和 dev 配置样本三个模板文件。

- [ ] **Step 1:** 创建目录 `internal/assets/_data/hertz/hertz-template/`
- [ ] **Step 2:** 编写 `main_go.yaml` — `go-common/log` 初始化（参考设计文档 5.1 节）
- [ ] **Step 3:** 编写 `conf_go.yaml` — `config.LoadYAML[T]()` + `hertzconfig.ServerConfig`（参考设计文档 5.2 节）
- [ ] **Step 4:** 编写 `conf_dev_yaml.yaml` — dev 环境配置样本（参考设计文档 5.2 节 config 格式）
- [ ] **Step 5:** 验证 yaml 格式正确（`path:` + `update_behavior:` + `body:` 三段式）

**Commit:** `feat(hertz-template): add main, conf, conf_dev templates (#N)`

### Task 4: 编写 server_go.yaml — 服务器入口

**Description:** 编写服务器模板，使用 `go-framework/hertz.NewHTTPServer()`，设置 Responder 和 AccessLog 中间件。

- [ ] **Step 1:** 编写 `server_go.yaml` — `NewHTTPServer()` + `responder.Middleware()` + `AccessLog()` + 路由注册（参考设计文档 5.3 节）
- [ ] **Step 2:** 确保 OTel 不直接导入（由 `NewHTTPServer` 内部处理）
- [ ] **Step 3:** 保留 DDD 层 wiring 注释（`// ncgo:wire:*` 风格占位符）
- [ ] **Step 4:** 验证：`UseCase()`/`Repository` 接口签名和 handler 初始化方式兼容

**Commit:** `feat(hertz-template): add server template with NewHTTPServer (#N)`

### Task 5: 编写 response_go.yaml + errcode_go.yaml

**Description:** 编写 HTTP 统一响应和错误码模板。

- [ ] **Step 1:** 编写 `response_go.yaml` — `hertz.Responder` 封装（参考设计文档 5.4 节）
- [ ] **Step 2:** 确保 `response.OK(ctx, data)` / `response.Err(ctx, c, err, msg)` 包级函数签名和 `package.yaml` 中 handler 模板兼容
- [ ] **Step 3:** 编写 `errcode_go.yaml` — `go-common/error` 预定义错误码重导出（参考设计文档 5.6 节）

**Commit:** `feat(hertz-template): add response and errcode templates (#N)`

### Task 6: 编写 middleware_go.yaml

**Description:** 编写中间件模板，重导出 `go-framework/hertz/middleware` 的 `Cors`、`AccessLog` 和 `Auth`。

- [ ] **Step 1:** 编写 `middleware_go.yaml`（参考设计文档 5.5 节）
- [ ] **Step 2:** `Cors` / `AccessLog` 无参，直接重导出为 `var`
- [ ] **Step 3:** `Auth` 需要 `AuthFace` 参数，包装为函数

**Commit:** `feat(hertz-template): add middleware template (#N)`

### Task 7: 编写 DDD 层模板（usecase + repository + data）

**Description:** 编写用例层、仓库层和数据层模板，使用 skip 策略保护用户修改，条件生成 WithDatabase。

- [ ] **Step 1:** 编写 `usecase_go.yaml` — `loop_service`, `update_behavior: skip`，和 kitex usecase.yaml 对称（参考设计文档 5.7 节）
- [ ] **Step 2:** 编写 `repository_go.yaml` — `loop_service`, `update_behavior: skip`, 条件 `WithDatabase`（参考设计文档 5.8 节）
- [ ] **Step 3:** 编写 `data_go.yaml` — 条件 `WithDatabase`, 使用 `go-middleware/db`（参考设计文档 5.9 节）

**Commit:** `feat(hertz-template): add DDD layer templates (usecase, repository, data) (#N)`

### Task 8: 编写构建和数据库配置模板（makefile + sqlc）

**Description:** 编写项目构建和 sqlc 配置模板。

- [ ] **Step 1:** 编写 `makefile_yaml.yaml` — 和 kitex makefile.yaml 对称（参考设计文档 5.10 节）
- [ ] **Step 2:** 编写 `sqlc_yaml.yaml` — 条件 `WithDatabase`（参考设计文档 5.11 节）

**Commit:** `feat(hertz-template): add makefile and sqlc templates (#N)`

### Task 9: 精简 layout.yaml

**Description:** 删除 layout.yaml 中将由 hertz-template cover 的文件的 body 内容，只保留 hz 负责的文件。

- [ ] **Step 1:** 列出 layout.yaml 中需要删除 body 的文件：`main.go`、`conf.go`、`server.go`、`response.go`、`errcode.go`、`middleware*.go`、`data.go`、`conf/dev/conf.yaml`
- [ ] **Step 2:** 逐一删除 body 内容，改为空目录条目
- [ ] **Step 3:** 确认保留 hz 负责的文件条目不变：`handler.go`、`router*.go`、`pb/*.go`、`i18n/*`、`docs/*`、`go.mod`、`.gitignore`
- [ ] **Step 4:** 检查 layout.yaml 中无引用 hertz-template 将要生成的文件路径

**Commit:** `refactor(hertz): slim layout.yaml — remove bodies covered by hertz-template (#N)`

### Task 10: 扩展 writeHertzTemplate() — 复制 hertz-template yaml 到生成项目

**Description:** 在 `internal/scaffold/mono/files.go` 中扩展 `writeHertzTemplate()`，从嵌入资源复制 `hertz/hertz-template/*.yaml` 到生成项目的 `template/hertz-template/` 目录。

- [ ] **Step 1:** 在 `writeHertzTemplate()` 中添加读取 `hertz/hertz-template/` 嵌入资源的逻辑
- [ ] **Step 2:** 创建 `template/hertz-template/` 子目录
- [ ] **Step 3:** 将所有 yaml 文件复制过去
- [ ] **Step 4:** 确认 `template.Apply()` 能正确读取并执行 hertz-template 的覆盖（现有代码已支持，不需要修改）

**Commit:** `feat(scaffold): extend writeHertzTemplate to copy hertz-template yaml (#N)`

### Task 11: 更新 Golden Tests

**Description:** 更新 golden test 快照以匹配新增的 hertz-template 和 layout.yaml 精简。

- [ ] **Step 1:** 运行 golden test 更新

```bash
go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -update-golden -count=1
```

- [ ] **Step 2:** 检查 golden diff，确认新增了 hertz-template 模板内容
- [ ] **Step 3:** 确认 layout.yaml 精简的条目在 golden 快照中正确反映
- [ ] **Step 4:** 确认 hertz-template 模板内容包含正确的 go-tools import 和函数签名

**Commit:** `test: update golden snapshots for hertz-template (#N)`

### Task 12: 全量验证

**Description:** 运行完整的构建、测试和端到端验证。

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

- [ ] **Step 5:** 生成 Hertz 项目并验证编译

```bash
go run . new --mode mono --kind hertz --name test-hertz /tmp/test-hertz-gotools
cd /tmp/test-hertz-gotools && go build ./...
```

- [ ] **Step 6:** 确认生成的 main.go 包含 go-common/log import
- [ ] **Step 7:** 确认生成的 conf.go 包含 hertzconfig.ServerConfig
- [ ] **Step 8:** 确认生成的 server.go 使用 NewHTTPServer
- [ ] **Step 9:** 确认 handler.go / router_gen.go 仍由 hz 生成且签名兼容

### Task 13: 收尾 — 本地合并后关闭 Issue

**Description:** 开发完成并本地合并到 base 分支后，push 并关闭 Issue。

> **注意：** 如果选择 PR 路径（Option 2），则不需要此任务 — `link-pr.sh` 会在 PR body 中包含 `Closes #N`，PR 合并时平台自动关闭 Issue。

- [ ] **Step 1: 确保已合并到 base 分支**

```bash
git branch --show-current  # 应该在 main/master 上
```

- [ ] **Step 2: 运行 scripts/finish-issue.sh（含验收 checkbox 同步）**

```bash
bash /Users/byx/.claude/skills/writing-plans-with-issue/scripts/finish-issue.sh
```

- [ ] **Step 3: 确认 Issue 已关闭且 checkbox 已打钩**

```bash
gh issue view "$(cat .claude/gh-issue/current-issue.txt 2>/dev/null || echo 'already cleaned')"
```

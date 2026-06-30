<!-- Issue: #N -->

# Scaffold Code Changes — hertz-template Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 scaffold 系统的代码变更以支持 hertz-template 流水线：扩展 `writeHertzTemplate()` 复制模板、精简 `layout.yaml`、添加 `hasInfra` 辅助函数、新增集成测试、更新 golden tests。

**Architecture:** 本计划覆盖将 hertz-template 挂接到 scaffold 生成流水线的所有代码级变更。模板内容本身由计划 01（kitex）和 02（hertz）负责；本计划专注 scaffold 引擎层：文件复制逻辑、layout 精简、FuncMap 扩展、测试。`template.Apply()` 无需修改（已支持读取 hertz-template 目录）。

**Tech Stack:** Go 1.25+, `embed.FS`, `text/template`, `os.WriteFile`, `fs.ReadDir`

**依赖:** `01-kitex-template-go-tools-integration` + `02-hertz-template-design`（模板内容已就绪后执行本计划）

## Global Constraints

- `writeHertzTemplate()` 必须与现有代码风格一致（panic-free、显式错误返回）
- layout.yaml 精简方式：`body: |-\n...` → `body: ""`（保留条目作为目录占位，hz 仍能创建目录结构）
- hz 必须在 `Apply()` 之前运行（hz 创建目录结构 + handler/router/pb，Apply 覆盖/补充 DDD 层）
- `hasInfra` 函数签名必须和现有 `FuncMap()` 模式一致
- 修改 layout.yaml 时不能误删 hz 依赖的条目（handler、router、pb、i18n、docs、go.mod）
- 所有 commit message 引用 Issue #N

## Issue 规划

**Issue 标题:** feat: scaffold engine changes for hertz-template integration

**Issue 标签:** enhancement,scaffold,hertz,go-tools,priority:high

**Issue 描述:**
实现 scaffold 引擎层的代码变更以完成 hertz-template 集成：扩展 writeHertzTemplate() 将 hertz-template/*.yaml 从嵌入资源复制到生成项目；精简 layout.yaml 删除已被 hertz-template cover 的文件 body；添加 hasInfra 到 FuncMap 支持条件 infra 渲染；新增集成测试验证 hertz-template 复制和应用流程。

**验收标准:**
- [ ] 所有任务完成
- [ ] 测试通过（单元测试 + 集成测试 + golden tests）
- [ ] 代码审查通过
- [ ] 文档更新
- [ ] `ncgo new --mode mono --kind hertz` 生成可编译项目
- [ ] `ncgo new --mode mono --kind kitex` 生成可编译项目（不受影响）
- [ ] 覆盖率 > 80%

**关联:**
- 计划文件: `docs/superpowers/plans/2026-06-30-scaffold-code-changes.md`
- 设计文档: `docs/superpowers/specs/03-scaffold-code-changes.md`
- 依赖: `01-kitex-template-go-tools-integration`、`02-hertz-template-design`
- 里程碑: v2.0 scaffold pipeline

## File Structure

```
internal/scaffold/mono/files.go          # ✏️ 修改 — writeHertzTemplate() 扩展
internal/assets/_data/hertz/layout.yaml  # ✏️ 修改 — 精简 body（8 个文件条目）
internal/scaffold/template/types.go      # ✏️ 修改 — FuncMap() 添加 hasInfra
internal/scaffold/mono/mono_test.go      # ✏️ 新增 — TestGenerateHertzTemplateApplied
internal/scaffold/mono/testdata/         # ✏️ 更新 golden test 快照
internal/scaffold/template/apply.go      # 不变 — 已支持读取 hertz-template
internal/scaffold/mono/mono.go           # 不变 — 已调用 writeHertzTemplate + Apply
```

## Tasks

### Task 1: 创建 Issue

**Description:** 从 "Issue 规划" 部分提取信息，创建 Issue 并保存编号到 `.claude/gh-issue/current-issue.txt`。

- [ ] **Step 1: 运行 scripts/create-issue.sh**

```bash
bash /Users/byx/.claude/skills/writing-plans-with-issue/scripts/create-issue.sh docs/superpowers/plans/2026-06-30-scaffold-code-changes.md
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

### Task 3: 扩展 writeHertzTemplate() — 复制 hertz-template yaml

**Description:** 在 `internal/scaffold/mono/files.go` 的 `writeHertzTemplate()` 函数中添加逻辑，将嵌入资源 `hertz/hertz-template/*.yaml` 复制到生成项目的 `template/hertz-template/` 目录。

- [ ] **Step 1:** 在 `template/` 目录创建后，添加 `hertz-template/` 子目录创建
- [ ] **Step 2:** 使用 `fs.ReadDir(srcFS, "hertz/hertz-template")` 读取嵌入资源目录条目
- [ ] **Step 3:** 遍历条目，跳过目录和非 `.yaml` 文件
- [ ] **Step 4:** 将每个 yaml 文件复制到 `template/hertz-template/` 下
- [ ] **Step 5:** 如果 `hertz/hertz-template/` 目录不存在（模板尚未添加），返回 `nil`（非致命）
- [ ] **Step 6:** 对文件读取/写入错误返回带上下文的包装错误
- [ ] **Step 7:** 确保 `rule_center_client.go` 的现有逻辑不被影响（放在新代码之后）
- [ ] **Step 8:** 参考设计文档第 3 节 Change 4 的实现示例

**Commit:** `feat(scaffold): copy hertz-template yaml from embedded assets in writeHertzTemplate (#N)`

### Task 4: 精简 layout.yaml — 删除被 hertz-template cover 的文件 body

**Description:** 将 `internal/assets/_data/hertz/layout.yaml` 中 8 个文件条目的 `body` 内容替换为空字符串，保留条目作为目录占位。

- [ ] **Step 1:** 逐一定位以下条目的 `body:` 字段（8 个）：
  | 文件路径 | 理由 |
  |---|---|
  | `main.go` | 由 hertz-template main_go.yaml cover |
  | `internal/base/conf/conf.go` | 由 hertz-template conf_go.yaml cover |
  | `internal/base/server/server.go` | 由 hertz-template server_go.yaml cover |
  | `internal/pkg/response/response.go` | 由 hertz-template response_go.yaml cover |
  | `internal/pkg/errcode/errcode.go` | 由 hertz-template errcode_go.yaml cover |
  | `internal/pkg/middleware/*.go` | 由 hertz-template middleware_go.yaml cover |
  | `internal/base/data/data.go` | 由 hertz-template data_go.yaml cover |
  | `conf/dev/conf.yaml` | 由 hertz-template conf_dev_yaml.yaml cover |

- [ ] **Step 2:** 将每个条目的 `body: |-\n...` 替换为 `body: ""`
- [ ] **Step 3:** 确认保留 hz 负责的文件条目不变：
  - `internal/handler/*/handler.go`（含 body — hz package.yaml 生成）
  - `internal/router/*.go`（含 body — hz 路由生成）
  - `internal/pb/*.go`
  - `internal/pkg/i18n/*`
  - `internal/docs/*`
  - `internal/handler/health/health.go`
  - `go.mod`
  - `internal/pkg/ratelimit/*`
  - `tools/i18n/*`

- [ ] **Step 4:** 检查 layout.yaml 中无残留的对 hertz-template 目标文件路径的 body 内容引用
- [ ] **Step 5:** 参考设计文档第 3 节 Change 3

**Commit:** `refactor(hertz): trim layout.yaml — delegate files to hertz-template (#N)`

### Task 5: 添加 hasInfra 辅助函数到 FuncMap

**Description:** 在 `internal/scaffold/template/types.go` 的 `FuncMap()` 中添加 `hasInfra` 函数，支持模板中按 infra 类型条件渲染。

- [ ] **Step 1:** 在 `FuncMap()` 的返回 map 中添加 `"hasInfra": hasInfra`
- [ ] **Step 2:** 添加 `hasInfra` 包级函数，实现字符串数组包含检查：

```go
func hasInfra(infra []string, name string) bool {
    for _, kind := range infra {
        if kind == name {
            return true
        }
    }
    return false
}
```

- [ ] **Step 3:** 确认签名和现有 FuncMap 模式一致
- [ ] **Step 4:** 添加 `hasInfra` 的单元测试（`types_test.go`，如已存在）
- [ ] **Step 5:** 参考设计文档第 3 节 Change 6

**Commit:** `feat(template): add hasInfra helper to FuncMap for conditional infra rendering (#N)`

### Task 6: 添加集成测试 — TestGenerateHertzTemplateApplied

**Description:** 在 `internal/scaffold/mono/mono_test.go` 中添加集成测试，验证 `writeHertzTemplate()` 正确复制 hertz-template yaml 到生成项目。

- [ ] **Step 1:** 添加 `TestGenerateHertzTemplateApplied` 测试函数
- [ ] **Step 2:** 使用 `Options{Kind: manifest.KindHertz, NoGenerate: true}` 避免运行真实 hz
- [ ] **Step 3:** 验证 `template/hertz-template/` 目录存在
- [ ] **Step 4:** 验证目录中有 `.yaml` 文件
- [ ] **Step 5:** 可选：验证关键文件内容包含 go-tools import（如 `main_go.yaml` 含 `go-common/log`）
- [ ] **Step 6:** 参考设计文档第 4.2 节的测试代码

**Commit:** `test(scaffold): add integration test for hertz-template copy (#N)`

### Task 7: 更新 Golden Tests

**Description:** 更新 `internal/scaffold/mono/testdata/` golden 快照以匹配所有变更。

- [ ] **Step 1:** 运行 golden test 更新

```bash
go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -update-golden -count=1
```

- [ ] **Step 2:** 检查 golden diff，确认：
  - layout.yaml 中被精简的条目 body 为空
  - hertz-template yaml 文件出现在项目树中
  - kitex-template 变更正确反映（如果有交叉依赖）

- [ ] **Step 3:** 确认 golden 快照中没有意外引入 `go-framework/kitex/option`（已知禁用）

**Commit:** `test: update golden snapshots for scaffold engine changes (#N)`

### Task 8: 全量验证

**Description:** 运行完整的构建、测试和端到端验证，确认两种模板类型均可正常生成。

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

- [ ] **Step 5:** 生成 Hertz 项目验证

```bash
go run . new --mode mono --kind hertz --name test-hz /tmp/test-hz-scaffold
cd /tmp/test-hz-scaffold && go build ./...
```

- [ ] **Step 6:** 生成 Kitex 项目验证（确认未受 layout/代码改动影响）

```bash
go run . new --mode mono --kind kitex --name test-kx /tmp/test-kx-scaffold
cd /tmp/test-kx-scaffold && go build ./...
```

- [ ] **Step 7:** 确认 `template/hertz-template/` 目录在生成项目中存在

```bash
ls /tmp/test-hz-scaffold/template/hertz-template/
```

### Task 9: 收尾 — 本地合并后关闭 Issue

**Description:** 开发完成并本地合并到 base 分支后，push 并关闭 Issue。

- [ ] **Step 1: 确保已合并到 base 分支**

```bash
git branch --show-current
```

- [ ] **Step 2: 运行 scripts/finish-issue.sh**

```bash
bash /Users/byx/.claude/skills/writing-plans-with-issue/scripts/finish-issue.sh
```

- [ ] **Step 3: 确认 Issue 已关闭且 checkbox 已打钩**

```bash
gh issue view "$(cat .claude/gh-issue/current-issue.txt 2>/dev/null || echo 'already cleaned')"
```

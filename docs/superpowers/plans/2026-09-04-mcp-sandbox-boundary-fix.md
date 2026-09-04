# MCP 工具沙箱边界校验补齐 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 8 个未受保护的 MCP 工具入口补齐 `sandboxRoot()` 调用，堵住工作区外任意文件读写风险（Issue #97），同时保持现有集成测试全部通过。

**Architecture:** 复用 `internal/mcp/sandbox.go` 中已存在的 `sandboxRoot(target string) (string, error)`，在每个工具的参数解析完成、发起任何文件系统操作之前调用它；若返回 error，走既有的 `return textResult(err.Error(), true), nil` 错误分支（与现有 5 个已合规工具 `tool_bff.go` / `tool_export_templates.go` / `tool_extract.go` / `tool_rpc.go` / `tool_upgrade.go` 完全一致的模式）。因为约 36 个既有测试用 `t.TempDir()` 或仓库内其它包目录（都在被测包 `internal/mcp` 的 cwd 之外）伪造工作区根目录，新增校验会让它们全部失败，因此同时引入一个测试专用的 `resolvePath` 放宽 helper（沿用 `tool_template_test.go` 已有先例）。

**Tech Stack:** Go 1.25+，标准库 `testing`，无新增依赖。

**Spec:** `docs/superpowers/specs/2026-09-04-mcp-sandbox-boundary-fix-design.md`

## Global Constraints

- 复用现有 `sandboxRoot()` 实现，不新增校验逻辑（Issue #97 验收标准第 1 条）。
- `internal/mcp` 是 contract-sensitive 包：MCP 工具的 `content[0].text` 与顶层结构化字段形状不得改变，本次改动只在参数校验阶段插入一次早退（early return），不改变成功路径的输出结构。
- `ncgo_check`（`tool_check.go`）与 `ncgo_import`（`tool_import.go`）不在本次范围内，禁止修改。
- 不得削弱生产代码的沙箱校验：测试 helper 只允许在 `_test.go` 文件里覆盖 `resolvePath` 变量，且必须通过 `t.Cleanup` 还原。

---

## Task 1: 新增越界路径拒绝测试（RED）

**Files:**
- Create: `internal/mcp/tool_sandbox_test.go`

**Interfaces:**
- Consumes：`EncodeMessage`、`DecodeResponses`、`New(ncgoVersion, assetsVersion string) *Server`、`(*Server).Serve(ctx, io.Reader, io.Writer) error`（均已存在于 `internal/mcp` 包，签名参考 `server_test.go:19-35`）。
- Produces：无新增导出符号，仅新增测试函数。

新增一个表驱动测试，覆盖 Issue #97 列出的 8 个工具，每个工具用两类越界输入调用一次：绝对路径逃逸（`/etc`）与相对路径 `../` 逃逸（`../../../etc`）。这一步是 RED：此时生产代码尚未加校验，除已合规的 5 个工具外，新测试对这 8 个工具应该失败（因为它们目前会尝试执行到底层调用并因为路径不存在等原因报出与"outside the workspace"无关的错误，甚至可能因为路径真实存在于系统而"成功"），验证方式见 Step 2。

- [ ] **Step 1: 写测试文件**

```go
package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestSandboxRootRejectsEscapePaths verifies every MCP tool that accepts a
// root/dir parameter rejects paths outside the workspace (cwd), whether via
// an absolute path or a "../" relative escape. This locks in the fix for
// Issue #97 (sandbox boundary validation was missing on 8 tool entrypoints).
func TestSandboxRootRejectsEscapePaths(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{name: "ncgo_new", args: map[string]any{"name": "demo", "module": "github.com/x/demo", "noGenerate": true}},
		{name: "ncgo_i18n_report", args: map[string]any{}},
		{name: "ncgo_i18n_check", args: map[string]any{}},
		{name: "ncgo_protolint", args: map[string]any{"files": []string{"app/demo.proto"}}},
		{name: "ncgo_doctor", args: map[string]any{}},
		{name: "ncgo_add_rule_center", args: map[string]any{"addr": "127.0.0.1:8888"}},
		{name: "ncgo_ai_sync", args: map[string]any{}},
		{name: "ncgo_ai_init_claude", args: map[string]any{}},
		{name: "ncgo_ai_context", args: map[string]any{}},
		{name: "ncgo_add_infra", args: map[string]any{"kind": "redis"}},
		{name: "ncgo_add_method", args: map[string]any{"spec": "device.Get"}},
	}
	escapes := map[string]string{
		"absolute": "/etc",
		"relative": "../../../etc",
	}

	for _, tc := range cases {
		for escapeName, escapePath := range escapes {
			t.Run(tc.name+"/"+escapeName, func(t *testing.T) {
				args := map[string]any{}
				for k, v := range tc.args {
					args[k] = v
				}
				rootKey := "root"
				if tc.name == "ncgo_new" {
					rootKey = "dir"
				}
				args[rootKey] = escapePath

				input := EncodeMessage(map[string]any{
					"jsonrpc": "2.0", "id": 1, "method": "tools/call",
					"params": map[string]any{"name": tc.name, "arguments": args},
				})
				var out bytes.Buffer
				if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
					t.Fatalf("Serve: %v", err)
				}
				responses, err := DecodeResponses(out.Bytes())
				if err != nil {
					t.Fatalf("DecodeResponses: %v", err)
				}
				result := responses[0].Result.(map[string]any)
				if !result["isError"].(bool) {
					t.Fatalf("%s with %s escape %q unexpectedly succeeded: %+v", tc.name, escapeName, escapePath, result)
				}
				text := resultText(result)
				if !strings.Contains(text, "outside the workspace") {
					t.Fatalf("%s with %s escape %q: content = %q, want it to contain %q", tc.name, escapeName, escapePath, text, "outside the workspace")
				}
			})
		}
	}
}
```

- [ ] **Step 2: 运行测试，确认当前失败**

Run: `go test ./internal/mcp/... -run TestSandboxRootRejectsEscapePaths -v`

Expected: 除已合规工具外的子测试 FAIL（错误信息不包含 "outside the workspace"，或调用本身未返回 error）。记录失败的子测试列表，作为 Task 2 的验收依据。

---

## Task 2: 生产代码补齐 `sandboxRoot()` 调用（GREEN）

**Files:**
- Modify: `internal/mcp/tool_new.go:84-89`
- Modify: `internal/mcp/tool_i18n.go:103-118`（`callI18NReport`）和 `internal/mcp/tool_i18n.go:131-146`（`callI18NCheck`）
- Modify: `internal/mcp/tool_protolint.go:23-38`
- Modify: `internal/mcp/tool_doctor.go:23-30`
- Modify: `internal/mcp/tool_rulecenter.go:11-26`
- Modify: `internal/mcp/tool_ai.go:31-43`（`callAISync`）和 `internal/mcp/tool_ai.go:58-69`（`callAIInitClaude`）
- Modify: `internal/mcp/tool_ai_context.go:13-19`
- Modify: `internal/mcp/tool_add.go:22-34`（`callAddInfra`）和 `internal/mcp/tool_add.go:76-86`（`callAddMethod`）

**Interfaces:**
- Consumes：`sandboxRoot(target string) (string, error)`（已存在于 `internal/mcp/sandbox.go:44`，不改动其实现）。
- Produces：无新签名，只在既有函数体内插入早退分支。

对每个位置插入的代码统一为：

```go
if _, err := sandboxRoot(args.Root); err != nil {
    return textResult(err.Error(), true), nil
}
```

`tool_new.go` 例外——它的目标参数叫 `dir` 不叫 `root`，且 `dir` 是从 `args.Dir`（若非空）或 `args.Name` 计算得来的，插入位置在 `dir := args.Name` / `if args.Dir != "" { dir = args.Dir }` 之后、`switch args.Mode` 之前：

```go
if _, err := sandboxRoot(dir); err != nil {
    return textResult(err.Error(), true), nil
}
```

逐文件改动：

- [ ] **Step 1: `tool_new.go` — 在 `callNew` 中校验 `dir`**

在 `internal/mcp/tool_new.go` 第 84-89 行：

```go
	var res *newResult
	dir := args.Name
	if args.Dir != "" {
		dir = args.Dir
	}
	switch args.Mode {
```

改为：

```go
	var res *newResult
	dir := args.Name
	if args.Dir != "" {
		dir = args.Dir
	}
	if _, err := sandboxRoot(dir); err != nil {
		return textResult(err.Error(), true), nil
	}
	switch args.Mode {
```

- [ ] **Step 2: `tool_i18n.go` — 在 `callI18NReport` 和 `callI18NCheck` 中校验 `Root`**

`callI18NReport`（第 103-114 行）：

```go
func callI18NReport(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := i18nReportMCPTool.resolveOutput(args.Output)
```

改为：

```go
func callI18NReport(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := i18nReportMCPTool.resolveOutput(args.Output)
```

`callI18NCheck`（第 131-140 行）：

```go
func callI18NCheck(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Mode   string `json:"mode"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := i18nCheckMCPTool.resolveOutput(args.Output)
```

改为：

```go
func callI18NCheck(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Mode   string `json:"mode"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := i18nCheckMCPTool.resolveOutput(args.Output)
```

- [ ] **Step 3: `tool_protolint.go` — 在 `callProtolint` 中校验 `Root`**

第 23-38 行：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Root) == "" {
		return textResult("protolint: root is required", true), nil
	}
	output, err := protolintMCPTool.resolveOutput(args.Output)
```

改为：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Root) == "" {
		return textResult("protolint: root is required", true), nil
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := protolintMCPTool.resolveOutput(args.Output)
```

- [ ] **Step 4: `tool_doctor.go` — 在 `callDoctor` 中校验 `Root`**

第 23-30 行：

```go
func (s *Server) callDoctor(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	output, err := doctorMCPTool.resolveOutput(args.Output)
```

改为：

```go
func (s *Server) callDoctor(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := doctorMCPTool.resolveOutput(args.Output)
```

- [ ] **Step 5: `tool_rulecenter.go` — 在 `callAddRuleCenter` 中校验 `Root`**

第 11-26 行：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Addr == "" {
		return textResult("addr is required", true), nil
	}

	output, err := addRuleCenterMCPTool.resolveOutput(args.Output)
```

改为：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Addr == "" {
		return textResult("addr is required", true), nil
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}

	output, err := addRuleCenterMCPTool.resolveOutput(args.Output)
```

- [ ] **Step 6: `tool_ai.go` — 在 `callAISync` 和 `callAIInitClaude` 中校验 `Root`**

`callAISync`（第 31-43 行）：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := aiSyncMCPTool.resolveOutput(args.Output)
```

改为：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := aiSyncMCPTool.resolveOutput(args.Output)
```

`callAIInitClaude`（第 58-69 行）：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := aiInitClaudeMCPTool.resolveOutput(args.Output)
```

改为：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := aiInitClaudeMCPTool.resolveOutput(args.Output)
```

（这两个函数的参数结构体都各自内联定义了同名 `Root string` 字段，插入位置各自作用域内的 `args.Root` 均指向正确的局部变量。）

- [ ] **Step 7: `tool_ai_context.go` — 在 `callAIContext` 中校验 `Root`**

第 13-19 行：

```go
func callAIContext(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	s, err := scan.Scan(args.Root)
```

改为：

```go
func callAIContext(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	s, err := scan.Scan(args.Root)
```

- [ ] **Step 8: `tool_add.go` — 在 `callAddInfra` 和 `callAddMethod` 中校验 `Root`**

`callAddInfra`（第 22-34 行）：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := addInfraMCPTool.resolveOutput(args.Output)
```

改为：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := addInfraMCPTool.resolveOutput(args.Output)
```

`callAddMethod`（第 76-86 行）：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := addMethodMCPTool.resolveOutput(args.Output)
```

改为：

```go
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := addMethodMCPTool.resolveOutput(args.Output)
```

- [ ] **Step 9: 编译 + 运行 Task 1 的拒绝测试，确认转绿**

Run: `go build ./... && go test ./internal/mcp/... -run TestSandboxRootRejectsEscapePaths -v`

Expected: 所有子测试 PASS。

- [ ] **Step 10: 运行整个 `internal/mcp` 包测试，记录回归失败清单**

Run: `go test ./internal/mcp/... -count=1 2>&1 | tee /tmp/mcp-test-before-fix.log`

Expected: 大量既有测试 FAIL（因为它们用 `t.TempDir()` 等工作区外路径作为 root/dir），这是预期的、Task 3 要修复的回归——不要在这里去"修" production 代码来迁就它们。

- [ ] **Step 11: Commit**

```bash
git add internal/mcp/tool_sandbox_test.go internal/mcp/tool_new.go internal/mcp/tool_i18n.go internal/mcp/tool_protolint.go internal/mcp/tool_doctor.go internal/mcp/tool_rulecenter.go internal/mcp/tool_ai.go internal/mcp/tool_ai_context.go internal/mcp/tool_add.go
git commit -m "fix(mcp): enforce sandboxRoot on 8 previously unvalidated tool entrypoints"
```

---

## Task 3: 修复因新增校验而失效的既有测试

**Files:**
- Modify: `internal/mcp/server_test.go`
- Modify: `internal/mcp/tool_new_test.go`

**Interfaces:**
- Consumes：`resolvePath`（包级变量，类型 `func(string) (string, error)`，定义于 `internal/mcp/sandbox.go:13`）；`filepath.Abs`（标准库）。
- Produces：新增测试专用函数 `allowAnyRootForTest(t *testing.T)`，供本任务及未来测试复用。

- [ ] **Step 1: 在 `server_test.go` 新增共享测试 helper**

在 `internal/mcp/server_test.go` 文件末尾（`resultJSONObject` 函数之后）追加：

```go
// allowAnyRootForTest relaxes the workspace-boundary check that sandboxRoot
// enforces via resolvePath, for tests that use t.TempDir() or a sibling
// package directory (both outside the internal/mcp package's cwd during `go
// test`) as a stand-in workspace root. Mirrors the override already used by
// tool_template_test.go for the same reason.
func allowAnyRootForTest(t *testing.T) {
	t.Helper()
	orig := resolvePath
	resolvePath = func(target string) (string, error) { return filepath.Abs(target) }
	t.Cleanup(func() { resolvePath = orig })
}
```

- [ ] **Step 2: 在每个受影响的测试函数体第一行调用 `allowAnyRootForTest(t)`**

在 `internal/mcp/server_test.go` 中，为以下测试函数体的第一行（紧跟 `func TestXxx(t *testing.T) {` 之后）插入 `allowAnyRootForTest(t)`：

- `TestServeToolCallDoctor`
- `TestServeToolCallDoctorSARIF`
- `TestServeToolCallDoctorInvalidOutput`
- `TestServeToolCallAIInitClaude`
- `TestServeToolCallAIInitClaudeJSON`
- `TestServeToolCallAIInitClaudeInvalidOutput`
- `TestServeToolCallAISyncIncludesStructuredFields`
- `TestServeToolCallAISyncJSON`
- `TestServeToolCallAISyncTargetParam`
- `TestServeToolCallAISyncInvalidOutput`
- `TestServeToolCallAIContext`
- `TestServeToolCallAIContextNoManifest`
- `TestServeToolCallAddInfra`
- `TestServeToolCallI18NReport`
- `TestServeToolCallI18NReportJSON`
- `TestServeToolCallI18NReportMissing`
- `TestServeToolCallI18NReportInvalidOutput`
- `TestServeToolCallI18NCheckDev`
- `TestServeToolCallI18NCheckJSON`
- `TestServeToolCallI18NCheckRelease`
- `TestServeToolCallI18NCheckInvalidMode`
- `TestServeToolCallI18NCheckInvalidOutput`
- `TestServeToolCallProtolintOK`
- `TestServeToolCallProtolintFailure`
- `TestServeToolCallProtolintSARIF`
- `TestServeToolCallProtolintWarningsAreNonBlocking`
- `TestServeToolCallProtolintIgnoreRule`
- `TestServeToolCallProtolintWorkspaceAutoDiscovery`
- `TestServeToolCallProtolintInvalidArgs`
- `TestServeToolCallAddInfraDryRun`
- `TestServeToolCallAddInfraJSON`
- `TestServeToolCallAddInfraInvalidOutput`
- `TestServeToolCallNew`
- `TestServeToolCallNewMicro`
- `TestServeToolCallAddMethodJSON`

例如 `TestServeToolCallDoctor`（第 144 行起）：

```go
func TestServeToolCallDoctor(t *testing.T) {
	old := runDoctorReport
```

改为：

```go
func TestServeToolCallDoctor(t *testing.T) {
	allowAnyRootForTest(t)
	old := runDoctorReport
```

对上面清单中其余 33 个函数，按同样方式在函数体第一行插入 `allowAnyRootForTest(t)`，插入点始终是 `func TestXxx(t *testing.T) {` 这一行之后、原函数体第一条语句之前，不改动函数体其余任何逻辑或断言。

不要对 `TestServeToolCallNewMissingModule`、`TestServeToolCallAddDomain`、`TestServeToolCallAddDomainDryRun`、`TestServeInitializeAndToolsList` 等未使用越界 root/dir 或调用范围外工具（`ncgo_add_domain` 属于 `tool_domain.go`，不在本次修复范围）的测试函数做任何改动。

- [ ] **Step 3: 在 `tool_new_test.go` 中同样处理 `TestServeToolCallNewAutoStepArgs`**

`internal/mcp/tool_new_test.go` 第 153 行起：

```go
func TestServeToolCallNewAutoStepArgs(t *testing.T) {
	dir := t.TempDir()
```

改为：

```go
func TestServeToolCallNewAutoStepArgs(t *testing.T) {
	allowAnyRootForTest(t)
	dir := t.TempDir()
```

- [ ] **Step 4: 运行整个 `internal/mcp` 包测试，确认全部通过**

Run: `go test ./internal/mcp/... -count=1 -v 2>&1 | tee /tmp/mcp-test-after-fix.log`

Expected: 全部 PASS，且 `TestSandboxRootRejectsEscapePaths` 的所有子测试仍然 PASS（确认 helper 只放宽了测试里显式调用它的函数，不影响 Task 1 新增的拒绝测试，因为该测试函数没有调用 `allowAnyRootForTest`）。

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/server_test.go internal/mcp/tool_new_test.go
git commit -m "test(mcp): relax workspace sandbox for existing tests using out-of-tree fixtures"
```

---

## Task 4: 全量验证 + 文档核实

**Files:**
- Verify only（无预期改动）: `internal/mcp/sandbox.go`

**Interfaces:**
- 无新增/变更接口。

- [ ] **Step 1: 核实 `sandbox.go` 注释准确性**

Run: `grep -rn "sandboxRoot(" internal/mcp/*.go | grep -v _test`

Expected: 输出恰好包含 13 个调用点（原有 5 个 + 本次新增 8 个：`tool_new.go` 1 处、`tool_i18n.go` 2 处、`tool_protolint.go` 1 处、`tool_doctor.go` 1 处、`tool_rulecenter.go` 1 处、`tool_ai.go` 2 处、`tool_ai_context.go` 1 处、`tool_add.go` 2 处，共 13 处校验语句，覆盖 `internal/mcp` 目录下全部接受 root/dir 的 MCP 工具入口）。`internal/mcp/sandbox.go:43` 的注释"It is called from every MCP tool handler that accepts a root or dir parameter."此时已准确，无需改动。

- [ ] **Step 2: 运行完整仓库验证**

Run: `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`

Expected: 全部通过，无失败用例，无 vet 警告。

- [ ] **Step 3: gofmt 检查**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`

Expected: 空输出（无未格式化文件）。

- [ ] **Step 4: Commit（若 Step 1-3 产生任何格式化改动）**

```bash
git status --porcelain
# 若有 gofmt 产生的改动：
git add -u
git commit -m "chore(mcp): gofmt after sandbox boundary fix"
```

若 Step 1-3 均为验证性质、无文件改动，跳过本步。

# Design: MCP 工具沙箱边界校验补齐

- Issue: #97
- Date: 2026-09-04
- Classification: bounded（在已有的 `sandboxRoot()` 沙箱机制上补齐调用点，无新增校验逻辑）

## Context

`internal/mcp/sandbox.go` 提供了 `sandboxRoot()`，用于校验目标路径必须落在当前工作区（cwd）内，拒绝绝对路径或 `../` 逃逸。目前只有 5 个工具调用它：`tool_bff.go`、`tool_export_templates.go`、`tool_extract.go`、`tool_rpc.go`、`tool_upgrade.go`。

以下 8 个文件的工具入口未做校验，只做 `filepath.Abs` 或直接透传 `root`/`dir` 参数：

- `tool_new.go`（`ncgo_new` 的 `dir` 参数）
- `tool_i18n.go`（report / check）
- `tool_protolint.go`
- `tool_doctor.go`
- `tool_rulecenter.go`
- `tool_ai.go`（sync / init claude）
- `tool_ai_context.go`
- `tool_add.go`（add infra / add method）

恶意或被 prompt injection 操控的 MCP client 传入绝对路径或 `../../` 可能导致工作区外任意文件读写。

## Goal

在上述工具入口统一补上 `sandboxRoot()` 调用，复用现有实现，不新增校验逻辑；同时不破坏现有集成测试。

## Approach

### 1. 生产代码改动（8 个文件）

在每个文件的工具入口，在参数解析完成、发起任何文件读写前，插入与现有 5 个已合规工具一致的校验模式：

```go
if _, err := sandboxRoot(args.Root); err != nil {
    return textResult(err.Error(), true), nil
}
```

- `tool_new.go`：`callNew` 对计算出的 `dir`（`args.Dir` 或回退到 `args.Name`）做校验，在 `switch args.Mode` 分支前。
- `tool_i18n.go`：`callI18NReport`、`callI18NCheck` 分别对 `args.Root` 校验。
- `tool_protolint.go`：`callProtolint` 对 `args.Root` 校验（在非空检查之后）。
- `tool_doctor.go`：`(s *Server) callDoctor` 对 `args.Root` 校验。
- `tool_rulecenter.go`：`callAddRuleCenter` 对 `args.Root` 校验。
- `tool_ai.go`：`callAISync`、`callAIInitClaude` 分别对 `args.Root` 校验。
- `tool_ai_context.go`：`callAIContext` 对 `args.Root` 校验。
- `tool_add.go`：`callAddInfra`、`callAddMethod` 分别对 `args.Root` 校验。

### 2. 文档更新

`sandbox.go:43` 上 `sandboxRoot` 的注释已经写"每个接受 root/dir 的工具都会调用"——完成上述改动后这句话就是准确的，本次不改动注释文案本身，只需确认其准确性（若发现遗漏工具，一并补上）。

### 3. 测试基础设施修复（意外发现，需一并处理）

`internal/mcp/server_test.go` 与 `tool_new_test.go` 中约 30 处测试用例使用 `t.TempDir()`（系统临时目录，在 cwd 之外）作为上述 8 个工具的 `root`/`dir` 测试输入。补上校验后，这些测试会因"路径在工作区外"而全部失败。

修复沿用仓库已有先例（`tool_template_test.go:357-359` 对 `resolvePath` 的处理方式）：新增共享测试 helper，例如：

```go
// allowAnyRootForTest relaxes the workspace-boundary check for tests that
// use t.TempDir() (outside cwd) as a stand-in workspace root.
func allowAnyRootForTest(t *testing.T) {
    t.Helper()
    orig := resolvePath
    resolvePath = func(target string) (string, error) { return filepath.Abs(target) }
    t.Cleanup(func() { resolvePath = orig })
}
```

在受影响的测试函数开头调用一行（Doctor / AIInitClaude / AISync / AIContext / AddInfra / I18NReport / I18NCheck / Protolint / AddMethod / New / NewMicro 相关测试，共约 30 个测试函数）。这不削弱生产代码的沙箱校验，只是让测试可以继续使用临时目录模拟工作区。

### 4. 新增拒绝测试

新增 `internal/mcp/tool_sandbox_test.go`，表驱动测试覆盖上述 8 个工具，分别用：
- 绝对路径逃逸（如 `/etc`）
- 相对路径 `../` 逃逸

断言 `Serve` 返回 `isError: true` 且错误文本包含 `"outside the workspace"`。

## Testing

- 新增 `tool_sandbox_test.go` 验证越界路径被拒绝。
- 修复因新增校验而失败的约 30 个既有测试（通过 `allowAnyRootForTest` helper）。
- `go build ./... && go vet ./... && go test ./internal/mcp/... -count=1`。
- 全量验证：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。

## Out of Scope

- `tool_check.go`（`ncgo_check`）与 `tool_import.go`（`ncgo_import`）：均为只读 / preview-only 工具，不在 Issue #97 列出的范围内，本次不改动。
- `tool_extract.go` 中 `To` 参数、`tool_new.go` 中 `Module`/`Name` 等非路径参数：不在沙箱校验范围内，维持现状。

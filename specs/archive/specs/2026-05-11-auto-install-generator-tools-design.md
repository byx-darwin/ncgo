# Auto-Install Generator Tools Design

**Date:** 2026-05-11
**Status:** Approved

## Problem

`go install github.com/byx-darwin/ncgo@latest` 只安装 ncgo 本身。当用户运行 `ncgo new` 时，如果 `hz` 或 `kitex` 不在 PATH 上，生成器调用会失败，用户需要手动安装后再重试。这个流程对新手不够友好。

## Goal

在 `ncgo new` 生成项目前，自动检测并安装缺失的生成器工具（hz/kitex），减少用户手动操作。

## Design

### Flow

```
ncgo new user-api --module xxx
  │
  ├── [1] 预检（仅当没有 --no-generate）
  │     ├── 根据 --kind 判断所需工具（hertz→hz，kitex→kitex）
  │     ├── 检查工具是否在 PATH 上
  │     ├── 全部存在 → 直接进入 [2]
  │     └── 有缺失 → 列出缺失项
  │           ├── 提示: "未找到以下工具:"
  │           ├── 列出: hz (>= v0.9.7), 安装命令
  │           └── 询问: "是否自动安装? [Y/n]"
  │                 ├── Y/回车 → 执行 go install，成功后进入 [2]
  │                 └── n → 终止，提示用户手动安装
  │
  ├── [2] mono.Generate（现有逻辑不变）
  │
  └── [3] 输出结果
```

### Components

#### `preflightTools(kind string, noGenerate bool, w io.Writer, r io.Reader) error`

位置: `internal/cli/root.go`

- `kind`: `manifest.KindHertz` 或 `manifest.KindKitex`，决定需要检查哪个工具
- `noGenerate`: 为 `true` 时跳过预检
- `w`: 输出（通常是 `cmd.OutOrStdout()`）
- `r`: 输入（通常是 `cmd.InOrStdin()`），用于读取用户确认
- 返回 `nil` 表示检查通过或已安装成功；返回 error 表示用户拒绝或安装失败

#### 安装逻辑

- 使用 `os/exec.Command("go", "install", <path>@latest)` 执行安装
- hz: `github.com/cloudwego/hertz/cmd/hz@latest`
- kitex: `github.com/cloudwego/kitex/tool/cmd/kitex@latest`
- 安装失败时返回带安装提示的 error，终止流程

### Files Changed

| File | Change |
|---|---|
| `internal/cli/root.go` | 新增 `preflightTools` 函数；在 `runNewMono` 中调用 |
| `internal/cli/root_test.go` | 新增预检交互场景的测试 |

### Error Handling

- 安装失败: 返回错误，包含 `go install` 失败输出 + 手动安装提示
- 用户拒绝安装: 返回错误，提示手动安装命令
- 网络/权限问题: 由 `go install` 的自然错误传达

### Testing

- 单元测试: 测试 `preflightTools` 在工具存在/缺失/用户拒绝/安装成功等场景的行为
- 集成测试: 在 `root_test.go` 中验证 `ncgo new` 完整流程

### Out of Scope

- `ncgo add rpc` / `ncgo add bff` 等 micro 命令的预检（后续可扩展）
- `ncgo doctor` 的重构（当前 doctor 的检查逻辑保持不变）
- 版本号校验（安装始终用 `@latest`，doctor 继续负责版本检查）

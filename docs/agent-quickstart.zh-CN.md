# Agent 快速开始

本文帮助 Codex、Claude Code 或 Cursor 连接 ncgo，并给 Code Agent 一套稳定的
生成项目探测、修改与验证流程。

English version: [agent-quickstart.md](agent-quickstart.md)。

## 1. 先识别当前仓库

- ncgo 源码仓库包含 `main.go`、`internal/cli/` 和 `internal/scaffold/`，遵循根目录
  `AGENTS.md`。
- Mono 服务包含 `.ncgo/manifest.yaml`。
- Micro 工作区包含 `ncgo.workspace`，成员服务各自包含 `.ncgo/manifest.yaml`。

不要根据目录名猜项目类型，应读取元数据文件。

## 2. 安装并验证 ncgo

```bash
go install github.com/byx-darwin/ncgo@latest
command -v ncgo
ncgo version
```

如果桌面客户端不会继承终端的 `PATH`，请在 MCP 配置中使用
`command -v ncgo` 返回的绝对路径。

在生成项目中先渲染所有 Agent 目标：

```bash
ncgo ai sync --root . --target all
ncgo check --root . --output json
```

这会生成通用 `AGENTS.md`，以及 Claude 和 Cursor 所需的上下文文件。

## 3. 连接 MCP 客户端

`ncgo mcp serve` 是本地 stdio server。它继承客户端的当前工作目录，MCP 文件操作
受该工作区边界限制。请从目标仓库根目录启动客户端。

所有 MCP `root`、`dir`、`templateDir`、`to` 与显式文件输入都必须使用相对路径。
含 `..` 的路径、绝对路径、解析后指向工作区外的 symlink，以及 dangling symlink，
都会在工具读写文件前被拒绝。解析目标仍在工作区内的 symlink 可以使用；尚未创建的
目标通过最近存在的父目录校验。按名称选择的 registry 模板来自 ncgo 管理的缓存。

Server 同时继承客户端环境。只读工具只需要 `ncgo` 二进制；生成工作流可能调用
`go`、`hz`、`kitex`、`protoc` 或 `sqlc`。请确保这些工具位于继承的 `PATH` 中，
必要时使用客户端的 MCP 环境变量配置。不要把密钥写入项目共享的 MCP 配置。

### Codex

本机 Codex CLI 通过 `codex mcp add` 配置 stdio MCP server：

```bash
codex mcp add ncgo -- /absolute/path/to/ncgo mcp serve
codex mcp get ncgo
```

可用 `codex mcp remove ncgo` 删除配置。

### Claude Code

```bash
claude mcp add ncgo -- /absolute/path/to/ncgo mcp serve
claude mcp get ncgo
```

如需项目共享配置，Claude Code 也支持 `.mcp.json`：

```json
{
  "mcpServers": {
    "ncgo": {
      "command": "/absolute/path/to/ncgo",
      "args": ["mcp", "serve"]
    }
  }
}
```

使用项目级 MCP server 前应先审查并批准配置。

### Cursor

在生成项目中创建 `.cursor/mcp.json`：

```json
{
  "mcpServers": {
    "ncgo": {
      "command": "/absolute/path/to/ncgo",
      "args": ["mcp", "serve"]
    }
  }
}
```

修改后刷新 Cursor 的 MCP server 列表或重启客户端。

## 4. 验证连接

让 Agent 列出 ncgo 工具并调用 `ncgo_version`。不经过 MCP 客户端时，可直接检查
stdio 协议：

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | ncgo mcp serve
```

第二条响应应包含 `ncgo_doctor`、`ncgo_check`、`ncgo_ai_context` 等工具。

## 5. 推荐 Agent 工作流

先建立事实，不要猜测：

1. 读取 `AGENTS.md` 以及 `.ncgo/manifest.yaml` 或 `ncgo.workspace`。
2. 调用 `ncgo_version`。
3. 运行 `ncgo doctor --root . --output json` 或调用 `ncgo_doctor`。
4. 运行 `ncgo check --root . --output json` 或调用 `ncgo_check`。
5. 调用 `ncgo_ai_context` 获取 domains、methods、anchors 和 issues。
6. 为任务选择范围最小的命令。
7. 写入前优先使用 `--plan` 或 `dryRun`。
8. 执行后消费返回的 `nextSteps`，不要自行猜测后续命令。
9. 运行聚焦的构建/测试和 `ncgo check`。
10. 执行 `ncgo ai sync --root . --target all`，然后再次检查。

决策时优先读取 MCP 顶层结构化字段，`content[0].text` 主要用于人类展示。

## 6. 安全与失败恢复

- 批准写工具前检查目标路径与副作用。
- 不要把 `--force` 当作第一种修复方式；先检查已有文件。
- 当 IDL 或模板才是事实来源时，不要直接修改 handler、router 或 `kitex_gen`。
- `doctor`、`check` 或 `protolint` 报告阻断错误时，先解决错误再做无关写入。
- 命令在调用 `hz`、`kitex`、`go mod tidy` 或 registry 后失败时，重试前先检查
  工作区状态。

## 7. 可复用提示词

```text
你正在操作 <root> 下的 ncgo 项目。先识别它是 ncgo 源码仓库、Mono 服务还是
Micro 工作区。读取 AGENTS.md 和 manifest/workspace 元数据，并通过 ncgo_version、
ncgo_doctor、ncgo_check、ncgo_ai_context 建立事实。写入前在可用时使用 plan/dry-run，
说明将修改的路径和副作用。优先消费结构化字段和 nextSteps。完成后运行聚焦测试、
ncgo check，并执行 ncgo ai sync --target all。工具失败时不要绕过错误继续猜测。
```

各命令契约和 payload 示例见
[examples.zh-CN.md](examples.zh-CN.md#0-mcp-contract-first-参考)。

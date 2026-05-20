# ncgo 架构设计

## 概述

ncgo 是一个 Go 微服务脚手架 CLI 工具，用于生成 Hertz（HTTP）和 Kitex（RPC）服务脚手架、渲染 AI 上下文文件（AGENTS.md、CLAUDE.md、Cursor rules），并通过 CLI 和 MCP（Model Context Protocol）stdio 服务器暴露操作接口。

## 系统架构

```
┌─────────────────────────────────────────────────────────┐
│                     入口                                 │
│                      main.go                            │
│                     cli.Main()                          │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                   CLI 层                                 │
│              internal/cli/ (Cobra)                       │
│                                                         │
│  命令: version, new, add, ai, i18n, protolint,          │
│         doctor, mcp, upgrade, extract, export, test      │
│                                                         │
│  契约面: 标志、JSON 输出、帮助文本、退出码                   │
└────────┬────────────┬──────────────┬────────────────────┘
         │            │              │
┌────────▼──┐  ┌──────▼──────┐ ┌─────▼───────────┐
│  MCP       │  │  脚手架      │ │  AI / i18n /    │
│  服务器    │  │  生成器      │ │  其他工具       │
│            │  │              │ │                  │
│ server.go  │  │ mono/        │ │ ai/ sync.go     │
│ tools.go   │  │ micro/       │ │ i18n/           │
│            │  │ bff/         │ │ doctor/         │
│ 12 个工具  │  │ rpc/         │ │ protolint/      │
│            │  │ domain/      │ │ upgrade/        │
│            │  │ infra/       │ │ extract/        │
│            │  │ method/      │ │ exec/           │
│            │  │ shared/      │ │ testutil/       │
└─────┬──────┘  └──────┬───────┘ └──────┬───────────┘
      │                │                 │
      │         ┌──────▼──────────┐      │
      │         │  模板            │      │
      │         │  嵌入式          │      │
      │         │                  │      │
      │         │  assets/_data/   │      │
      │         │    hertz/        │      │
      │         │    kitex/        │      │
      │         │    optional/     │      │
      │         │    docs/         │      │
      │         └──────────────────┘      │
      │                                   │
      │         ┌──────────────────┐      │
      │         │  清单 (Manifest)  │      │
      │         │  .ncgo/          │      │
      │         │  manifest.yaml   │      │
      │         │  单一事实来源     │      │
      │         └──────────────────┘      │
      │                                   │
      └───────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────────────────────┐
│                     生成输出                              │
│                                                          │
│  Hertz 服务 (HTTP)                                       │
│  Kitex 服务 (RPC)                                        │
│  AI 上下文文件 (AGENTS.md, CLAUDE.md, .cursorrules)      │
│  Docker、pre-commit、CI 配置                             │
└──────────────────────────────────────────────────────────┘
```

## 层级详情

### 1. CLI 层 (`internal/cli/`)

- **框架**: Cobra
- **根命令**: `ncgo`
- **入口**: `main.go → cli.Main()`
- **关键结构**:
  - `newRootCmd()` - 注册所有子命令
  - `newOptions` 结构体 - 共享标志：`--module`、`--mode`、`--kind`、`--db`、`--infra`、`--preset`、`--idl`、`--dir`、`--no-generate`、`--rule-center-addr`
  - `runNewMono()` / `runNewMicro()` - 编排脚手架生成

### 2. MCP 层 (`internal/mcp/`)

- **协议**: JSON-RPC 2.0，Content-Length 分帧
- **服务器**: `server.go` 处理 `initialize`、`tools/list`、`tools/call`
- **工具**（共 12 个，`tools.go`）:
  - `ncgo_version`、`ncgo_doctor`、`ncgo_new`、`ncgo_add_domain`
  - `ncgo_ai_init_claude`、`ncgo_ai_sync`
  - `ncgo_i18n_report`、`ncgo_i18n_check`
  - `ncgo_protolint`、`ncgo_add_infra`、`ncgo_add_method`、`ncgo_add_rule_center`
- **契约**: `content[0].text` 用于人类可读输出，顶层 JSON 字段用于 Agent 消费

### 3. 清单 (`internal/manifest/`)

- **结构**: `.ncgo/manifest.yaml`
- **结构体**: `Ncgo` (Meta) → `Mode`、`Module`、`Service`、`Infra`、`Domains`、`GeneratedAt`
- **验证**: mode（`mono`|`micro`）、module（合法 Go 路径）、service.name 必填、service.kind（`hertz`|`kitex`）
- **写入模式**: 原子写入（临时文件 + 重命名）

### 4. 脚手架生成器 (`internal/scaffold/`)

| 包 | 用途 |
|---|---|
| `mono/` | 单服务生成（Hertz/Kitex），含金色测试 |
| `micro/` | 多服务工作区生成 |
| `bff/` | BFF（Hertz）服务生成 |
| `rpc/` | RPC（Kitex）服务生成 |
| `domain/` | 领域用例/仓库生成 |
| `infra/` | 可选基础设施附加组件（Redis、Kafka、ES、可观测性、灰度、日志） |
| `method/` | 在 ncgo 锚点处插入方法存根 |
| `shared/` | 共享辅助（容器文件、Docker、pre-commit） |

### 5. 模板 (`internal/assets/_data/`)

- **嵌入方式**: `assets.go` 中的 `//go:embed all:_data`
- **分类**:
  - `hertz/` - HTTP 服务模板
  - `kitex/` - RPC 服务模板
  - `optional/` - 基础设施附加模板
  - `docs/` - 生成项目的设计文档
- **渲染**: Go `text/template`，FuncMap（`ToLower`、`ToUpper`、`LowerFirst`、`exportName`）
- **版本**: `VERSION` 文件，带 `ncgo_assets_version` 标记

### 6. AI 上下文 (`internal/ai/`)

- **管理标记**: `<!-- ncgo:managed -->` 用于文件所有权追踪
- **同步**: 从清单和设计文档渲染所有受管理的 AI 产物
- **来源解析**: 检测 `.ncgo/manifest.yaml` 或工作区，加载设计文档
- **写入策略**: 遵循 managed-marker / Force / DryRun 规则
- **结果**: 追踪 `Written`、`Skipped`、`Notes`、`NextSteps`、`Scope`、`SourceRef`

## 关键契约

### CLI/MCP/脚手架契约面
CLI 标志、JSON 输出、MCP schemas（`content[0].text`、顶层结构化字段）、脚手架模板和生成文件布局属于契约敏感区域。变更需要同时更新测试和文档。

### 模板交接顺序
Kitex 脚手架必须先运行 `make sqlc` 再运行 `go mod tidy`。Hertz 仅在 `WithDatabase=true` 时需要相同顺序。

### 生成文件
不要手动编辑下游生成的项目文件。应修复模板或生成器。

## 测试策略

| 层级 | 位置 | 模式 |
|------|------|------|
| 单元测试 | 代码旁的 `*_test.go` | 辅助函数、纯逻辑、schema 解析 |
| 集成测试 | `internal/cli/add_test.go`、`internal/mcp/server_test.go` | CLI 命令、MCP 工具、多包串联 |
| 金色测试 | `internal/scaffold/mono/golden_test.go` | 快照式，`testdata/`，使用 `-update-golden` 更新 |
| 冒烟测试 | `./scripts/smoke.sh` | 端到端 CLI 验证 |

## 前置要求

- Go 1.25+
- `hz >= v0.9.7`（Hertz 流程）
- `kitex >= v0.16.1`（Kitex 流程）
- `pre-commit`（git 钩子）

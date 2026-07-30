---
sidebar_position: 5
title: AI 协作
---

# AI 协作

`ncgo` 会生成 AI 上下文文件，并通过 MCP 暴露操作接口，
帮助编码 agent 理解生成的项目。

## 初始化 .claude 起始文件

```bash
ncgo ai init claude --preset minimal
```

## 同步生成上下文

```bash
ncgo ai sync
```

生成 `AGENTS.md`、`CLAUDE.md`、`.claude/generated/project-context.md`
以及 Cursor rules。

## 通过 MCP 暴露操作

```bash
ncgo mcp serve
```

启动一个 MCP stdio server，暴露 `ncgo_version`、`ncgo_doctor`、
`ncgo_ai_sync` 等工具。

## `ncgo ai init claude` flags

| Flag | 说明 |
| --- | --- |
| `--dry-run` | 报告计划执行的操作，但不实际写入文件 |
| `--force` | 覆盖已存在的起始文件 |
| `--output string` | 输出格式：`text` 或 `json`（默认 `"text"`） |
| `--preset string` | 起始预设：`minimal` \| `team`（默认 `"minimal"`） |
| `--root string` | 要初始化 `.claude/` 的仓库根目录（默认 `"."`） |
| `-h, --help` | `claude` 帮助信息 |

## `ncgo ai sync` flags

| Flag | 说明 |
| --- | --- |
| `--dry-run` | 报告计划执行的操作，但不实际写入文件 |
| `--force` | 覆盖缺少 `ncgo:managed` 标记的文件 |
| `--lang string` | 设计文档语言：`en` \| `zh-CN`（默认 `"en"`） |
| `--output string` | 输出格式：`text` 或 `json`（默认 `"text"`） |
| `--root string` | 包含 `.ncgo/manifest.yaml` 的服务根目录，或包含 `ncgo.workspace` 的微服务工作区根目录（默认 `"."`） |
| `-h, --help` | `sync` 帮助信息 |

## `ncgo mcp serve` flags

| Flag | 说明 |
| --- | --- |
| `-h, --help` | `serve` 帮助信息 |

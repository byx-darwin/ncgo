---
sidebar_position: 1
title: 新建服务
---

# 新建服务

`ncgo new` 用于生成一个 mono 模式的 Hertz（HTTP）或 Kitex（RPC）服务。

## Hertz（HTTP）

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz
```

## Kitex（RPC）

```bash
ncgo new user-svc --module github.com/acme/user-svc --kind kitex
```

## 跳过生成器

只写入 manifest、模板输入与 IDL 占位文件：

```bash
ncgo new user-api --module github.com/acme/user-api --no-generate
```

## Flags

| Flag | 说明 |
| --- | --- |
| `--db string` | Mono 数据库：`postgres` \| `none`（默认 `"none"`） |
| `--dir string` | 目标目录，默认 `./<name>` |
| `--idl string` | Mono IDL 路径，默认 `idl/app/<name>.proto`（hertz）或 `idl/<service>.proto`（kitex） |
| `--infra strings` | 创建时一并生成的 mono infra 附加组件（目前：`redis`） |
| `--kind string` | Mono 服务类型：`hertz` \| `kitex`（默认 `"hertz"`） |
| `--mode string` | 项目模式：`mono` \| `micro`（默认 `"mono"`） |
| `--module string` | Go module 路径，例如 `github.com/acme/user-api`（必填） |
| `--no-generate` | 仅 mono：跳过生成器调用，只写入 manifest + `template/` + IDL 占位文件 |
| `--preset string` | Mono 预设名称：`rule-center`（带限流的 Kitex） |
| `--rule-center-addr string` | Rule-center gRPC 地址，用于限流规则查询（例如 `localhost:8888`） |
| `-h, --help` | `new` 帮助信息 |

## 生成的内容

- `.ncgo/manifest.yaml` — 项目元数据（唯一事实来源）
- `idl/` — IDL 占位文件
- `template/` — 生成器使用的模板输入
- handler / service / repo 层（通过 `hz` / `kitex` 生成）
- AI 上下文文件（`AGENTS.md`、`CLAUDE.md`、`.claude/generated/`）

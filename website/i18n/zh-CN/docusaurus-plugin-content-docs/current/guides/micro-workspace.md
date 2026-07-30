---
sidebar_position: 2
title: 微服务工作区
---

# 微服务工作区

创建一个微服务工作区，包含一个根 `ncgo.workspace` 以及多个服务：

```bash
ncgo new shop --module github.com/acme/shop --mode micro
```

添加一个 Kitex RPC 服务：

```bash
ncgo add rpc user --root ./shop --module github.com/acme/shop/user
```

添加一个 Hertz BFF 服务：

```bash
ncgo add bff gateway --root ./shop --module github.com/acme/shop/gateway
```

## `ncgo add rpc` flags

| Flag | 说明 |
| --- | --- |
| `--dir string` | 服务目录，相对于 root；默认 `services/<name>` |
| `--dry-run` | 预览即将写入的 RPC 服务文件，不实际修改 |
| `--module string` | RPC 服务的 Go module 路径；默认 `<workspace.module>/<service dir>` |
| `--no-generate` | 跳过 `kitex` 调用，只写入服务 manifest + `template/` + IDL 占位文件 |
| `--output string` | 输出格式：`text` 或 `json`（默认 `"text"`） |
| `--plan` | `--dry-run --output json` 的快捷写法 |
| `--preset string` | 使用的预设模板（例如 `rule-center`） |
| `--root string` | 包含 `ncgo.workspace` 的微服务工作区根目录（默认 `"."`） |
| `-h, --help` | `rpc` 帮助信息 |

## `ncgo add bff` flags

| Flag | 说明 |
| --- | --- |
| `--dir string` | 服务目录，相对于 root；默认 `services/<name>` |
| `--dry-run` | 预览即将写入的 BFF 服务文件，不实际修改 |
| `--module string` | BFF 服务的 Go module 路径；默认 `<workspace.module>/<service dir>` |
| `--no-generate` | 跳过 `hz` 调用，只写入服务 manifest + `template/` + IDL 占位文件 |
| `--output string` | 输出格式：`text` 或 `json`（默认 `"text"`） |
| `--plan` | `--dry-run --output json` 的快捷写法 |
| `--root string` | 包含 `ncgo.workspace` 的微服务工作区根目录（默认 `"."`） |
| `-h, --help` | `bff` 帮助信息 |

---
sidebar_position: 3
title: 领域与方法
---

# 领域与方法

为某个领域生成 usecase / repository / DI 注册文件：

```bash
ncgo add domain order --root ./user-api
```

在 ncgo 锚点标记处插入方法 stub：

```bash
ncgo add method order.CreateOrder --root ./user-api --in usecase
```

## `ncgo add domain` flags

| Flag | 说明 |
| --- | --- |
| `--dry-run` | 预览即将写入的领域文件，不实际修改 |
| `--force` | 覆盖已存在的生成文件 |
| `--output string` | 输出格式：`text` 或 `json`（默认 `"text"`） |
| `--plan` | `--dry-run --output json` 的快捷写法 |
| `--root string` | 包含 `.ncgo/manifest.yaml` 的项目根目录（默认 `"."`） |
| `-h, --help` | `domain` 帮助信息 |

## `ncgo add method` flags

| Flag | 说明 |
| --- | --- |
| `--in string` | 目标层：`usecase`（默认 `"usecase"`） |
| `--root string` | 包含 `.ncgo/manifest.yaml` 的项目根目录（默认 `"."`） |
| `-h, --help` | `method` 帮助信息 |

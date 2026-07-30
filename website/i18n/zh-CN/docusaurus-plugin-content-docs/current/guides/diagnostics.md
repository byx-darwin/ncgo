---
sidebar_position: 6
title: 诊断与演进
---

# 诊断与演进

## 诊断

```bash
ncgo doctor
```

检查宿主工具、项目元数据以及默认 proto 的契约问题。加上
`--json`（或 `--output sarif`）可得到供 AI agent、CI 或代码扫描工具
消费的结构化输出。

## 升级元数据

```bash
ncgo upgrade --plan
```

仅涉及元数据的 MVP：预览 manifest / assets 元数据更新，
不会重写生成的源码文件。

## 提取领域（mono → micro）

```bash
ncgo extract domain <name>
ncgo extract domain <name> --json
ncgo extract domain <name> --apply
```

保守的 mono 到 micro 提取方式。不加 `--apply` 时仅校验领域，
并打印需要迁移的文件与 import。`--json` 输出机器可读的迁移计划；
`--apply` 将计划中的文件复制到目标 Kitex 服务，并重写领域本地的 import。

## `ncgo doctor` flags

| Flag | 说明 |
| --- | --- |
| `--json` | 输出机器可读的 JSON（`--output json` 的兼容别名） |
| `--output string` | 输出格式：`text`、`json` 或 `sarif`（默认 `"text"`） |
| `--root string` | 要检查的项目根目录；传 `''` 可跳过项目检查（默认 `"."`） |
| `-h, --help` | `doctor` 帮助信息 |

## `ncgo upgrade` flags

| Flag | 说明 |
| --- | --- |
| `--dry-run` | 报告计划执行的元数据更新，但不写入文件 |
| `--plan` | 打印详细的元数据升级计划，但不写入文件 |
| `--root string` | 项目或微服务工作区根目录（默认 `"."`） |
| `-h, --help` | `upgrade` 帮助信息 |

## `ncgo extract domain` flags

| Flag | 说明 |
| --- | --- |
| `--apply` | 将计划中的文件复制到已存在的目标 Kitex 服务 |
| `--json` | 输出机器可读的提取计划 |
| `--root string` | 包含 `.ncgo/manifest.yaml` 的 mono 项目根目录（默认 `"."`） |
| `--to string` | 目标服务目录，相对于 root；默认 `services/<name>-rpc` |
| `-h, --help` | `domain` 帮助信息 |

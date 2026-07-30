---
sidebar_position: 4
title: 基础设施
---

# 基础设施

`ncgo add infra` 用于向已有项目添加可选的 infra 辅助组件。

```bash
ncgo add infra redis --root ./user-api
```

支持的 kind 包括：`redis`、`kafka`、`es`、`clickhouse`、
`observability_logging`、`logging`、`release_canary`、`canary` 以及
`registry_polaris`。

## Flags

| Flag | 说明 |
| --- | --- |
| `--dry-run` | 预览即将写入的附加组件与 `--wire` 变更，不实际修改文件 |
| `--force` | 覆盖已存在的生成附加组件文件 |
| `--output string` | 输出格式：`text` 或 `json`（默认 `"text"`） |
| `--plan` | `--dry-run --output json` 的快捷写法 |
| `--root string` | 包含 `.ncgo/manifest.yaml` 的项目根目录（默认 `"."`） |
| `--wire` | 可选：在支持时更新生成的 server/client 装配代码 |
| `-h, --help` | `infra` 帮助信息 |

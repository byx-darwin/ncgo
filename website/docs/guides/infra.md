---
sidebar_position: 4
title: Infrastructure
---

# Infrastructure

`ncgo add infra` adds optional infra helpers to an existing project.

```bash
ncgo add infra redis --root ./user-api
```

Supported kinds include: `redis`, `kafka`, `es`, `clickhouse`,
`observability_logging`, `logging`, `release_canary`, `canary`, and
`registry_polaris`.

## Flags

| Flag | Description |
| --- | --- |
| `--dry-run` | Preview intended add-on writes and `--wire` changes without modifying files |
| `--force` | Overwrite existing generated add-on file |
| `--output string` | Output format: `text` or `json` (default `"text"`) |
| `--plan` | Shorthand for `--dry-run --output json` |
| `--root string` | Project root containing `.ncgo/manifest.yaml` (default `"."`) |
| `--wire` | Opt-in: update generated server/client wiring when supported |
| `-h, --help` | help for `infra` |

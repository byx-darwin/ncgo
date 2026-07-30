---
sidebar_position: 3
title: Domain & Method
---

# Domain & Method

Generate usecase / repository / DI register files for a domain:

```bash
ncgo add domain order --root ./user-api
```

Insert a method stub at the ncgo anchor markers:

```bash
ncgo add method order.CreateOrder --root ./user-api --in usecase
```

## `ncgo add domain` flags

| Flag | Description |
| --- | --- |
| `--dry-run` | Preview intended domain writes without modifying files |
| `--force` | Overwrite existing generated files |
| `--output string` | Output format: `text` or `json` (default `"text"`) |
| `--plan` | Shorthand for `--dry-run --output json` |
| `--root string` | Project root containing `.ncgo/manifest.yaml` (default `"."`) |
| `-h, --help` | help for `domain` |

## `ncgo add method` flags

| Flag | Description |
| --- | --- |
| `--in string` | Target layer: `usecase` (default `"usecase"`) |
| `--root string` | Project root containing `.ncgo/manifest.yaml` (default `"."`) |
| `-h, --help` | help for `method` |

---
sidebar_position: 6
title: Diagnostics & Evolution
---

# Diagnostics & Evolution

## Diagnose

```bash
ncgo doctor
```

Checks host tools, project metadata, and default proto contract issues. Add
`--json` (or `--output sarif`) for machine-readable output consumed by AI
agents, CI, or code scanning tools.

## Upgrade metadata

```bash
ncgo upgrade --plan
```

Metadata-only MVP: previews manifest / assets metadata updates without
rewriting generated source files.

## Extract domain (mono → micro)

```bash
ncgo extract domain --plan
ncgo extract domain --apply
```

Conservative mono-to-micro extraction. `--plan` previews; `--apply` performs.

## `ncgo doctor` flags

| Flag | Description |
| --- | --- |
| `--json` | Emit machine-readable JSON (compatibility alias for `--output json`) |
| `--output string` | Output format: `text`, `json`, or `sarif` (default `"text"`) |
| `--root string` | Project root to inspect; pass `''` to skip project checks (default `"."`) |
| `-h, --help` | help for `doctor` |

## `ncgo upgrade` flags

| Flag | Description |
| --- | --- |
| `--dry-run` | Report planned metadata updates without writing files |
| `--plan` | Print a detailed metadata upgrade plan without writing files |
| `--root string` | Project or micro workspace root (default `"."`) |
| `-h, --help` | help for `upgrade` |

## `ncgo extract domain` flags

| Flag | Description |
| --- | --- |
| `--apply` | Copy planned files into the existing target Kitex service |
| `--json` | Emit machine-readable extraction plan |
| `--root string` | Mono project root containing `.ncgo/manifest.yaml` (default `"."`) |
| `--to string` | Target service directory relative to root; default `services/<name>-rpc` |
| `-h, --help` | help for `domain` |

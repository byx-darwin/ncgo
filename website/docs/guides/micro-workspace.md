---
sidebar_position: 2
title: Micro Workspace
---

# Micro Workspace

Create a micro workspace with a root `ncgo.workspace` plus services:

```bash
ncgo new shop --module github.com/acme/shop --mode micro
```

Add a Kitex RPC service:

```bash
ncgo add rpc user --root ./shop --module github.com/acme/shop/user
```

Add a Hertz BFF service:

```bash
ncgo add bff gateway --root ./shop --module github.com/acme/shop/gateway
```

## `ncgo add rpc` flags

| Flag | Description |
| --- | --- |
| `--dir string` | Service directory relative to root; default `services/<name>` |
| `--dry-run` | Preview intended RPC service writes without modifying files |
| `--module string` | Go module path for the RPC service; default `<workspace.module>/<service dir>` |
| `--no-generate` | Skip `kitex` invocation; only write service manifest + `template/` + idl placeholder |
| `--output string` | Output format: `text` or `json` (default `"text"`) |
| `--plan` | Shorthand for `--dry-run --output json` |
| `--preset string` | Preset template to use (e.g., `rule-center`) |
| `--root string` | Micro workspace root containing `ncgo.workspace` (default `"."`) |
| `-h, --help` | help for `rpc` |

## `ncgo add bff` flags

| Flag | Description |
| --- | --- |
| `--dir string` | Service directory relative to root; default `services/<name>` |
| `--dry-run` | Preview intended BFF service writes without modifying files |
| `--module string` | Go module path for the BFF service; default `<workspace.module>/<service dir>` |
| `--no-generate` | Skip `hz` invocation; only write service manifest + `template/` + idl placeholder |
| `--output string` | Output format: `text` or `json` (default `"text"`) |
| `--plan` | Shorthand for `--dry-run --output json` |
| `--root string` | Micro workspace root containing `ncgo.workspace` (default `"."`) |
| `-h, --help` | help for `bff` |

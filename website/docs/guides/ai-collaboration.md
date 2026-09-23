---
sidebar_position: 5
title: AI Collaboration
---

# AI Collaboration

`ncgo` renders AI context files and exposes operations over MCP so coding
agents understand generated projects.

## Bootstrap .claude starter files

```bash
ncgo ai init claude --preset minimal
```

## Sync generated context

```bash
ncgo ai sync
```

All three consumer groups are enabled for generated projects. The default
renders `AGENTS.md`, `CLAUDE.md`, the Claude skill and generated project
context, and Cursor rules. Use `--target agents|claude|cursor` only to narrow a
single refresh. `ncgo check --root .` audits every enabled file; missing or
user-owned paths are structured warnings and stale managed files fail the check.

## Expose operations over MCP

```bash
ncgo mcp serve
```

Starts an MCP stdio server exposing tools such as `ncgo_version`,
`ncgo_doctor`, and `ncgo_ai_sync`.

## `ncgo ai init claude` flags

| Flag | Description |
| --- | --- |
| `--dry-run` | Report intended actions without writing files |
| `--force` | Overwrite existing starter files |
| `--output string` | Output format: `text` or `json` (default `"text"`) |
| `--preset string` | Starter preset: `minimal` \| `team` (default `"minimal"`) |
| `--root string` | Repository root where `.claude/` should be bootstrapped (default `"."`) |
| `-h, --help` | help for `claude` |

## `ncgo ai sync` flags

| Flag | Description |
| --- | --- |
| `--dry-run` | Report intended actions without writing files |
| `--force` | Overwrite files that lack the `ncgo:managed` marker |
| `--lang string` | Design-doc language: `en` \| `zh-CN` (default `"en"`) |
| `--output string` | Output format: `text` or `json` (default `"text"`) |
| `--root string` | Service root with `.ncgo/manifest.yaml` or micro workspace root with `ncgo.workspace` (default `"."`) |
| `--target string` | Target group: `all` \| `agents` \| `claude` \| `cursor` (default `all`) |
| `-h, --help` | help for `sync` |

## `ncgo mcp serve` flags

| Flag | Description |
| --- | --- |
| `-h, --help` | help for `serve` |

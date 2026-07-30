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

Renders `AGENTS.md`, `CLAUDE.md`, `.claude/generated/project-context.md`, and
Cursor rules.

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
| `-h, --help` | help for `sync` |

## `ncgo mcp serve` flags

| Flag | Description |
| --- | --- |
| `-h, --help` | help for `serve` |

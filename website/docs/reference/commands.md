---
sidebar_position: 1
title: Commands
---

# Command Reference

Top-level commands (from `ncgo --help`):

| Command | Purpose |
| --- | --- |
| `ncgo new` | Scaffold a new service or workspace |
| `ncgo add` | Add a feature (`infra` / `domain` / `rpc` / `bff`) to an existing project |
| `ncgo import` | Import an existing hz/kitex project into ncgo |
| `ncgo ai` | AI collaboration helpers (sync context, bootstrap `.claude`) |
| `ncgo doctor` | Diagnose host tools and the current project |
| `ncgo protolint` | Lint `.proto` files with Proto I/O rules |
| `ncgo i18n` | Inspect project i18n artifacts with structured output |
| `ncgo extract` | Plan or perform mono-to-micro extractions |
| `ncgo export` | Export code templates from an existing project |
| `ncgo upgrade` | Upgrade project metadata |
| `ncgo mcp` | MCP server for AI agents |
| `ncgo test` | Run generated tests for a project |
| `ncgo completion` | Generate shell completion script |
| `ncgo version` | Print ncgo, build, and assets versions |

For per-command flags, run `ncgo <command> --help`. See the
[guides](../guides/new-service.md) for worked examples.

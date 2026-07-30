---
sidebar_position: 1
slug: /intro
title: Introduction
---

# ncgo

`ncgo` is an AI-friendly scaffold CLI for Go microservices. It generates
Hertz (HTTP) and Kitex (RPC) service scaffolds, renders AI context files
(`AGENTS.md`, `CLAUDE.md`, Cursor rules), and exposes operations via both CLI
and an MCP stdio server.

## Why ncgo

- **Deterministic scaffolding** — manifests, IDL placeholders, and templates stay under version control.
- **Generator-aware** — orchestrates `hz` / `kitex` while supporting `--no-generate`.
- **Agent-friendly by default** — renders AI context files and exposes MCP tools.
- **Lifecycle helpers** — `doctor`, `upgrade`, and conservative `extract domain`.

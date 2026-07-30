---
sidebar_position: 1
title: New Service
---

# New Service

`ncgo new` scaffolds a mono Hertz (HTTP) or Kitex (RPC) service.

## Hertz (HTTP)

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz
```

## Kitex (RPC)

```bash
ncgo new user-svc --module github.com/acme/user-svc --kind kitex
```

## Skip the generator

Write only the manifest, template inputs and IDL placeholder:

```bash
ncgo new user-api --module github.com/acme/user-api --no-generate
```

## Flags

| Flag | Description |
| --- | --- |
| `--db string` | Mono database: `postgres` \| `none` (default `"none"`) |
| `--dir string` | Target directory, default `./<name>` |
| `--idl string` | Mono IDL path, default `idl/app/<name>.proto` (hertz) or `idl/<service>.proto` (kitex) |
| `--infra strings` | Mono infra add-ons to scaffold at creation time (currently: `redis`) |
| `--kind string` | Mono service kind: `hertz` \| `kitex` (default `"hertz"`) |
| `--mode string` | Project mode: `mono` \| `micro` (default `"mono"`) |
| `--module string` | Go module path, e.g. `github.com/acme/user-api` (required) |
| `--no-generate` | Mono only: skip the generator invocation; only write manifest + `template/` + idl placeholder |
| `--preset string` | Mono preset name: `rule-center` (Kitex with rate-limiting) |
| `--rule-center-addr string` | Rule-center gRPC address for rate-limit rule queries (e.g., `localhost:8888`) |
| `-h, --help` | help for `new` |

## What gets generated

- `.ncgo/manifest.yaml` — project metadata (single source of truth)
- `idl/` — IDL placeholder
- `template/` — template inputs for the generator
- handler / service / repo layers (via `hz` / `kitex`)
- AI context files (`AGENTS.md`, `CLAUDE.md`, `.claude/generated/`)

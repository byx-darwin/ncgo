---
sidebar_position: 2
title: 30-Second Tour
---

# 30-Second Tour

Assuming `hz` is already on `PATH`, the shortest happy path is:

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo new user-api --module github.com/acme/user-api
cd user-api
go mod tidy
make dev
```

`ncgo new` writes a manifest (`.ncgo/manifest.yaml`), an IDL placeholder,
template inputs, then invokes `hz` to generate the handler / service / repo
layers. AI context files (`AGENTS.md`, `CLAUDE.md`) are rendered alongside.

For Kitex, micro workspaces, and generator-free flows, see the
[guides](../guides/new-service.md).

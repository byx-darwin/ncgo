---
sidebar_position: 2
title: 30 秒上手
---

# 30 秒上手

假设 `hz` 已在 `PATH` 中，最短的完整路径如下：

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo new user-api --module github.com/acme/user-api
cd user-api
go mod tidy
make dev
```

`ncgo new` 会写入 manifest（`.ncgo/manifest.yaml`）、IDL 占位文件、
模板输入，然后调用 `hz` 生成 handler / service / repo 层。
同时还会生成 AI 上下文文件（`AGENTS.md`、`CLAUDE.md`）。

关于 Kitex、微服务工作区以及不走生成器的流程，请参见
[指南](../guides/new-service.md)。

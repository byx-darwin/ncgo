---
sidebar_position: 1
slug: /intro
title: 简介
---

# ncgo

`ncgo` 是面向 Go 微服务的 AI 友好脚手架 CLI。它生成 Hertz（HTTP）与 Kitex（RPC）服务骨架，
渲染 AI 上下文文件（`AGENTS.md`、`CLAUDE.md`、Cursor 规则），并通过 CLI 与 MCP stdio 服务器暴露操作。

## 为什么选择 ncgo

- **确定性脚手架** —— manifest、IDL 占位符与模板全部纳入版本控制。
- **感知生成器** —— 编排 `hz` / `kitex`，同时支持 `--no-generate`。
- **默认对 Agent 友好** —— 渲染 AI 上下文文件并暴露 MCP 工具。
- **生命周期助手** —— 内置 `doctor`、`upgrade` 与保守的 `extract domain`。

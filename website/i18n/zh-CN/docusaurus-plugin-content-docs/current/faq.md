---
sidebar_position: 5
title: 常见问题
---

# 常见问题

### 必须安装 `hz` / `kitex` 吗？

对于生成流程，是的 —— Hertz 需要 `hz >= v0.9.7`，Kitex 需要
`kitex >= v0.16.1`。使用 `--no-generate` 可以跳过生成器，只写入
manifest、模板输入与 IDL 占位文件。运行 `ncgo doctor` 可以检查你的工具链。

### ncgo 是一个框架吗？

不是。`ncgo` 是一个脚手架与生命周期 CLI。它编排 `hz` / `kitex` 生成器，
并将 manifest、模板与 AI 上下文文件纳入版本控制。

### 可以在已有项目上使用吗？

可以。`ncgo import` 会为已有的 Hertz 或 Kitex 项目生成
`.ncgo/manifest.yaml`，以便后续的 `ncgo` 命令正常工作。

### 它能配合 AI 编码 agent 使用吗？

这正是它的核心用途。`ncgo` 会生成 `AGENTS.md`、`CLAUDE.md`、Cursor rules，
并提供一个 MCP stdio server，让 agent 能够操作生成的项目。

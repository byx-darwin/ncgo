---
sidebar_position: 1
title: 安装
---

# 安装

## 前置条件

- Go 1.25+
- `hz >= v0.9.7`（用于 Hertz 流程）
- `kitex >= v0.16.1`（用于 Kitex 流程）

## 安装 ncgo

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo version
```

如果安装后找不到 `ncgo`，请确认 `GOBIN`
（或 `$(go env GOPATH)/bin`）已加入 `PATH`。

也可以从本地仓库安装：

```bash
go install .
ncgo version
```

## 验证

运行内置诊断，确认宿主工具已就绪：

```bash
ncgo doctor
```

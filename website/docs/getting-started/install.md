---
sidebar_position: 1
title: Install
---

# Install

## Prerequisites

- Go 1.25+
- `hz >= v0.9.7` (for Hertz flows)
- `kitex >= v0.16.1` (for Kitex flows)

## Install ncgo

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo version
```

If `ncgo` is not found after installation, make sure your `GOBIN`
(or `$(go env GOPATH)/bin`) is on `PATH`.

From a local checkout, the repository root is also installable:

```bash
go install .
ncgo version
```

## Verify

Run the built-in diagnostics to confirm host tools are ready:

```bash
ncgo doctor
```

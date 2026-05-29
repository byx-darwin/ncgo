# ncgo 发布说明模板

适用于 tag 发布前后，对 GitHub 自动生成的 release notes 做人工润色。

建议结构：

## 一句话概述

- 这一版最重要的变化是什么
- 建议控制在 1~3 句内

## Highlights

- 新增能力：
- 重要修复：
- 文档/发布改进：

## 安装 / 升级

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo version
```

如需强调版本号，可写：

- 升级到 `vX.Y.Z`
- 重新执行 `ncgo version` 确认版本

## 兼容性说明

- 是否有 breaking changes
- 是否需要手动升级生成器（如 `hz` / `kitex`）
- 是否只影响新生成项目，不影响已有项目

## 验证方式

- `go test ./... -count=1`
- `./scripts/smoke.sh`
- `go install .`

## 自动生成变更列表

将 GitHub 根据 `.github/release.yml` 生成的分类变更列表保留在说明底部，作为详细 diff。
# 贡献指南

感谢你为 `ncgo` 做贡献。

English version: [CONTRIBUTING.md](CONTRIBUTING.md)

本文档是仓库协作者的统一入口，集中说明：

- 本地开发前提
- 本地检查命令
- PR / Issue 提交流程
- release labels 与发布资料入口

## 开发前提

- Go `1.25+`
- 如果会涉及 Hertz 生成流程：`hz >= v0.9.7`
- 如果会涉及 Kitex 生成流程：`kitex >= v0.16.1`

如果你当前只需要检查脚手架输入文件，优先使用 `--no-generate` 工作流。

## 本地检查

在创建或更新 PR 前，建议至少完成以下本地检查：

```bash
go build ./...
go build .
go vet ./...
go test ./... -count=1
./scripts/smoke.sh
```

日常迭代时可以先跑更小范围的检查，但 PR 最终状态应能通过上面的完整检查。

## 改动约定

- 尽量保持改动最小，并遵循当前项目结构
- 如果代码行为变化，记得补测试
- 如果命令、安装方式、发布流程或生成结果发生变化，记得同步更新文档 / 示例
- 如果只是文档改动，请在 PR 中明确说明

## Pull Request

- 使用 `.github/PULL_REQUEST_TEMPLATE.md`
- 说明用户可见影响
- 记录验证方式
- 如果有 breaking change，要明确写出影响和迁移方式
- 为 PR 选择合适的 release label

## Issue

- 使用 GitHub issue forms 提交 bug、feature request 或 docs improvement
- bug issue 尽量提供最小可复现步骤
- 文档问题尽量指出对应文件或章节

## 发布标签

release notes 由 PR 标签驱动自动生成。

主要参考文档：

- `docs/release-labels.zh-CN.md`

常见标签：

- `feature` / `enhancement`
- `fix` / `bug`
- `docs`
- `chore`、`ci`、`refactor`、`test`
- 如有兼容性破坏：`breaking-change` 或 `semver-major`
- 不希望进入 release notes：`skip-release-notes`

## 发布资料

- `docs/release.zh-CN.md` — 发布流程与人工发布步骤
- `docs/release-notes-template.zh-CN.md` — 发布说明人工润色模板
- `.github/release.yml` — GitHub 自动生成 release notes 的分类规则

## 相关文档

- `README.md`
- `README.zh-CN.md`
- `docs/examples.md`
- `docs/examples.zh-CN.md`
- `docs/context-handoff.zh-CN.md`

如果你暂时不确定改动应该落在哪，建议尽早创建 Draft PR，并说明：

- 预期用户影响
- 计划采用的验证方式
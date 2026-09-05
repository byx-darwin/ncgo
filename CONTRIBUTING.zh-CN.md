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
- 如果需要在本地运行数据库相关生成流程测试（`TestGenerateHertzWithDatabaseCompiles`、`TestGenerateKitexCompiles`）：`sqlc`，安装方式为 `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- 任何涉及 IDL 代码生成或校验的流程都需要 `protoc`（Protocol Buffers 编译器）

如果你当前只需要检查脚手架输入文件，优先使用 `--no-generate` 工作流。

CI 现在会安装锁定版本的 `hz`/`kitex`/`protoc`/`sqlc`，并在每次 push 和 PR 时真实运行编译验证测试（`TestGenerateHertzCompiles`、`TestGenerateHertzWithDatabaseCompiles`、`TestGenerateKitexCompiles`），不再被静默跳过。具体版本见 `.github/workflows/ci.yml` 中的 `Install code-gen tools` 步骤。

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

## 可选的 pre-commit 工作流

仓库现在提供了 `.pre-commit-config.yaml`，方便使用
[`pre-commit`](https://pre-commit.com/) 的协作者把部分检查前移到本地。

推荐用法：

1. 用你偏好的包管理方式安装 `pre-commit`
2. 执行 `pre-commit install --install-hooks --hook-type pre-commit --hook-type pre-push`
3. 初次接入或修改 hook 配置后，执行一次 `pre-commit run --all-files`

hook 分层如下：

- `pre-commit`：文件卫生检查，以及对暂存 Go 文件执行 `gofmt`
- `pre-push`：执行 `go vet ./...`、`go test ./... -count=1`、`go build .` 和 `./scripts/smoke.sh`

`pre-push` 故意对齐仓库 CI 的核心检查，因此耗时会明显高于 `pre-commit` 阶段。

## 改动约定

- 尽量保持改动最小，并遵循当前项目结构
- 如果代码行为变化，记得补测试
- 如果命令、安装方式、发布流程或生成结果发生变化，记得同步更新文档 / 示例
- 如果只是文档改动，请在 PR 中明确说明

## 模板 handoff 顺序

如果你修改 `internal/scaffold/mono` 或内置的 Hertz / Kitex 模板资产，
要保证生成项目交接给用户的步骤顺序与真实代码生成依赖保持一致：

- `nextSteps()` / `postGenerateNextSteps()` 应该反映 fresh scaffold 中可直接
  执行的安全顺序。
- Kitex 脚手架即使是默认 starter 场景，也必须先执行 `make sqlc` 再执行
  第一次 `go mod tidy`，因为生成的 `internal/base/data` / repository 接线会
  import `internal/db/gen`。
- Hertz 脚手架只有在 `WithDatabase=true` 时，才需要同样的
  `make sqlc` → `go mod tidy` 顺序。
- 如果你调整了这条顺序，记得在同一个 PR 里同步更新对应 mono 测试，以及
  受影响的文档 / 示例。

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

- `specs/008-release-labels.zh-CN.md`

常见标签：

- `feature` / `enhancement`
- `fix` / `bug`
- `docs`
- `chore`、`ci`、`refactor`、`test`
- 如有兼容性破坏：`breaking-change` 或 `semver-major`
- 不希望进入 release notes：`skip-release-notes`

## 发布资料

- `specs/008-release-process.zh-CN.md` — 发布流程与人工发布步骤
- `specs/008-release-notes-template.zh-CN.md` — 发布说明人工润色模板
- `.github/release.yml` — GitHub 自动生成 release notes 的分类规则

## 相关文档

- `README.md`
- `README.zh-CN.md`
- `docs/examples.md`
- `docs/examples.zh-CN.md`
- `specs/005-context-handoff.zh-CN.md`
- `specs/README.md` — 完整规格文档索引

如果你暂时不确定改动应该落在哪，建议尽早创建 Draft PR，并说明：

- 预期用户影响
- 计划采用的验证方式

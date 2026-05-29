# ncgo CI / Release 工程化

## CI

`.github/workflows/ci.yml` 会在 `push` 与 `pull_request` 时运行：

- `gofmt` 检查；
- `go vet ./...`；
- `go test ./... -count=1`；
- `go build .`；
- `./scripts/smoke.sh`。

本地提交前建议先完成 `CONTRIBUTING.zh-CN.md` 中的本地检查。

## Release workflow

`.github/workflows/release.yml` 支持两种入口：

- `workflow_dispatch`：只构建 snapshot artifacts，不创建 GitHub Release；
- 推送 `v*.*.*` tag：构建正式 artifacts、生成 `checksums.txt`，并创建 GitHub Release。

正式发布会构建以下平台：

- `linux/amd64`；
- `linux/arm64`；
- `darwin/amd64`；
- `darwin/arm64`；
- `windows/amd64`。

Release 二进制会通过 `-ldflags` 写入 `ncgo version` 使用的版本信息：

- `internal/cli.Version`：tag 版本号，非 tag 构建时为 `snapshot-<sha7>`；
- `internal/cli.BuildVersion`：当前 commit 短 SHA；
- `internal/cli.BuildTime`：UTC 编译时间，格式为 RFC3339。

本地直接执行 `go install .` 时不会自动注入 `-ldflags`；此时 `ncgo version` 会优先从 Go build info 中读取 `vcs.revision` / `vcs.time` 作为 fallback，并在工作区有未提交变更时给 build version 追加 `-dirty`。

GitHub Release 会使用 `gh release create --generate-notes` 自动生成发布说明；分类规则由 `.github/release.yml` 控制。

如果需要在正式发布前做人工润色，可参考 `docs/release-notes-template.zh-CN.md`。

如果需要统一 PR 标签与 release notes 分类，可参考 `docs/release-labels.zh-CN.md`。

发布完成后，用户应通过根模块安装：`go install github.com/byx-darwin/ncgo@latest`。

## 人工发布步骤

部署、发布、push、tag 均需人工确认后执行。建议流程：

1. 确认工作区干净，变更已 review。
2. 按 `CONTRIBUTING.zh-CN.md` 完成本地检查。
3. 确定语义化版本号，例如 `v0.1.0`。
4. 经人工确认后创建 annotated tag。
5. 经人工确认后 push tag，触发 Release workflow。
6. 在 GitHub Release 页面核对 artifacts、`checksums.txt` 与自动生成的 release notes；必要时按模板补充摘要、安装方式和兼容性说明。

示例命令：

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

如果 tag 或 release 需要撤回，应先在团队内确认影响范围，再手动删除 GitHub Release 与远端 tag；不要由自动化流程隐式回滚。

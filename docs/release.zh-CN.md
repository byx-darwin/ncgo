# ncgo CI / Release 工程化

## CI

`.github/workflows/ci.yml` 会在 `push` 与 `pull_request` 时运行：

- `gofmt` 检查；
- `go vet ./...`；
- `go test ./... -count=1`；
- `go build ./cmd/ncgo`；
- `./scripts/smoke.sh`。

本地提交前建议执行同一组检查：

```bash
go build ./...
go vet ./...
go test ./... -count=1
./scripts/smoke.sh
```

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

Release 二进制会通过 `-ldflags "-X main.version=<tag>"` 写入 `ncgo version` 使用的版本号。

## 人工发布步骤

部署、发布、push、tag 均需人工确认后执行。建议流程：

1. 确认工作区干净，变更已 review。
2. 本地执行 CI 等价检查。
3. 确定语义化版本号，例如 `v0.1.0`。
4. 经人工确认后创建 annotated tag。
5. 经人工确认后 push tag，触发 Release workflow。
6. 在 GitHub Release 页面核对 artifacts 与 `checksums.txt`。

示例命令：

```bash
go build ./...
go vet ./...
go test ./... -count=1
./scripts/smoke.sh

git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

如果 tag 或 release 需要撤回，应先在团队内确认影响范围，再手动删除 GitHub Release 与远端 tag；不要由自动化流程隐式回滚。
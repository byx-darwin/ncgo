# ncgo CI / Release 工程化

English version: [008-release-process.md](008-release-process.md)

## 验证分层

`.github/workflows/ci.yml` 同时承担日常 CI 与可复用 Release gate。它把确定性的
仓库检查与需要解析生成项目依赖的联网测试明确分开：

- **Hermetic unit gate**：先单独执行一次 `go mod download`，随后
  `scripts/test-unit.sh` 会关闭模块代理和自动工具链下载，并检查生成式参考文档
  漂移、格式、vet、short unit tests、全包构建与 CLI smoke。
- **生成项目集成验证**：固定使用 Go `1.26.5`、`protoc 28.3`、
  `hz v0.9.7`、`kitex v0.16.1` 与 `sqlc v1.30.0`。它会真实生成、tidy、
  构建并 race-test Hertz/Kitex 项目（含数据库组合），也会验证固定依赖的
  Polaris SDK fixture。验证会先预取受审的 Hertz/Kitex dependency lock fixture，
  随后让生成项目在离线状态下 tidy，并使用 `-mod=readonly` 构建和测试。模块缓存
  key 同时包含这些 lockfile、兼容性常量与模板。
- **Race 与 coverage**：每周一运行，并在 Release 调用可复用 workflow 时强制运行；
  使用 hermetic/short 测试面。

联网生成测试必须显式设置 `NCGO_INTEGRATION=1`；否则跳过。进入 integration
模式后，缺少任何生成器都会直接失败，不再静默 skip。

## 必须通过的 Release gate

`.github/workflows/release.yml` 会以 `release_gate: true` 调用 CI。以下检查全部
成功前，不会开始任何制品构建：

1. 生成式参考文档与格式检查；
2. `go vet`、hermetic unit tests 与全包构建；
3. CLI smoke tests；
4. 真实生成项目与固定 SDK 验证；
5. race 与 coverage 检查。

`workflow_dispatch` 走同一 gate，只生成 snapshot workflow artifacts；推送
`v*.*.*` tag 才会额外发布 GitHub Release。

## 可重复制品

Release workflow 只使用 `scripts/build-release.sh` 与
`scripts/package-release.py` 这一套发布实现。仓库已移除未被 workflow 使用的
GoReleaser 配置，避免两条发布路径继续漂移。

每个平台都会构建、打包两次，并在上传前要求 archive 逐字节一致。构建使用
`-trimpath`；`BuildTime` 与 archive 时间戳来自源 commit
（`SOURCE_DATE_EPOCH`）；archive 顺序、owner/group 与 gzip/zip metadata 都会归一化。

发布目标包括：

- `linux/amd64`、`linux/arm64`（`tar.gz`）
- `darwin/amd64`、`darwin/arm64`（`tar.gz`）
- `windows/amd64`（`zip`）

metadata job 强制要求恰好五个 archive，生成并立即校验排序后的
`checksums.txt`，同时发布含仓库、完整 revision、ref、workflow run 与制品摘要的
`provenance.json`。GitHub `actions/attest@v4` 还会在发布前为 archive 创建签名
artifact attestations。Required release path 中的每个 Action 都固定到受审的完整
commit SHA，release job 使用明确的 `ubuntu-24.04` runner label。

## 验证下载的制品

从同一个 Release 下载 archive、`checksums.txt` 与 `provenance.json`。Linux：

```bash
asset=ncgo_vX.Y.Z_linux_amd64.tar.gz
test "$(awk -v a="$asset" '$2 == a {n++} END {print n+0}' checksums.txt)" -eq 1 && \
  sha256sum -c <(awk -v a="$asset" '$2 == a' checksums.txt) && \
  gh attestation verify "$asset" -R byx-darwin/ncgo
```

macOS 可直接验证选中的摘要：

```bash
asset=ncgo_vX.Y.Z_darwin_arm64.tar.gz
test "$(awk -v a="$asset" '$2 == a {n++} END {print n+0}' checksums.txt)" -eq 1 && \
  shasum -a 256 -c <(awk -v a="$asset" '$2 == a' checksums.txt) && \
  gh attestation verify "$asset" -R byx-darwin/ncgo
```

还应确认 `provenance.json.source.revision` 是目标 tag commit，且其中 artifact digest
与 `checksums.txt` 一致。

## 人工发布步骤

发布、tag 与 push 都必须先获得人工确认。

1. 确认工作区干净，目标 commit 已在 `main`。
2. 执行 `go mod download && ./scripts/test-unit.sh`。修改生成器或模板时，还要安装
   上述精确版本并执行 `./scripts/test-generated.sh`。
3. 复核 [发布说明模板](008-release-notes-template.zh-CN.md) 与
   [release label 约定](008-release-labels.zh-CN.md)。
4. 创建永不复用的 annotated tag：`git tag -a vX.Y.Z -m "vX.Y.Z"`。
5. 经确认后推送：`git push origin vX.Y.Z`。
6. 对外宣布前，核对 Release gate、五个制品、checksums、provenance manifest 与
   attestations。
7. 验证 `go install github.com/byx-darwin/ncgo@vX.Y.Z` 与 `ncgo version`。

## 故障恢复

- **Gate 或构建失败**：不会发布任何内容。在新 commit 上修复，并创建新的 patch
  tag。不要移动或复用失败 tag；Go module proxy 可能已经观察到该版本。
- **发布前的瞬时失败**：每次 run 都通过双重构建证明 run 内可复现。复用不完整的
  不可变 tag/SHA run 前，应将新 digest 与上一次比较。托管 runner 或压缩运行时维护
  仍可能造成跨 run 字节变化；如有任何 digest 变化，不要替换 asset，应先审查原因并
  发布新的 patch。
- **GitHub Release 不完整**：停止对外公告，记录 workflow URL；经人工确认后只删除
  不完整的 GitHub Release，再以同一不可变 tag 重跑。禁止静默覆盖单个 asset。
- **已发布版本有缺陷**：标记受影响版本，发布修复后的 patch 版本，并说明升级/
  补救步骤。删除 Release 或 tag 无法召回 Go proxy 已缓存的版本，因此绝不能把旧
  版本号重新指向新 commit。
- **Checksum 或 attestation 不匹配**：把制品视为不可信，不要安装；保留证据与 run
  URL，查明原因后只发布新的 patch 版本。

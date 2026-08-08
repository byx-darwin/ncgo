# Issue #50 — ncgo check 主动触发自动化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** 让 `ncgo check` 通过两层自动闸门主动触发：层1 smoke dogfooding（生成→check→负向断言）、层2 生成项目 pre-commit 钩子（含 stale）。层3/4 已内置，仅确认。

**Architecture:** 改 `internal/scaffold/shared/precommit.go` 的 `PreCommitConfig` 常量（渲染生成项目的 `.pre-commit-config.yaml`）+ 扩展 `scripts/smoke.sh`。一处模板改动，mono/bff/rpc/micro 四类生成器经共享常量全部生效，黄金快照同步更新。

**Tech Stack:** bash (smoke), YAML pre-commit hooks, Go (shared/precommit.go 常量)。

## Global Constraints

- 生成项目的 pre-commit 配置是 contract-sensitive（黄金 testdata 锁定）。
- `ncgo check` 钩子含 context.stale 检查（提交前必须 `ai sync`），此行为已确认。
- 钩子放 pre-commit 阶段（快），go-vet/test/build 保持 pre-push（重）。
- 覆盖全部生成器（mono/bff/rpc/micro），黄金快照同步更新。
- smoke 保持快速、非破坏性。
- 最终验证：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。

---

### Task 1: 层1 — smoke dogfooding 增强

**Files:**
- Modify: `scripts/smoke.sh`

**Interfaces:**
- Consumes: `ncgo new --no-generate`（生成确定性 mono 项目，含 usecase 锚点）、`ncgo check --root .`（exit 0/1/2）。

- [ ] **Step 1**: 在 smoke.sh 现有 check 步骤后追加两段（dogfooding + 负向）：

```bash
log "generated project passes ncgo check (dogfooding)"
GEN_ROOT="$TMP_DIR/gen-check"
"$BIN" new gen-check --module github.com/x/gen-check --kind hertz --no-generate --dir "$GEN_ROOT" >/dev/null
"$BIN" check --root "$GEN_ROOT" >"$TMP_DIR/gen-check.out"
grep -q 'all checks passed' "$TMP_DIR/gen-check.out"

log "broken anchor fails ncgo check (negative)"
USE_CASE=$(find "$GEN_ROOT/internal/usecase" -name '*.go' | head -1)
sed -i '' '/ncgo:methods:start/d' "$USE_CASE"
"$BIN" check --root "$GEN_ROOT" >/dev/null 2>&1 && { echo "check should have failed"; exit 1; } || true
```

> **注意**：macOS 的 `sed -i ''` 与 GNU sed 语法不同。smoke 在 ubuntu CI 跑，用 `sed -i`（GNU）。落地时用 `sed -i` 且确认生成的 usecase 文件确实含 `ncgo:methods:start`。

- [ ] **Step 2**: 跑 `./scripts/smoke.sh` 确认新步骤通过。
- [ ] **Step 3**: Commit。

### Task 2: 层2 — 生成项目 pre-commit 加 ncgo-check 钩子

**Files:**
- Modify: `internal/scaffold/shared/precommit.go`（`PreCommitConfig` 常量）
- Regenerate: 8 个黄金 testdata 快照

**Interfaces:**
- Produces: 生成的 `.pre-commit-config.yaml` 含 `ncgo-check` local hook。

- [ ] **Step 1**: 在 `PreCommitConfig` 的 `- repo: local` 下、gofmt 之后插入：

```yaml
      - id: ncgo-check
        name: ncgo check (anchors/manifest/context)
        entry: ncgo check --root . --output text
        language: system
        pass_filenames: false
        always_run: true
        stages: [pre-commit]
```

- [ ] **Step 2**: 更新黄金快照：对受影响的 4 类生成器跑 golden 更新。

```bash
go test ./internal/scaffold/mono/... -update-golden -count=1
go test ./internal/scaffold/bff/... -update-golden -count=1
go test ./internal/scaffold/rpc/... -update-golden -count=1
go test ./internal/scaffold/micro/... -update-golden -count=1
```

- [ ] **Step 3**: 检查生成的 `.pre-commit-config.yaml` 快照含 `ncgo-check`；确认 gofmt/vet/test/build 钩子仍在。
- [ ] **Step 4**: Commit。

### Task 3: 层3/4 确认（无代码改动）

- [ ] **Step 1**: 确认 `ncgo-dev` skill WorkflowBody 已含 `check → 修 → ai sync → 再 check`（无需改动）。
- [ ] **Step 2**: 确认手动 `ncgo check` / `ai sync` 兜底已存在（无需改动）。

### Task 4: 全量验证

- [ ] **Step 1**: `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`
- [ ] **Step 2**: 修复发现的问题，重跑直到全绿。

---

## Self-Review

**Spec coverage:** 层1 (smoke dogfooding + 负向) → Task 1 ✓; 层2 (pre-commit hook 全生成器 + 黄金) → Task 2 ✓; 层3/4 (确认) → Task 3 ✓; 全量验证 → Task 4 ✓.
**Placeholder scan:** 无 TBD；两处 GNU/macOS sed 差异已在注释标注。
**Type consistency:** 无跨任务类型依赖（bash + YAML 常量）。

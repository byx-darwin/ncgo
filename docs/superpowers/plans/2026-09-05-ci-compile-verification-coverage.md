# CI Compile-Verification Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make CI actually install `hz`/`kitex`/`protoc`/`sqlc` so the currently-`t.Skip`ped `TestGenerateHertzCompiles`, `TestGenerateHertzWithDatabaseCompiles`, and `TestGenerateKitexCompiles` run for real on every push/PR, and document the new prerequisite (`sqlc`) for local parity.

**Architecture:** Add one new step to the existing `test` job in `.github/workflows/ci.yml` — no new job, no test-code changes. The three tests already call `exec.LookPath("hz")` / `requireTools(t, ...)` and only skip when a tool is missing; once the tools are on `PATH`, `go test ./... -count=1` (already a step in the job) executes them for real, with zero code changes.

**Tech Stack:** GitHub Actions (`ubuntu-latest` runner), `apt-get` for `protoc`, `go install` for `hz`/`kitex`/`sqlc`.

**Spec:** `docs/superpowers/specs/2026-09-05-ci-compile-verification-coverage-design.md`

## Global Constraints

- `hz` version pinned to `v0.9.7` (CONTRIBUTING.md documented minimum) — not `@latest`.
- `kitex` version pinned to `v0.16.1` (CONTRIBUTING.md documented minimum) — not `@latest`.
- `sqlc` installed at `@latest` — no minimum version is documented anywhere in this repo today.
- No new CI job — extend the existing `test` job in `.github/workflows/ci.yml`.
- No dedicated `actions/cache` step for the installed tools (decided YAGNI in the design doc).
- No changes to `internal/scaffold/mono/mono_test.go` or any other Go test file — the tests are already correct, they just need tools on `PATH`.
- No changes to `.github/PULL_REQUEST_TEMPLATE.md`.
- English (`CONTRIBUTING.md`) and Chinese (`CONTRIBUTING.zh-CN.md`) docs must stay aligned per this repo's convention.

---

### Task 1: Install code-gen tools in CI and verify the compile tests run

**Files:**
- Modify: `.github/workflows/ci.yml` (insert a new step between the existing `Download modules` step at line 24-26 and the `Check formatting` step at line 28-34)

**Interfaces:**
- Consumes: nothing (this is a standalone CI config change)
- Produces: `hz`, `kitex`, `protoc`, `sqlc` binaries on the CI runner's `PATH` before the `Test` step runs — later tasks (Task 2 doc updates) reference these exact tool names and pinned versions.

- [ ] **Step 1: Read the current workflow file to confirm the exact insertion point**

Run: `cat .github/workflows/ci.yml`

Confirm the step order is: `Checkout` → `Setup Go` → `Download modules` → `Check formatting` → `Vet` → `Test` → `Build CLI` → `Smoke test` → `Verify polaris adapter compiles against pinned SDK`. The new step goes immediately after `Download modules` and before `Check formatting`.

- [ ] **Step 2: Insert the "Install code-gen tools" step**

Edit `.github/workflows/ci.yml` so the file reads exactly as follows (only the new step and the `PATH` env line are additions — everything else is unchanged):

```yaml
name: CI

on:
  push:
  pull_request:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  test:
    name: test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Download modules
        run: go mod download

      - name: Install code-gen tools
        run: |
          sudo apt-get update
          sudo apt-get install -y protobuf-compiler
          go install github.com/cloudwego/hertz/cmd/hz@v0.9.7
          go install github.com/cloudwego/kitex/tool/cmd/kitex@v0.16.1
          go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
          echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"
          protoc --version
          hz --version
          kitex --version
          sqlc version

      - name: Check formatting
        run: |
          unformatted="$(gofmt -l $(find . -name '*.go' -not -path './.git/*'))"
          if [ -n "$unformatted" ]; then
            echo "Go files need gofmt:"
            echo "$unformatted"
            exit 1
          fi

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test ./... -count=1

      - name: Build CLI
        run: go build .

      - name: Smoke test
        run: ./scripts/smoke.sh

      - name: Verify polaris adapter compiles against pinned SDK
        run: ./scripts/verify-polaris-adapter.sh
```

Note: `echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"` is the GitHub Actions mechanism for persisting a `PATH` addition to all subsequent steps in the job (a plain `export`/`PATH=` assignment in one `run:` step does not carry over to the next step, since each `run:` step is a fresh shell). The four version-print commands (`protoc --version`, `hz --version`, `kitex --version`, `sqlc version`) both fail fast if installation was silently broken and leave a readable version trail in the Actions log for future debugging.

- [ ] **Step 3: Validate the YAML is well-formed**

Run: `python3 -c "import yaml, sys; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo OK`

Expected: `OK` (no YAML parse error). If Python/PyYAML isn't available locally, alternatively run `gofmt -l .github/workflows/ci.yml 2>/dev/null; true` is not applicable (not a Go file) — use `cat .github/workflows/ci.yml` and manually re-check indentation instead, or push to a branch and let GitHub Actions' own YAML parser validate it (Task 1 Step 5 below covers this).

- [ ] **Step 4: Run the currently-skipped tests locally to confirm they still pass on this machine (sanity check before trusting CI)**

Run: `go test ./internal/scaffold/mono/... -run 'TestGenerateHertzCompiles|TestGenerateHertzWithDatabaseCompiles|TestGenerateKitexCompiles' -v -count=1`

Expected: all three tests `PASS` if `hz`/`kitex`/`make`/`protoc`/`sqlc` are already on this machine's `PATH` (per `CONTRIBUTING.md` prerequisites), or `SKIP` with a clear "not found on PATH" message per-tool if not. Either outcome is fine locally — this step is a smoke check that the tests themselves are not broken; the actual coverage assertion is CI passing them for real (Step 5).

- [ ] **Step 5: Push to a branch and confirm the three tests show `PASS` (not `SKIP`) in the GitHub Actions log**

This step cannot be run as a local shell command — it requires the delivery step (PR / branch push) that happens later in the gf-workflow's Phase 3. Record as a manual verification note: after the branch is pushed and CI runs, open the Actions log for the `test` job's `Test` step and confirm none of `TestGenerateHertzCompiles`, `TestGenerateHertzWithDatabaseCompiles`, `TestGenerateKitexCompiles` are reported as `SKIP`.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: install hz/kitex/protoc/sqlc so compile-verification tests run for real"
```

---

### Task 2: Document the new CI coverage and the previously-undocumented `sqlc` prerequisite

**Files:**
- Modify: `CONTRIBUTING.md` (prerequisites list, lines 11-14)
- Modify: `CONTRIBUTING.zh-CN.md` (prerequisites list, lines 15-18)

**Interfaces:**
- Consumes: the exact pinned versions and tool names from Task 1 (`hz >= v0.9.7`, `kitex >= v0.16.1`, `sqlc`, `protoc`) — must match verbatim so docs don't drift from what CI actually installs.
- Produces: nothing consumed by later tasks (this is the last task in the plan).

- [ ] **Step 1: Update `CONTRIBUTING.md`'s prerequisites section**

Current text (lines 11-16):

```markdown
## Development prerequisites / 开发前提

- Go `1.25+`
- `hz >= v0.9.7` when working on Hertz generator flows
- `kitex >= v0.16.1` when working on Kitex generator flows

If you only need to inspect scaffold inputs, prefer `--no-generate` workflows.
```

Replace with:

```markdown
## Development prerequisites / 开发前提

- Go `1.25+`
- `hz >= v0.9.7` when working on Hertz generator flows
- `kitex >= v0.16.1` when working on Kitex generator flows
- `sqlc` when running database-backed generator flows locally (`TestGenerateHertzWithDatabaseCompiles`, `TestGenerateKitexCompiles`) — install with `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- `protoc` (the Protocol Buffers compiler) for any flow that generates or verifies IDL-derived code

If you only need to inspect scaffold inputs, prefer `--no-generate` workflows.

CI installs pinned `hz`/`kitex`/`protoc`/`sqlc` and runs the real compile-verification tests (`TestGenerateHertzCompiles`, `TestGenerateHertzWithDatabaseCompiles`, `TestGenerateKitexCompiles`) on every push and PR — they are no longer silently skipped. See `.github/workflows/ci.yml`'s `Install code-gen tools` step for the exact versions.
```

- [ ] **Step 2: Update `CONTRIBUTING.zh-CN.md`'s prerequisites section**

Current text (lines 13-19):

```markdown
## 开发前提

- Go `1.25+`
- 如果会涉及 Hertz 生成流程：`hz >= v0.9.7`
- 如果会涉及 Kitex 生成流程：`kitex >= v0.16.1`

如果你当前只需要检查脚手架输入文件，优先使用 `--no-generate` 工作流。
```

Replace with:

```markdown
## 开发前提

- Go `1.25+`
- 如果会涉及 Hertz 生成流程：`hz >= v0.9.7`
- 如果会涉及 Kitex 生成流程：`kitex >= v0.16.1`
- 如果需要在本地运行数据库相关生成流程测试（`TestGenerateHertzWithDatabaseCompiles`、`TestGenerateKitexCompiles`）：`sqlc`，安装方式为 `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- 任何涉及 IDL 代码生成或校验的流程都需要 `protoc`（Protocol Buffers 编译器）

如果你当前只需要检查脚手架输入文件，优先使用 `--no-generate` 工作流。

CI 现在会安装锁定版本的 `hz`/`kitex`/`protoc`/`sqlc`，并在每次 push 和 PR 时真实运行编译验证测试（`TestGenerateHertzCompiles`、`TestGenerateHertzWithDatabaseCompiles`、`TestGenerateKitexCompiles`），不再被静默跳过。具体版本见 `.github/workflows/ci.yml` 中的 `Install code-gen tools` 步骤。
```

- [ ] **Step 3: Run markdown diagnostics on both files**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')` — expected empty (this is unrelated to the markdown edits but is the repo's standard "did I break Go formatting" check; run it since editing files in the same commit is a good habit). For the markdown files themselves, visually diff the before/after with `git diff CONTRIBUTING.md CONTRIBUTING.zh-CN.md` and confirm heading levels, list markers, and code fences are well-formed (no unclosed ``` blocks, no broken nesting).

- [ ] **Step 4: Commit**

```bash
git add CONTRIBUTING.md CONTRIBUTING.zh-CN.md
git commit -m "docs: document sqlc prerequisite and new CI compile-verification coverage"
```

---

## Final Validation

- [ ] Run the full local check suite: `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`
- [ ] Confirm `git log --oneline -2` shows both commits from Task 1 Step 6 and Task 2 Step 4
- [ ] After delivery (branch pushed / PR opened), confirm in the GitHub Actions log that `TestGenerateHertzCompiles`, `TestGenerateHertzWithDatabaseCompiles`, and `TestGenerateKitexCompiles` show `PASS`, not `SKIP` — this is the acceptance criterion from Issue #104

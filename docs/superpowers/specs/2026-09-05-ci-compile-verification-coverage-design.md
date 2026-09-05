# CI Compile-Verification Coverage for hz/kitex/protoc/sqlc

**Issue:** [#104](https://github.com/byx-darwin/ncgo/issues/104)
**Classification:** Bounded (extends existing `.github/workflows/ci.yml` `test` job and existing, currently-skipped tests; no new subsystem)
**Status:** Approved

## Problem

`internal/scaffold/mono/mono_test.go` has `TestGenerateHertzCompiles`,
`TestGenerateHertzWithDatabaseCompiles`, and `TestGenerateKitexCompiles`,
which build a generated project against the real, network-resolved
`go-tools` dependency. They `t.Skip` unless `hz`/`make`/`protoc`
(and `kitex`/`sqlc` for their variants) are on `PATH`. CI's `test` job only
runs `actions/setup-go` — none of these tools are installed — so the tests
are silently skipped in CI today. If a future `go-tools` release renames or
drops a field/method ncgo's templates depend on (as nearly happened with the
`Insecure` field in v0.3.0, see #95), nothing in CI would catch it.

## Decision

Option (a): add real CI coverage, rather than (b) documentation-only manual
verification. Precedent already exists in this repo —
`scripts/verify-polaris-adapter.sh` runs unconditionally in CI and performs a
real compile check against a pinned SDK. The same approach extends naturally
to the hz/kitex compile tests.

## Design

Extend the existing `test` job in `.github/workflows/ci.yml` — no new job.

1. **New step "Install code-gen tools"**, after `Download modules`, before
   `Test`:
   - `protoc`: `sudo apt-get install -y protobuf-compiler` (ubuntu-latest has
     apt; `make` is already preinstalled on the runner)
   - `hz`: `go install github.com/cloudwego/hertz/cmd/hz@v0.9.7` — pinned to
     the minimum version documented in `CONTRIBUTING.md`, not `@latest`, to
     keep CI deterministic and aligned with what contributors are told to
     install
   - `kitex`: `go install github.com/cloudwego/kitex/tool/cmd/kitex@v0.16.1`
     — same reasoning
   - `sqlc`: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` — not
     currently documented as a prerequisite anywhere, despite being required
     to run `TestGenerateHertzWithDatabaseCompiles` /
     `TestGenerateKitexCompiles` locally
   - Ensure `$(go env GOPATH)/bin` is on `PATH` for subsequent steps

2. **No dedicated caching step.** `actions/setup-go`'s `cache: true` already
   caches the Go module/build cache keyed on `go.sum`; the three `go install`
   calls take a few seconds each. Adding `actions/cache` for `GOBIN` is
   unnecessary complexity for the time it would save (YAGNI) — revisit only
   if CI timing later shows it matters.

3. **No test-code changes.** The existing `go test ./... -count=1` step
   naturally executes the three previously-skipped tests once the tools are
   found via `exec.LookPath`/`requireTools`.

4. **Docs:** update `CONTRIBUTING.md` (and `CONTRIBUTING.zh-CN.md`) to add
   `sqlc` to the prerequisites list, and note that CI installs pinned
   `hz`/`kitex`/`protoc`/`sqlc` and runs these compile tests — so
   contributors touching `go-tools`-dependent templates get real CI signal,
   not just local best-effort verification.

5. **No PR template change, no separate job.** Option (a) is chosen in full;
   option (b)'s "confirm which compile tests were run locally" framing is not
   needed.

## Testing

Pure CI/docs change — no new Go tests. Validation: the CI run must show the
three previously-skipped tests as `PASS` (not `SKIP`) in the Actions log;
`gofmt`/`go vet` on touched files; `./scripts/smoke.sh` locally.

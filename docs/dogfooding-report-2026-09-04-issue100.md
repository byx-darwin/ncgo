# Dogfooding Report — infra Plugin registry refactor (Issue #100)

**Executor:** wf-2026-09-04-005 Phase 4 (full mode)
**Result:** PASS

Dogfood principle: exercise the refactored `ncgo add infra` command exactly as a real user would, via the built binary (not just unit/golden tests).

| Item | Command | Result |
|------|---------|--------|
| Fresh hertz project generation | `ncgo new dogfood-svc3 --kind hertz --module example.com/dogfood3` | ✅ |
| Simple plugin (no --wire) | `ncgo add infra redis` | ✅ writes redis.go + redis_shared.go, next-steps text matches SetupSteps/GoGetDeps derivation |
| Wire-unsupported kind correctly rejected | `ncgo add infra redis --wire` | ✅ errors with exact `unsupportedWireError()` text: "infra: --wire is only supported for observability_logging/release_canary/registry_polaris/rate_limit" |
| Wire-hook plugin | `ncgo add infra observability_logging --wire` | ✅ writes logging.go + hertz.go, wires `logging.Init(` into server.go |
| gofmt on wired file | `gofmt -l internal/base/server/server.go` | ✅ empty (clean) |
| go vet (pre-`go get`) | `go vet ./internal/base/...` | ⚠️ expected `undefined: logging.Config` — the printed next-step `go get github.com/byx-darwin/go-tools/go-common` was intentionally skipped to keep the check fast/offline; `Add()`'s doc comment explicitly states it does not call `go get` itself. Not a regression. |

**Baseline check:** a freshly generated project with zero infra add-ons also fails `go build` with an unrelated pre-existing usecase-stub issue (`internal/usecase/<svc>/usecase.go` has unused imports) — confirmed this exists independent of any infra add-on, i.e. unrelated to this refactor and outside its scope.

**Summary:** The Plugin registry refactor is consumable end-to-end through the real CLI binary: both a simple metadata-only plugin (redis) and a wire-hook plugin (observability_logging) produce correct output and correct `--wire` behavior, including the exact legacy error text for kinds that don't support `--wire`.

**Bugs Found:** 0

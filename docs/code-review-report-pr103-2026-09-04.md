# Code Review Report — PR #103 (Phase-4 Delivery-Gate Review)

- **Repo:** byx-darwin/ncgo
- **PR:** #103 — `chore(deps): 升级 go-tools 钉死版本 v0.1.0 → v0.3.0 并修复 Insecure 透传`
- **Branch:** `feat/95-go-tools-v0.3.0-upgrade` → `main`
- **Closes:** #95
- **Status at review time:** merged (`mergedAt: 2026-09-04T08:40:23Z`)
- **Reviewer role:** independent Phase-4 delivery-gate check (fresh eyes, after 2 prior review rounds: per-commit reviewers + a whole-branch senior review that already found and fixed stale `ObservabilityConfig` field-list prose in 4 embedded design docs — not re-litigated here)
- **Method:** fetched full unified diff via `gf pr diff 103` (not `gh`), cross-checked against the current working tree for template-site completeness (grep for all `ObservabilityConfig{` / `JaegerOption` / version-pin literals across `internal/assets/_data` and every `internal/scaffold/*` package), read `mono_test.go` diff and the accompanying plan doc, and inspected `.github/workflows/ci.yml` and `scripts/smoke.sh` for what is/isn't actually exercised by automation.

## Scope Verified

- Version bump `v0.1.0` → `v0.3.0` for `go-common`/`go-framework` in both generator paths (`internal/scaffold/mono/files.go: writeKitexGoMod`, `internal/assets/_data/hertz/layout.yaml`). `go-middleware` correctly left at v0.1.0, matches stated out-of-scope note.
- `Insecure` pass-through confirmed at all 3 — and only 3 — `ObservabilityConfig{...}` construction sites repo-wide (hertz `server_go.yaml`, kitex `server.yaml`, kitex `ratelimit_server.yaml`); grep across the whole `internal/assets/_data` tree and every `internal/scaffold/*` package found no other construction site and no other golden suite (checked `micro`, `kitexclient`, `rulecenter`, `domain`, `template`, `test` — none embed these templates). Matches the PR's claimed 3-site/3-suite scope exactly.
- `Insecure: true` dev-default added only to hertz `conf_go.yaml`/`conf_dev_yaml.yaml`; confirmed kitex's `conf_dev.yaml` has no jaeger block at all today (only a nil-by-default `*config.JaegerOption` field in the struct, guarded by `cfg.Jaeger != nil && cfg.Jaeger.Enable` before use), so the asymmetry is real and pre-existing, not introduced or masked by this PR.
- Golden fixtures for `mono` (5 scenarios), `rpc` (2), and `bff` (1) all show exactly the version-string + `Insecure`-field diffs with no unrelated churn — consistent with the templates they mirror.
- Bilingual docs (`README.md`/`.zh-CN`, `docs/examples.md`/`.zh-CN`, embedded hertz/kitex design docs EN/ZH) updated consistently; cross-reference from `docs/examples.zh-CN.md` to the README section title was updated together with the title itself (no dangling reference).
- Historical/pinned docs under `specs/017-go-tools-v0.1.0-adaptation*.md` and `docs/eval-go-engineer.md` still say v0.1.0 — correctly untouched, since they're dated historical records of a past PR series, not living contract docs.

## Findings

### 1. (Medium) The new `Insecure` field's compile-correctness against the real go-tools v0.3.0 API is not exercised by CI or `smoke.sh`

The entire fix rests on `go-tools@v0.3.0`'s `config.JaegerOption` and `config.ObservabilityConfig` actually exporting an `Insecure bool` field with these exact names. That is only checkable by compiling a generated project against the real, published module.

- `internal/scaffold/mono/mono_test.go` does have `TestGenerateHertzCompiles`, `TestGenerateHertzWithDatabaseCompiles`, and `TestGenerateKitexCompiles`, which would catch this — but all three `t.Skip` unless `hz`, `make`, and `protoc` (kitex too, for the kitex variant) are on `PATH`.
- `.github/workflows/ci.yml`'s `test` job does not install `hz`/`kitex`/`protoc`, so these tests are skipped in CI as it stands today (confirmed by reading the workflow — no toolchain setup steps beyond `actions/setup-go`).
- `scripts/smoke.sh` does not generate-and-build a real project against network-resolved dependencies either (checked: no `go mod tidy`/`go build` invocation on a freshly `ncgo new`-ed tree — only output-text greps).
- The PR's own test-plan checklist doesn't explicitly claim these three compile tests ran (just "all packages" + smoke.sh), so it's unclear whether this was verified with local tooling before merge, or only validated by golden-string diffing (which would pass even if `go-tools` v0.3.0 had renamed or dropped the field).

This isn't a defect in the change as written — the diff is internally consistent and the golden tests are correct reflections of the templates — but it is a real residual-risk gap for a delivery gate: if `go-tools@v0.3.0`'s actual field name/shape differs even slightly from what's assumed here, every generated Hertz/Kitex project would fail to compile, and nothing in CI would catch it before it reaches a user running `ncgo new`.

**Suggested follow-up (not a blocker since the PR is already merged):** file a tracking issue to either (a) add a CI job that installs `hz`/`kitex`/`protoc` and runs the skipped compile tests, or (b) at minimum, have the PR author state explicitly in the PR body which compile tests were run locally and confirm the toolchain versions used, so future reviewers don't have to infer coverage from `t.Skip` conditions.

## Not Re-Flagged (already covered by prior review rounds)

- Stale `ObservabilityConfig` field-list prose in the 4 embedded design docs — already caught and fixed in the whole-branch review; diff confirms the fix (`Insecure` now listed alongside `Enabled, Endpoint, ServiceName` in all 4 files).
- `tools/verifyexamples/polaris-adapter/go.mod` still pinning `go-common v0.1.0` — already flagged by the PR author as an intentional out-of-scope follow-up; verified the file is indeed untouched by this diff, consistent with that note.

## Verdict

No correctness, contract, or completeness defects found in the diff itself — the version bump, `Insecure` wiring, golden regeneration, and bilingual docs are all internally consistent and match the stated scope exactly. One residual-risk item (finding #1) is worth a tracking issue but is not a reason to reverse a merge that has already gone through two review rounds.

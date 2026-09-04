# Fix: micro-mode compose/Dockerfile support for `go.mod` local `replace` deps (Issue #94)

## Context

In `micro` mode, ncgo scaffolds each service's `compose.yaml` build context as `./services/<name>` and its Dockerfile with `COPY . .`, assuming every service is fully self-contained. This breaks when a service's `go.mod` has a local `replace <module> => ../<sibling>` directive pointing at a sibling service's source — a pattern used to share Go types/logic across services without a published module. ncgo has no awareness of this: it doesn't detect `replace` directives and doesn't adjust `compose.yaml`/Dockerfile accordingly. The only symptom is a hard Docker build failure (`failed to calculate checksum of ref ...: "/services/<name>": not found`) that surfaces during an actual `docker compose up --build`, not during `go build`/`go vet`/`ncgo check`. A prior real-world fix for one such service was applied by hand and the same fix was missed for another service in the same commit — because nothing ties the Dockerfile's COPY list to compose.yaml's build context, this class of drift is easy to introduce and invisible until deploy time.

This plan implements the issue's suggested fix in full: detect local replace deps at scaffold time, generate the correct `compose.yaml`/Dockerfile shape for affected services, and add an `ncgo doctor` check to catch drift introduced by later hand-edits to `go.mod` (the realistic path, since a `replace ../sibling` can only be added after both service directories already exist — i.e. after initial scaffolding).

## Confirmed design decisions

1. **go.mod parsing**: add `golang.org/x/mod/modfile` as a new dependency (`go get golang.org/x/mod`) rather than manual line-scanning — handles grouped `replace (...)` blocks, comments, and versioned-vs-local targets correctly.
2. **Doctor check**: in scope for this issue (not deferred) — it's the main safety net for the realistic hand-edit-after-scaffold flow.
3. **`.dockerignore`**: when any service's compose entry gets `context: .`, ensure `<workspace-root>/.dockerignore` exists with the same exclude patterns as the per-service one — idempotent, additive, never overwrites a user's existing root file.
4. **Tests**: unit tests only in `internal/scaffold/shared` and `internal/doctor`; no new end-to-end golden tree in `internal/scaffold/micro/golden_test.go` (keeps diff minimal, avoids new testdata fixture churn).

## Implementation steps

### 1. Add dependency
`go get golang.org/x/mod` → updates `go.mod`/`go.sum`.

### 2. New file: `internal/scaffold/shared/gomod_replace.go`

```go
type LocalReplace struct {
    Module string // replaced module path (LHS)
    Target string // relative filesystem target as written, e.g. "../authority"
}

// ParseLocalReplaces reads <serviceDir>/go.mod and returns every replace
// directive whose target is a local filesystem path (no version on the RHS).
// Returns (nil, nil) if go.mod does not exist yet.
func ParseLocalReplaces(serviceDir string) ([]LocalReplace, error)

// SiblingDirs resolves each LocalReplace's Target against serviceDir and
// matches it by resolved filesystem path to a manifest.WorkspaceService.Dir.
// Returns matched sibling Dir values (workspace-root-relative, slash form),
// sorted and de-duplicated. Excludes the service's own Dir.
func SiblingDirs(root, serviceRel string, replaces []LocalReplace, services []manifest.WorkspaceService) []string
```

- `ParseLocalReplaces`: `os.ReadFile(filepath.Join(serviceDir, "go.mod"))`; `os.IsNotExist` → `nil, nil`; else `modfile.Parse(path, data, nil)`, keep `f.Replace` entries where `r.New.Version == ""` and `r.New.Path` starts with `./` or `../`.
- `SiblingDirs`: for each replace, resolve `filepath.Clean(filepath.Join(root, serviceRel, r.Target))` and compare against `filepath.Clean(filepath.Join(root, svc.Dir))` for each `svc` in `services`; match → record `svc.Dir`.

### 3. `internal/scaffold/shared/container.go` changes

- `composeApp` struct: add `Dockerfile string` field (empty = default `"Dockerfile"`; keeps `WriteMonoCompose`'s standalone `context: "."` path unaffected).
- `renderAppCompose` (~line 276-301): emit `app.Dockerfile` if set, else `"Dockerfile"`.
- `loadWorkspaceComposeApps` (~line 175-214): after resolving `serviceRoot` for each `svc`, call `ParseLocalReplaces(serviceRoot)` then `SiblingDirs(root, svc.Dir, replaces, w.Services)`. If siblings found: `app.Context = "."`, `app.Dockerfile = filepath.ToSlash(filepath.Join(svc.Dir, "Dockerfile"))`. Else leave current behavior (`Context: "./" + svc.Dir`, `Dockerfile: ""`).
- `WriteWorkspaceCompose` / its render path: if any app ended up with `Context == "."`, ensure `<root>/.dockerignore` exists — write it using the same content as `serviceDockerIgnore` (the existing per-service dockerignore template) only if the file doesn't already exist (never overwrite user content).

### 4. New function for multi-COPY Dockerfile: `RewriteServiceDockerfileForSiblings`

Added to `internal/scaffold/shared/container.go` (or a new `dockerfile_siblings.go` in the same package). Not a parameter added to `WriteServiceContainerFiles` — that function is shared by standalone mono/rpc/bff flows via `mono.Generate` and must stay a no-op for the non-sibling case.

```go
// RewriteServiceDockerfileForSiblings overwrites dir/Dockerfile with a
// variant whose builder stage COPYs each sibling directory (workspace-root
// relative, e.g. "services/authority") plus the service's own directory,
// then sets WORKDIR to the service's own root-relative path before the
// build step — so `go build` resolves `../sibling` replace targets exactly
// as they resolve on disk. Call only when len(siblings) > 0.
func RewriteServiceDockerfileForSiblings(dir, kind, rootRel string, siblings []string) error
```

Factor the shared boilerplate (final stage, base image, `.dockerignore` content) out of `WriteServiceContainerFiles` into a small internal helper reused by both functions, to avoid duplicating the Dockerfile template twice — minimal extraction, not a broad refactor.

### 5. Wiring: `rpc.Add()` / `bff.Add()`

In `internal/scaffold/rpc/rpc.go` and `internal/scaffold/bff/bff.go`, after `mono.Generate` succeeds and the service is merged into the workspace (~rpc.go:108-115), call `ParseLocalReplaces` + `SiblingDirs` (candidate set = `w.Services` plus the just-added service) and, if non-empty, call `RewriteServiceDockerfileForSiblings`. This handles idempotent re-scaffolds where go.mod already has a replace; the doctor check (step 6) is the primary safety net for the more common "replace added by hand after both services already exist" flow, since `Add()` runs before a user could plausibly add a cross-service replace.

`RefreshWorkspaceComposeForServiceRoot` (container.go, called from `internal/scaffold/infra/infra.go`) needs no wiring change — `loadWorkspaceComposeApps` re-derives `Context`/`Dockerfile` from current go.mod on every compose regen, so it already re-detects replaces on each refresh.

### 6. New doctor check: `internal/doctor/compose_check.go`

```go
// composeConsistencyChecks validates, for a micro workspace, that each
// service's go.mod local replace directives are reflected in compose.yaml's
// build.context/dockerfile and in the service's own Dockerfile COPY list.
func composeConsistencyChecks(root string, w *manifest.Workspace) []Check
```

Follows `internal/doctor/infra_check.go`'s style (`Check{ID, Severity, Message, Hint}`, one check per finding). For each service with `len(siblings) > 0`:
- **Check (a)** `compose.context.<name>`, `SeverityError`: parse `compose.yaml` (minimal YAML struct via `gopkg.in/yaml.v3`), fail if that service's `build.context != "."` or `build.dockerfile != "<svc.Dir>/Dockerfile"`. Hint points to re-running `ncgo add infra` on the service (triggers refresh) or editing compose.yaml directly.
- **Check (b)** `compose.dockerfile.<name>`, `SeverityError`: read the service's Dockerfile, fail if it doesn't contain a `COPY <sibling>/` line for each detected sibling. Hint suggests re-adding the sibling COPY line or re-running scaffold.

Skip emitting a check for services with no detected siblings (symmetric with `checkInfraFiles`'s pattern of only emitting for relevant cases).

Wire into `internal/doctor/doctor.go`'s `projectChecks`, workspace branch, right after `workspaceProtoLintChecks(ctx, root, w)`:
```go
out = append(out, composeConsistencyChecks(root, w)...)
```

### 7. Tests

- `internal/scaffold/shared/gomod_replace_test.go` (new): table tests for `ParseLocalReplaces` (missing go.mod, no replace, single-line replace, grouped `replace (...)` block, versioned replace ignored, module-path replace ignored) and `SiblingDirs` (path-resolved match, self-exclusion, out-of-workspace target ignored, dedup + sort).
- `internal/scaffold/shared/container_test.go`: add `TestLoadWorkspaceComposeAppsWithLocalReplace` next to the existing `TestWriteWorkspaceComposeLoadsServiceManifestDependencies` — two temp service dirs, one go.mod with a `replace` to the other; assert rendered compose has `context: .` + `dockerfile: services/<name>/Dockerfile` for the replacing service and unchanged output for the sibling. Add `TestRenderAppComposeDefaultDockerfile` to lock the empty-`Dockerfile`-field default. Add a test for `RewriteServiceDockerfileForSiblings` asserting `COPY services/<sibling>/ services/<sibling>/`, correct `WORKDIR`, and absence of bare `COPY . .`. Add a test asserting root `.dockerignore` gets created (and is left alone if already present) when a sibling-context app exists.
- `internal/doctor/compose_check_test.go` (new): no-replace (no checks emitted), replace + correct compose/Dockerfile (checks pass), replace + stale compose context (error a), replace + Dockerfile missing COPY (error b).
- Run existing `internal/scaffold/mono/golden_test.go`, `internal/scaffold/rpc/*_test.go`, `internal/scaffold/bff/*_test.go`, `internal/scaffold/micro/golden_test.go` unchanged to confirm the standalone (`Context: "."`) and empty-workspace paths are unaffected — no golden fixture updates expected; if any diff appears, it indicates an unintended behavior change to fix, not a golden update to accept.

### 8. Docs

- `README.md` / `README.zh-CN.md`: add a note under the micro-mode / compose generation section describing the local-replace detection behavior (context/dockerfile shape, sibling COPY, root `.dockerignore`).
- `docs/examples.md` / `docs/examples.zh-CN.md`: mirror if they document micro-mode's generated compose/Dockerfile layout.
- `docs/prd.md`: check if it's cited as the authoritative generation-rules doc (referenced from `internal/manifest`'s package doc) and update if so.
- Mention the new `compose.context.*` / `compose.dockerfile.*` doctor check IDs in doctor's documented check list, if one exists.

## Critical files

- `internal/scaffold/shared/container.go` — `composeApp`, `renderAppCompose`, `loadWorkspaceComposeApps`, `WriteServiceContainerFiles`
- `internal/scaffold/shared/gomod_replace.go` (new)
- `internal/scaffold/rpc/rpc.go`, `internal/scaffold/bff/bff.go` — `Add()`
- `internal/doctor/compose_check.go` (new), `internal/doctor/doctor.go` — `projectChecks`
- `internal/manifest/workspace.go` — `WorkspaceService.Dir` (read-only, used for path resolution)

## Verification

1. `go build ./...` and `go vet ./...`
2. `go test ./internal/scaffold/shared/... -count=1`
3. `go test ./internal/doctor/... -count=1`
4. `go test ./internal/scaffold/... -count=1` (confirm no golden diffs in mono/rpc/bff/micro)
5. `go test ./... -count=1`
6. Manual smoke: scaffold a `micro` workspace, add two rpc services, hand-edit one's `go.mod` to add `replace <other-module> => ../<other>`, run `ncgo add infra <kind>` on it (triggers compose refresh) and inspect the regenerated `compose.yaml`/Dockerfile for `context: .` + sibling `COPY` lines; then run `ncgo doctor` in the workspace before/after the refresh to confirm the check fires when stale and clears once regenerated.
7. `./scripts/smoke.sh`

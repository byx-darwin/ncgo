# Design: route Vegeta dockerfile flag through framework.ComposeFeatureFlags

**Issue:** #111
**Path:** bounded (single-file refactor)

## Problem

`internal/scaffold/mono/mono.go:147` independently re-derives the rule
"Hertz + WithDatabase enables the Vegeta load-test sidecar" via:

```go
if defaultKind(opts.Kind) == manifest.KindHertz && opts.WithDatabase {
    if err := shared.WriteVegetaDockerfile(dir); err != nil {
```

Issue #101 already centralized this rule in
`framework.Adapter.ComposeFeatures(withDatabase).Vegeta`
(`hertzAdapter` returns `Vegeta: withDatabase`; `kitexAdapter` always
returns `Vegeta: false`). The mono.go check is a second, duplicate
expression of the same rule.

## Change

Replace the inline condition at `mono.go:147` with:

```go
if framework.MustGet(defaultKind(opts.Kind)).ComposeFeatures(opts.WithDatabase).Vegeta {
```

`framework` is already imported in `mono.go`; no new import required.

## Equivalence

- Kitex: `kitexAdapter.ComposeFeatures(...)` always returns
  `Vegeta: false` → condition is always false, matching the old
  `defaultKind == KindHertz && ...` check (never true for Kitex).
- Hertz: `hertzAdapter.ComposeFeatures(withDatabase).Vegeta ==
  withDatabase` → condition reduces to `opts.WithDatabase`, matching
  the old check (`KindHertz == KindHertz && opts.WithDatabase`).

Pure refactor; no behavior or generated-output change.

## Testing

- `go test ./internal/scaffold/mono/... -count=1` — all 6
  `TestGenerateGolden*` must be zero-diff.

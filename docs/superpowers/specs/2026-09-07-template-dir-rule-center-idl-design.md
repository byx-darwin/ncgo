# Design: fix `--template-dir` rule-center IDL selection (Issue #115)

## Problem

`ncgo new <name> --template-dir <rule-center-package>` and `ncgo new <name> --preset rule-center`
resolve a different IDL file for the same rule-center template package, so a
`--template-dir` render fails `go build` (`undefined: conf.RateLimitRuleConfig`,
`cannot find module providing package .../ruleservice`, etc.).

## Root cause (confirmed via local repro)

Reproduced against `byx-darwin/ncgo-templates`'s `rule-center` package:

- `--preset rule-center` → `.ncgo/manifest.yaml` records `idl: idl/rule-center.proto`
  (correct — `service RuleService`). `mono.go:96-97` hardcodes this path, bypassing
  generic IDL discovery.
- `--template-dir <rule-center>` → manifest records `idl: idl/scratch-center.proto`
  (wrong — `service ScratchService`). This comes from `mono.go:117-124`, which picks
  `pkg.IDLs[0]` — the first file (lexical order via `filepath.Walk`) under the
  package's `idl/` directory. The rule-center package ships two generic placeholder
  files there (`{{ToLower .ServiceName}}.proto`, `{{ToLower .ServiceName}}-center.proto`)
  that are unrelated to the package's real proto. Because `-` (0x2D) sorts before
  `.` (0x2E), `{{ToLower .ServiceName}}-center.proto` is picked and rendered to
  `scratch-center.proto`.

The package's *real* proto (the one `internal/pkg/ratelimit/resolver.go`, `store.go`,
and the generated handler/server code hard-reference via `RateLimitRuleConfig` /
`ruleservice`) is written separately, to the fixed path `idl/rule-center.proto`, by
`overlayTemplatePackage`'s per-file copy of `kitex-template/ratelimit_proto.yaml`.
That mechanism is completely disconnected from the `pkg.IDLs[0]` selection in
`mono.go`, so under `--template-dir` the generator (`kitex`) is invoked against the
wrong IDL file even though the correct one is present on disk.

## Fix

In `internal/scaffold/mono/mono.go`, inside the `opts.TemplateDir != ""` branch,
special-case the rule-center package the same way the `--preset rule-center` branch
already does: when `pkg.Meta.Name == "rule-center"`, force
`idl = filepath.ToSlash(filepath.Join("idl", "rule-center.proto"))` and skip the
generic `pkg.IDLs[0]` resolution for this package. The generic resolution logic is
left unchanged for every other template package.

## Scope

- Files: `internal/scaffold/mono/mono.go` (~10 lines).
- Tests: extend `internal/scaffold/mono` tests (golden or integration) to cover
  `--template-dir` against a rule-center-shaped package, asserting the resolved
  `idl` / manifest value and generator invocation target.
- Out of scope: the two unused generic placeholder proto files
  (`scratch.proto` / `scratch-center.proto`) that still get copied into the
  output tree — harmless to the build, and they live in the `ncgo-templates`
  content, not in `ncgo`'s generator logic.

## Approval

Design presented in chat and approved by the user (bounded-path brainstorming,
2026-09-07). No further design questions outstanding.

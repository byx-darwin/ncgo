# Design: Protect rule-center's fixed Go identifiers during export (#121)

## Problem

`internal/scaffold/template/export.go`'s Go-file export path
(`fileToTemplate` → `replaceServiceName`) has the same PascalCase/lowercase
collision risk that #120 fixed for `rule-center.proto`'s IDL export path,
but unfixed for Go source.

The rule-center preset ships
`internal/pkg/middleware/rule_center_client.go` with fixed identifiers
(`NewRuleCenterClient`, `RuleCenterConfig`, `RuleCenterClient`) and a fixed
address example in a comment (`"rule-center.internal:8888"`).

If an exporting project's `ServiceName` collides with these (e.g.
`ServiceName: "Rule"`), `replaceServiceName`'s PascalCase branch rewrites
`RuleCenterClient` → `{{.ServiceName}}CenterClient`, and its lowercase
`\b`-bounded branch rewrites `rule-center.internal` →
`{{ToLower .ServiceName}}-center.internal`, producing an exported Go
template that no longer compiles for a consumer using a different service
name.

## Investigation

- The file rule matching this file (`internal/pkg/**/*.go` in
  `KitexRules()`) does not set `LoopService`, so `templatePath()` never
  parameterizes this file's export path — it stays a literal path
  regardless of `ServiceName`. This differs from the IDL case (#120),
  where the exported *filename* also needed protecting via
  `idlTemplatePath`.
- Every string in `rule_center_client.go` that `replaceServiceName` could
  touch (`RuleCenterClient`, `RuleCenterConfig`, `NewRuleCenterClient*`,
  the `"rule-center.internal:8888"` comment) belongs to this one file, and
  none of it is meant to vary with the exporting project's `ServiceName`.

Given both points, **whole-file exclusion** is sufficient — there is no
sub-content in this file that legitimately needs `ServiceName`
templating, so there's no need for a finer-grained (identifier/substring)
protection mechanism.

## Design

Mirror #120's `fixedContractIDLs` pattern, scoped to `fileToTemplate`'s
Go-file path instead of `exportIDLs`' IDL path:

- Add `fixedContractGoFiles []string` in `export.go`, listing paths
  relative to the project root:
  `[]string{"internal/pkg/middleware/rule_center_client.go"}`
- Add `isFixedContractGoFile(rel string) bool` analogous to
  `isFixedContractIDL`.
- In `fileToTemplate`, after the `{{.Module}}` substitution and before the
  `replaceServiceName` call, check `isFixedContractGoFile(relPath)`; if
  true, skip `replaceServiceName` entirely (still run
  `escapeNonTemplateBraces` and the normal `templatePath` computation,
  both unaffected here since this file isn't `LoopService`).

## Testing

- New regression test in `export_test.go`, mirroring
  `TestExport_IDL_FixedContractNotRenamed`: export
  `internal/pkg/middleware/rule_center_client.go` with a colliding
  `ServiceName: "Rule"`, assert `RuleCenterClient`, `NewRuleCenterClient`,
  and `"rule-center.internal:8888"` survive unchanged, and `{{.Module}}`
  is still substituted correctly.
- Re-run existing `TestExport_IDL_FixedContractNotRenamed` and the
  Makefile fixed-contract test (#120) to confirm no interference.
- Verify `internal/pkg/**/*.go` files outside the fixed list still get
  normal `replaceServiceName` treatment (existing tests should already
  cover this; add one if missing).

## Out of scope

- Any change to `fixedContractIDLs` or the IDL export path (#120's
  mechanism is untouched).
- Any change to #119's `\b`-boundary fix in `replaceServiceName`'s
  lowercase branch (still used for all non-fixed files).

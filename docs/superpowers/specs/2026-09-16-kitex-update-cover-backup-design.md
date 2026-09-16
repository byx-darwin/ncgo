# Design: prevent silent loss of hand-edits on `make update` for kitex `cover`-type files

Issue: #123
Workflow: wf-2026-09-16-001

## Problem

`make update` in a generated kitex service invokes the vendored `kitex`
binary directly:

```
kitex -module $(MODULE) -template-dir template/kitex-template -type protobuf $(IDL_FILE)
```

This call bypasses ncgo entirely — ncgo has no runtime hook into it. Each
`template/kitex-template/*.yaml` fragment declares `update_behavior: cover`
or `update_behavior: skip`. `cover` means "unconditionally rewrite the file
to the fragment's rendered body on every `make update`". Several of ncgo's
shipped `cover`-type files are, in practice, files users are expected to
keep extending by hand (e.g. adding a new business error constant to
`internal/pkg/rpcerror/rpcerror.go`). Every `make update` silently discards
those hand-added lines, with no warning, no backup, and no way to recover
the change short of `git diff`/`git stash` discipline the tooling doesn't
enforce.

This is a repeat of the class of bug fixed for
`ratelimit_usecase.yaml` in issue #117 (skip vs cover for hand-edited
files), but issue #123 additionally asks for a *general* safety net: even
for files legitimately left as `cover`, a hand-edit should never be lost
without at least a warning and a way to recover it.

Note: several files listed in the original issue report
(`internal/infrastructure/auth/password.go`, `internal/infrastructure/token/redis.go`,
casbin adapter/config) do not exist in ncgo's shipped
`internal/assets/_data/kitex/kitex-template/*.yaml` set at all — they come
from customizations in the reporter's downstream repo and are out of scope
for ncgo itself. Only `main.go`, `internal/base/conf/conf.go`, and
`internal/pkg/rpcerror/rpcerror.go` correspond to real ncgo-shipped
templates.

## Constraints

- `make update` calls the vendored `kitex` binary directly; ncgo's own Go
  code does not run at that point. Any fix must live in the Makefile
  template (or other files consumed by `kitex`/shell at update time), not
  in `internal/scaffold/**` runtime logic.
- No existing baseline/hash-tracking infrastructure exists in
  `internal/manifest/` — building one is out of scope (rejected as
  over-engineering for this issue; see Alternatives).
- The fix must not introduce a new dependency on the `ncgo` binary being
  installed/on PATH in the generated project's build environment (CI
  pipelines that only have `kitex`/`go`/`sqlc` etc. must keep working).

## Chosen approach: unconditional backup before `cover` overwrite

Modify the `update:` target in
`internal/assets/_data/kitex/kitex-template/makefile.yaml` so that, before
invoking `kitex`:

1. Scan `template/kitex-template/*.yaml` (the same directory `kitex` itself
   reads) for fragments whose `update_behavior.type` is `cover`.
2. For each such fragment, extract its `path:` field.
3. If that path exists in the working tree, copy it (preserving directory
   structure) into `.ncgo-backup/<timestamp>/<path>`.
4. Run `kitex` as before.
5. Print a one-line summary: `Backed up N cover-managed file(s) to
   .ncgo-backup/<timestamp>/ before regeneration — diff them if you made
   manual edits.`

This is pure POSIX shell (`awk`, `mkdir -p`, `cp`) embedded in the Makefile
body — no new ncgo CLI surface, no state file, no dependency on `ncgo`
being present at update time. Because the file list is discovered by
scanning the yaml directory at `make update` runtime, it automatically
adapts to whichever preset/optional files are actually present in a given
project (rule-center preset, database add-on, etc.) without needing a
precomputed per-project list baked in at scaffold time.

Backups are unconditional (no diff/hash check against a baseline) — this
is a deliberate simplification: it trades "quiet unless something is
lost" for "no new state to maintain, no false negatives". A user who made
no hand-edits pays a near-zero cost (a few small file copies); a user who
did gets a guaranteed recovery path.

## Bundled narrow fix

`internal/assets/_data/kitex/kitex-template/rpcerror.yaml`'s
`update_behavior` changes from `cover` to `skip`. `rpcerror.go` is a
genuine hand-extension point (business error codes/functions are added
over a service's lifetime) with no merge mechanism, exactly matching the
precedent set by issue #117's fix to `ratelimit_usecase.yaml`. `skip` is
strictly better here than relying on backup+manual restore.

`main.go` and `internal/base/conf/conf.go` stay `cover`:
- `main.go`'s template comment already documents that wiring belongs in
  `internal/base/server`, i.e. it is intentionally not a hand-edit target.
- `conf.go` is regenerated via the framework's config-merge mechanism when
  infra add-ons are added (see `internal/scaffold/framework`), so wholesale
  regeneration is expected there, not a bug.

## Alternatives considered

1. **Baseline hash + smart warning** — record a content hash of every
   `cover`-type file at scaffold time, compare before each `make update`,
   warn only on drift. More precise (no backups when nothing changed) but
   requires new persistent state (`.ncgo/generated-baseline.json` or
   similar), a maintenance story for keeping baselines in sync across
   scaffold/add-on operations, and materially more code. Rejected as
   disproportionate to the issue for now; can be revisited if the
   unconditional-backup approach proves insufficient in practice.
2. **`ncgo`-wrapped `make update`** — have the Makefile call an `ncgo`
   subcommand instead of `kitex` directly, which performs baseline
   diffing/backup itself. Rejected: introduces a hard runtime dependency on
   `ncgo` being installed wherever `make update`/CI runs, which none of the
   generated project's other targets require today.
3. **Audit-only (flip more files to `skip`)** — narrower fix limited to
   `rpcerror.yaml`. Rejected as the sole fix per user request: doesn't
   address the general "any cover file can silently lose hand-edits"
   concern the issue raises for the class of problem, only the one
   instance found in ncgo's current template set.

## Testing

- Extend `internal/scaffold/mono/kitex_template_update_behavior_test.go`
  (or a sibling test file) with a functional test: write a temp directory
  containing sample `template/kitex-template/*.yaml` fragments (mix of
  `cover`/`skip`) and sample target files, extract the backup shell
  snippet from the rendered Makefile body, execute it via `sh -c`, and
  assert only `cover`-type files got backed up under `.ncgo-backup/`.
- The existing `TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior`
  marker-based test continues to pass unchanged; `rpcerror.yaml` does not
  use the `"Edit business logic here"` marker so it isn't currently
  covered by that test — no change needed there, but worth noting it's a
  narrower net than the new functional test.
- Golden fixtures under `internal/scaffold/mono/testdata/**` embedding the
  Makefile body must be regenerated with `-update-golden`.

## Docs

Update `README.md` / `README.zh-CN.md` and `docs/examples.md` /
`docs/examples.zh-CN.md` wherever `make update` is documented, to mention
the new backup behavior and the `.ncgo-backup/` directory.

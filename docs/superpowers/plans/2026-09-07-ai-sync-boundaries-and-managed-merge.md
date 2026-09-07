# ai sync: 边界表格失真 + managed 文件全量覆盖丢失手工内容 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two independent `ncgo ai sync` bugs from Issue #114: (1) the edit-boundaries table renders purely from `manifest.Domains` and goes stale when a domain is removed from the manifest but its directory still exists on disk; (2) `ai sync` full-overwrites `<!-- ncgo:managed -->` files, silently destroying hand-authored content with no preservation mechanism.

**Architecture:** Bug 1 adds a filesystem-existence union check to `EditBoundaries` at service scope only (workspace scope already renders per-service, not per-domain, and is untouched). Bug 2 adds a new `internal/ai/anchors.go` module implementing `<!-- ncgo:custom:<name>:start/end -->` markers; `writeTarget` and `writeStandaloneDocs` call it to preserve well-formed anchors from the previous file version, appending them under a new `## Custom Notes` section, and refuse to overwrite (same semantics as the existing "missing marker" skip) when a malformed anchor is detected.

**Tech Stack:** Go (stdlib only: `os`, `path/filepath`, `regexp`, `sort`, `strings`, `fmt`).

**Spec:** `docs/superpowers/specs/2026-09-07-ai-sync-boundaries-and-managed-merge-design.md`

## Global Constraints

- `internal/ai` is a contract-sensitive surface (AI context generation) per project `CLAUDE.md` / `.claude/rules/go.md` — keep diffs minimal, preserve existing error wording and skip semantics.
- Bug 1's filesystem-union check applies to **service scope only**; workspace-scope `EditBoundaries` behavior is unchanged (confirmed during planning: it already renders per-service-name, not per-domain, so a domain-directory existence check does not apply there).
- No new external dependencies.
- Every new/changed logic branch needs a unit test (per `.claude/rules/agent-engineering.md` §5.1).
- Final validation: `go build ./... && go build . && go vet ./... && go test ./... -count=1` (run `./scripts/smoke.sh` only if the final task's targeted checks leave doubt about CLI-level behavior).

---

### Task 1: `mergeCustomAnchors` — extract and merge named anchors

**Files:**
- Create: `internal/ai/anchors.go`
- Test: `internal/ai/anchors_test.go`

**Interfaces:**
- Produces: `func mergeCustomAnchors(oldContent, rendered []byte) (merged string, malformed []string)` — used by Task 2 and Task 3.
  - `merged`: `rendered` unchanged (as a string) when there's nothing to preserve or `len(malformed) == 0` is false; when `malformed` is non-empty, `merged` is `""` and the caller must NOT use it.
  - `malformed`: human-readable reasons; non-empty means "refuse to overwrite, same as unmanaged-file skip".

- [ ] **Step 1: Write the failing tests**

```go
package ai

import (
	"strings"
	"testing"
)

func TestMergeCustomAnchorsNoAnchors(t *testing.T) {
	old := "<!-- ncgo:managed -->\nold body, no anchors\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	merged, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) != 0 {
		t.Fatalf("expected no malformed anchors, got %v", malformed)
	}
	if merged != rendered {
		t.Errorf("expected rendered content unchanged, got %q", merged)
	}
}

func TestMergeCustomAnchorsSingleAnchor(t *testing.T) {
	old := "<!-- ncgo:managed -->\n" +
		"## Local Docker Development\n\n" +
		"<!-- ncgo:custom:docker-dev:start -->\n" +
		"docker compose up -d\n" +
		"<!-- ncgo:custom:docker-dev:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	merged, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) != 0 {
		t.Fatalf("expected no malformed anchors, got %v", malformed)
	}
	if !strings.Contains(merged, "## Custom Notes") {
		t.Errorf("expected Custom Notes section, got %q", merged)
	}
	if !strings.Contains(merged, "docker compose up -d") {
		t.Errorf("expected preserved anchor body, got %q", merged)
	}
	if !strings.Contains(merged, "<!-- ncgo:custom:docker-dev:start -->") ||
		!strings.Contains(merged, "<!-- ncgo:custom:docker-dev:end -->") {
		t.Errorf("expected preserved anchor markers, got %q", merged)
	}
}

func TestMergeCustomAnchorsMultipleNamedAnchorsPreserveOrder(t *testing.T) {
	old := "<!-- ncgo:managed -->\n" +
		"<!-- ncgo:custom:first:start -->\nfirst body\n<!-- ncgo:custom:first:end -->\n" +
		"<!-- ncgo:custom:second:start -->\nsecond body\n<!-- ncgo:custom:second:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	merged, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) != 0 {
		t.Fatalf("expected no malformed anchors, got %v", malformed)
	}
	firstIdx := strings.Index(merged, "first body")
	secondIdx := strings.Index(merged, "second body")
	if firstIdx == -1 || secondIdx == -1 || firstIdx > secondIdx {
		t.Errorf("expected first before second in merged output, got %q", merged)
	}
}

func TestMergeCustomAnchorsMissingEndIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n<!-- ncgo:custom:oops:start -->\nunterminated\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for missing end marker")
	}
	if !strings.Contains(malformed[0], "oops") {
		t.Errorf("expected malformed reason to name the anchor, got %v", malformed)
	}
}

func TestMergeCustomAnchorsEndWithoutStartIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n<!-- ncgo:custom:oops:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for end without start")
	}
}

func TestMergeCustomAnchorsDuplicateNameIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n" +
		"<!-- ncgo:custom:dup:start -->\na\n<!-- ncgo:custom:dup:end -->\n" +
		"<!-- ncgo:custom:dup:start -->\nb\n<!-- ncgo:custom:dup:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for duplicate name")
	}
}

func TestMergeCustomAnchorsInvalidNameIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n<!-- ncgo:custom:Bad_Name:start -->\nx\n<!-- ncgo:custom:Bad_Name:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for invalid name")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ai/... -run TestMergeCustomAnchors -v`
Expected: FAIL with `undefined: mergeCustomAnchors` (compile error — `anchors.go` doesn't exist yet).

- [ ] **Step 3: Write the implementation**

```go
package ai

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	customAnchorNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	customAnchorLineRE = regexp.MustCompile(`^<!-- ncgo:custom:([a-z0-9-]*):(start|end) -->$`)
)

type customAnchor struct {
	name  string
	block string // full text including its start/end marker lines
}

// mergeCustomAnchors extracts well-formed `<!-- ncgo:custom:<name>:start -->`
// ... `<!-- ncgo:custom:<name>:end -->` blocks from oldContent and appends
// them, in first-seen order, under a new "## Custom Notes" section at the
// end of rendered. When at least one malformed anchor is found (missing end
// marker, end without a matching start, invalid name, duplicate name),
// merged is "" and malformed carries a human-readable reason per problem —
// callers must refuse to overwrite rather than merge, mirroring the
// existing "file exists without ncgo:managed marker" skip semantics.
func mergeCustomAnchors(oldContent, rendered []byte) (merged string, malformed []string) {
	anchors, malformed := extractCustomAnchors(string(oldContent))
	if len(malformed) > 0 {
		return "", malformed
	}
	if len(anchors) == 0 {
		return string(rendered), nil
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(string(rendered), "\n"))
	b.WriteString("\n\n## Custom Notes\n\n")
	for i, a := range anchors {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(a.block)
	}
	b.WriteString("\n")
	return b.String(), nil
}

// extractCustomAnchors scans content line by line for ncgo:custom marker
// pairs, returning them in first-seen order. malformed reports an
// unterminated start, an end without a matching start, an invalid name, or
// a name reused across more than one anchor.
func extractCustomAnchors(content string) (anchors []customAnchor, malformed []string) {
	lines := strings.Split(content, "\n")
	seen := map[string]bool{}
	openName := ""
	openStart := 0
	for i, line := range lines {
		m := customAnchorLineRE.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		name, kind := m[1], m[2]
		if kind == "start" {
			if openName != "" {
				malformed = append(malformed, fmt.Sprintf("anchor %q starts before anchor %q ends (line %d)", name, openName, i+1))
				continue
			}
			if !customAnchorNameRE.MatchString(name) {
				malformed = append(malformed, fmt.Sprintf("anchor name %q is invalid (line %d)", name, i+1))
				continue
			}
			if seen[name] {
				malformed = append(malformed, fmt.Sprintf("anchor %q appears more than once", name))
				continue
			}
			openName, openStart = name, i
			continue
		}
		// kind == "end"
		if openName == "" || openName != name {
			malformed = append(malformed, fmt.Sprintf("anchor %q end marker has no matching start (line %d)", name, i+1))
			continue
		}
		anchors = append(anchors, customAnchor{name: name, block: strings.Join(lines[openStart:i+1], "\n")})
		seen[name] = true
		openName = ""
	}
	if openName != "" {
		malformed = append(malformed, fmt.Sprintf("anchor %q is missing its end marker", openName))
	}
	return anchors, malformed
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ai/... -run TestMergeCustomAnchors -v`
Expected: PASS (all 7 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/ai/anchors.go internal/ai/anchors_test.go
git commit -m "feat(ai): add mergeCustomAnchors for managed-file custom section preservation"
```

---

### Task 2: Wire `mergeCustomAnchors` into `writeTarget`

**Files:**
- Modify: `internal/ai/sync.go:487-521` (`writeTarget`)
- Test: `internal/ai/sync_test.go`

**Interfaces:**
- Consumes: `mergeCustomAnchors(oldContent, rendered []byte) (merged string, malformed []string)` from Task 1.
- Produces: no new exported symbols; behavior change only. `Skip.Reason` gains a new value shape: `"malformed ncgo:custom anchor(s): ...; fix markers or pass --force"`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ai/sync_test.go`:

```go
func TestSyncPreservesCustomAnchorOnManagedFile(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	pre := ManagedMarker + "\n" +
		"# stale body\n\n" +
		"<!-- ncgo:custom:docker-dev:start -->\n" +
		"docker compose up -d\n" +
		"<!-- ncgo:custom:docker-dev:end -->\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(pre), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	if _, err := Sync(Options{Root: root, Target: TargetAll}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), "## Custom Notes") || !strings.Contains(string(body), "docker compose up -d") {
		t.Errorf("expected custom anchor preserved in AGENTS.md, got %q", string(body))
	}
}

func TestSyncRefusesManagedFileWithMalformedAnchor(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	pre := ManagedMarker + "\n<!-- ncgo:custom:oops:start -->\nunterminated\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(pre), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	res, err := Sync(Options{Root: root, Target: TargetAll})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); string(got) != pre {
		t.Errorf("AGENTS.md must not be overwritten when a custom anchor is malformed")
	}
	var skipped bool
	for _, s := range res.Skipped {
		if s.Path == "AGENTS.md" && strings.Contains(s.Reason, "malformed ncgo:custom anchor") {
			skipped = true
		}
	}
	if !skipped {
		t.Errorf("expected AGENTS.md skip for malformed anchor; got %+v", res.Skipped)
	}
}

func TestSyncForceOverwritesManagedFileWithMalformedAnchor(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	pre := ManagedMarker + "\n<!-- ncgo:custom:oops:start -->\nunterminated\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(pre), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	if _, err := Sync(Options{Root: root, Force: true, Target: TargetAll}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if strings.Contains(string(body), "unterminated") {
		t.Errorf("--force should discard the malformed anchor content, got %q", string(body))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ai/... -run TestSyncPreservesCustomAnchorOnManagedFile -run TestSyncRefusesManagedFileWithMalformedAnchor -run TestSyncForceOverwritesManagedFileWithMalformedAnchor -v`
Expected: FAIL — `TestSyncPreservesCustomAnchorOnManagedFile` and `TestSyncRefusesManagedFileWithMalformedAnchor` fail (no merge/skip logic yet); `TestSyncForceOverwritesManagedFileWithMalformedAnchor` currently passes trivially since force already overwrites — it must still pass after Step 3.

- [ ] **Step 3: Implement the merge step in `writeTarget`**

Replace the body of `writeTarget` in `internal/ai/sync.go` (lines 487-521) with:

```go
func writeTarget(opts Options, t target, inputs renderInputs, res *Result) error {
	full, skip, err := safeWritePath(opts.Root, t.RelPath)
	if err != nil {
		return fmt.Errorf("ai sync: %w", err)
	}
	if skip != "" {
		res.Skipped = append(res.Skipped, Skip{Path: t.RelPath, Reason: skip})
		return nil
	}
	rendered := t.Render(inputs)
	existing, err := os.ReadFile(full)
	switch {
	case err == nil:
		if !isManaged(existing) && !opts.Force {
			res.Skipped = append(res.Skipped, Skip{
				Path:   t.RelPath,
				Reason: "exists without ncgo:managed marker; pass --force to overwrite",
			})
			return nil
		}
		if isManaged(existing) {
			merged, malformed := mergeCustomAnchors(existing, []byte(rendered))
			if len(malformed) > 0 {
				if !opts.Force {
					res.Skipped = append(res.Skipped, Skip{
						Path:   t.RelPath,
						Reason: "malformed ncgo:custom anchor(s): " + strings.Join(malformed, "; ") + "; fix markers or pass --force",
					})
					return nil
				}
			} else {
				rendered = merged
			}
		}
	case errors.Is(err, fs.ErrNotExist):
		// no existing file; nothing to merge
	default:
		return fmt.Errorf("ai sync: stat %s: %w", full, err)
	}
	if opts.DryRun {
		res.Skipped = append(res.Skipped, Skip{Path: t.RelPath, Reason: "dry-run"})
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("ai sync: mkdir %s: %w", filepath.Dir(full), err)
	}
	stamped := stampGeneratedAt(rendered, time.Now())
	if err := os.WriteFile(full, []byte(stamped), 0o644); err != nil {
		return fmt.Errorf("ai sync: write %s: %w", full, err)
	}
	res.Written = append(res.Written, t.RelPath)
	return nil
}
```

This preserves the original control flow exactly (same skip messages, same ordering of the `DryRun` check relative to the managed-marker check) and only adds the anchor-merge branch when the existing file is managed. `strings` is already imported by `sync.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ai/... -run TestSync -v`
Expected: PASS for all `TestSync*` tests, including the three new ones and all pre-existing ones (`TestSyncRefusesUnmanagedFile`, `TestSyncForceOverwritesUnmanagedFile`, `TestSyncAppendsLocalNotes`, etc.).

- [ ] **Step 5: Commit**

```bash
git add internal/ai/sync.go internal/ai/sync_test.go
git commit -m "fix(ai): writeTarget preserves ncgo:custom anchors instead of full-overwriting managed files"
```

---

### Task 3: Wire `mergeCustomAnchors` into `writeStandaloneDocs`

**Files:**
- Modify: `internal/ai/sync.go:529-572` (`writeStandaloneDocs`)
- Test: `internal/ai/sync_test.go`

**Interfaces:**
- Consumes: `mergeCustomAnchors` from Task 1 (already wired into `writeTarget` in Task 2).
- Produces: no new exported symbols.

- [ ] **Step 1: Write the failing test**

Add to `internal/ai/sync_test.go`:

```go
func TestSyncPreservesCustomAnchorOnStandaloneDoc(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	docPath := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.en.md")
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pre := ManagedMarker + "\n# stale\n\n" +
		"<!-- ncgo:custom:team-notes:start -->\n" +
		"see runbook at go/team-runbook\n" +
		"<!-- ncgo:custom:team-notes:end -->\n"
	if err := os.WriteFile(docPath, []byte(pre), 0o644); err != nil {
		t.Fatalf("seed standalone doc: %v", err)
	}
	if _, err := Sync(Options{Root: root, Target: TargetAll}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(docPath)
	if !strings.Contains(string(body), "## Custom Notes") || !strings.Contains(string(body), "go/team-runbook") {
		t.Errorf("expected custom anchor preserved in standalone doc, got %q", string(body))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/... -run TestSyncPreservesCustomAnchorOnStandaloneDoc -v`
Expected: FAIL — standalone docs are still fully overwritten.

- [ ] **Step 3: Implement the merge step in `writeStandaloneDocs`**

Replace the body of `writeStandaloneDocs` in `internal/ai/sync.go` (lines 529-572, i.e. from `func writeStandaloneDocs` through its closing `}` before `listDocSpecs`) with:

```go
// writeStandaloneDocs generates standalone design-doc files to docs/ncgo/
// in the user project, with cross-link rewriting.
func writeStandaloneDocs(opts Options, res *Result, profile string) error {
	for _, spec := range listDocSpecs(profile, opts.Lang) {
		full, skip, err := safeWritePath(opts.Root, spec.RelPath)
		if err != nil {
			return fmt.Errorf("ai sync: %w", err)
		}
		if skip != "" {
			res.Skipped = append(res.Skipped, Skip{Path: spec.RelPath, Reason: skip})
			continue
		}
		if opts.DryRun {
			res.Skipped = append(res.Skipped, Skip{Path: spec.RelPath, Reason: "dry-run"})
			continue
		}
		b, err := fs.ReadFile(assets.FS(), spec.AssetPath)
		if err != nil {
			return fmt.Errorf("ai sync: read embedded %s: %w", spec.AssetPath, err)
		}
		// Mark the materialized doc as managed so a later sync refreshes it
		// instead of treating it as a user-owned file without the marker.
		content := ManagedMarker + "\n" + rewriteDocLinks(string(b))
		existing, err := os.ReadFile(full)
		switch {
		case err == nil:
			if !isManaged(existing) && !opts.Force {
				res.Skipped = append(res.Skipped, Skip{
					Path:   spec.RelPath,
					Reason: "exists without ncgo:managed marker; pass --force to overwrite",
				})
				continue
			}
			if isManaged(existing) {
				merged, malformed := mergeCustomAnchors(existing, []byte(content))
				if len(malformed) > 0 {
					if !opts.Force {
						res.Skipped = append(res.Skipped, Skip{
							Path:   spec.RelPath,
							Reason: "malformed ncgo:custom anchor(s): " + strings.Join(malformed, "; ") + "; fix markers or pass --force",
						})
						continue
					}
				} else {
					content = merged
				}
			}
		case errors.Is(err, fs.ErrNotExist):
			// no existing file; nothing to merge
		default:
			return fmt.Errorf("ai sync: check %s: %w", spec.RelPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("ai sync: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return fmt.Errorf("ai sync: write %s: %w", full, err)
		}
		res.Written = append(res.Written, spec.RelPath)
	}
	return nil
}
```

This preserves the original ordering exactly (skip / dry-run checks still happen before any file reads) and only adds the merge branch inside the existing-file case.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ai/... -run TestSync -v`
Expected: PASS for all `TestSync*` tests, including `TestSyncPreservesCustomAnchorOnStandaloneDoc` and pre-existing standalone-doc tests (`TestSyncWritesStandaloneDocs`, `TestSyncRefreshesStandaloneDocsOnSecondPass`, `TestSyncDryRunWritesNoStandaloneDocs`).

- [ ] **Step 5: Commit**

```bash
git add internal/ai/sync.go internal/ai/sync_test.go
git commit -m "fix(ai): writeStandaloneDocs preserves ncgo:custom anchors instead of full-overwriting managed docs"
```

---

### Task 4: `EditBoundaries` filesystem-union check (Bug 1, service scope only)

**Files:**
- Modify: `internal/ai/boundaries.go`
- Modify: `internal/ai/sync.go:171` (caller)
- Test: `internal/ai/boundaries_test.go`

**Interfaces:**
- Produces: `func EditBoundaries(source syncSource, root string) (mayEdit, doNotEdit []boundaryEntry)` — signature change from the current `EditBoundaries(source syncSource)`. `root` is used only when `source.Scope == syncScopeService`; ignored for `syncScopeWorkspace`.

- [ ] **Step 1: Update existing call sites to the new signature (so the package still compiles for the next step's failing tests)**

In `internal/ai/boundaries_test.go`, change both existing calls:

```go
mayEdit, doNotEdit := EditBoundaries(source)
```

to:

```go
mayEdit, doNotEdit := EditBoundaries(source, "")
```

(both `TestEditBoundariesMonoService` and `TestEditBoundariesWorkspace` — passing `""` preserves their current behavior exactly, since `domainsOnDiskNotInManifest` treats an empty root as "no filesystem check").

In `internal/ai/sync.go:171`, change:

```go
inputs.EditBoundaries = RenderBoundaries(EditBoundaries(source))
```

to:

```go
inputs.EditBoundaries = RenderBoundaries(EditBoundaries(source, opts.Root))
```

- [ ] **Step 2: Write the new failing tests**

Add to `internal/ai/boundaries_test.go`:

```go
func TestEditBoundariesUnionsDomainStillOnDiskButNotInManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "repository", "role"), 0o755); err != nil {
		t.Fatalf("mkdir repository/role: %v", err)
	}
	source := syncSource{
		Scope: syncScopeService,
		Service: &manifest.Manifest{
			Domains: []string{"device"}, // "role" was removed from the manifest
		},
	}
	mayEdit, _ := EditBoundaries(source, root)
	var found *boundaryEntry
	for i := range mayEdit {
		if mayEdit[i].Path == "internal/repository/role/" {
			found = &mayEdit[i]
		}
	}
	if found == nil {
		t.Fatalf("expected internal/repository/role/ row despite manifest removal; got %+v", mayEdit)
	}
	if !strings.Contains(found.Reason, "not in manifest") {
		t.Errorf("expected Reason to flag the row as not in manifest, got %q", found.Reason)
	}
}

func TestEditBoundariesDoesNotDuplicateManifestDomains(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "repository", "device"), 0o755); err != nil {
		t.Fatalf("mkdir repository/device: %v", err)
	}
	source := syncSource{
		Scope: syncScopeService,
		Service: &manifest.Manifest{
			Domains: []string{"device"},
		},
	}
	mayEdit, _ := EditBoundaries(source, root)
	count := 0
	for _, e := range mayEdit {
		if e.Path == "internal/repository/device/" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one internal/repository/device/ row, got %d in %+v", count, mayEdit)
	}
}

func TestEditBoundariesWorkspaceScopeIgnoresFilesystem(t *testing.T) {
	root := t.TempDir() // deliberately empty — no internal/repository dirs at all
	source := syncSource{
		Scope: syncScopeWorkspace,
		WorkspaceServices: []workspaceServiceFacts{
			{Name: "user-rpc", Kind: "kitex", Dir: "services/user-rpc"},
		},
	}
	mayEdit, _ := EditBoundaries(source, root)
	var found bool
	for _, e := range mayEdit {
		if e.Path == "internal/usecase/user-rpc/" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected workspace-scope behavior unchanged (service-name-based row), got %+v", mayEdit)
	}
}
```

Add `"os"` and `"path/filepath"` to the imports in `internal/ai/boundaries_test.go`.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/ai/... -run TestEditBoundaries -v`
Expected: FAIL to compile — `EditBoundaries` still takes one argument.

- [ ] **Step 4: Implement the union check in `boundaries.go`**

Replace the full contents of `internal/ai/boundaries.go` with:

```go
package ai

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// boundaryEntry describes one file path in the edit-boundaries table.
type boundaryEntry struct {
	Path   string
	Reason string
}

// EditBoundaries returns the may-edit and do-not-edit file tables for a
// sync source. The may-edit list is derived from the manifest domains; for
// service scope it is unioned with domain directories that still exist on
// disk under internal/repository/ and internal/usecase/ even though they
// were removed from the manifest, so the table doesn't silently drop a row
// for a directory a hand-written caller still depends on. The do-not-edit
// list is static plus generated method-stub markers per rendered domain.
//
// root is the filesystem root to check for service scope (pass opts.Root).
// It is ignored for workspace scope, which renders one row per workspace
// service name rather than per domain and has no single root against which
// to check domain directories.
func EditBoundaries(source syncSource, root string) (mayEdit, doNotEdit []boundaryEntry) {
	mayEdit = []boundaryEntry{
		{Path: "internal/handler/", Reason: "HTTP/RPC handlers"},
		{Path: "internal/adapter/", Reason: "Outbound RPC clients"},
		{Path: "internal/pkg/middleware/", Reason: "Custom middleware"},
		{Path: "internal/base/server/server.go", Reason: "DI wiring"},
	}
	doNotEdit = []boundaryEntry{
		{Path: "internal/base/data/", Reason: "Generated data layer"},
		{Path: "internal/db/gen/", Reason: "sqlc-generated code"},
		{Path: "internal/router/register.go", Reason: "Generated routes"},
		{Path: "internal/pb/", Reason: "Protobuf generated types"},
		{Path: "CLAUDE.md, AGENTS.md", Reason: "Managed by ncgo ai sync"},
	}
	var domains []string
	var extra []string
	switch source.Scope {
	case syncScopeService:
		domains = source.Service.Domains
		extra = domainsOnDiskNotInManifest(root, domains)
	case syncScopeWorkspace:
		for _, svc := range source.WorkspaceServices {
			domains = append(domains, svc.Name)
		}
	}
	for _, d := range domains {
		mayEdit = append(mayEdit,
			boundaryEntry{Path: "internal/usecase/" + d + "/", Reason: "Business logic"},
			boundaryEntry{Path: "internal/repository/" + d + "/", Reason: "Data access implementation"},
		)
		doNotEdit = append(doNotEdit,
			boundaryEntry{Path: "internal/usecase/" + d + "/" + d + ".go between anchors", Reason: "Generated method stubs"},
		)
	}
	for _, d := range extra {
		mayEdit = append(mayEdit,
			boundaryEntry{Path: "internal/usecase/" + d + "/", Reason: "Business logic (not in manifest; verify manual usage)"},
			boundaryEntry{Path: "internal/repository/" + d + "/", Reason: "Data access implementation (not in manifest; verify manual usage)"},
		)
	}
	return mayEdit, doNotEdit
}

// domainsOnDiskNotInManifest returns domain names, sorted, that have an
// existing internal/repository/<name>/ or internal/usecase/<name>/
// directory under root but are absent from manifestDomains. Returns nil
// when root is empty (e.g. tests constructing a syncSource without a real
// filesystem root) or when neither directory exists/can be read.
func domainsOnDiskNotInManifest(root string, manifestDomains []string) []string {
	if root == "" {
		return nil
	}
	known := make(map[string]bool, len(manifestDomains))
	for _, d := range manifestDomains {
		known[d] = true
	}
	found := make(map[string]bool)
	var extra []string
	for _, sub := range []string{"internal/repository", "internal/usecase"} {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(sub)))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || known[e.Name()] || found[e.Name()] {
				continue
			}
			found[e.Name()] = true
			extra = append(extra, e.Name())
		}
	}
	sort.Strings(extra)
	return extra
}

// RenderBoundaries renders the may-edit / do-not-edit tables as markdown.
func RenderBoundaries(mayEdit, doNotEdit []boundaryEntry) string {
	var b strings.Builder
	b.WriteString("## Boundaries\n\n")
	b.WriteString("### You may edit\n\n")
	b.WriteString("| Path | Purpose |\n")
	b.WriteString("|------|---------|\n")
	for _, e := range mayEdit {
		b.WriteString("| `" + e.Path + "` | " + e.Reason + " |\n")
	}
	b.WriteString("\n### Do not edit\n\n")
	b.WriteString("| Path | Why |\n")
	b.WriteString("|------|-----|\n")
	for _, e := range doNotEdit {
		b.WriteString("| `" + e.Path + "` | " + e.Reason + " |\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ai/... -run TestEditBoundaries -v`
Expected: PASS for all `TestEditBoundaries*` tests, including the 3 new ones and the 2 pre-existing ones.

- [ ] **Step 6: Commit**

```bash
git add internal/ai/boundaries.go internal/ai/boundaries_test.go internal/ai/sync.go
git commit -m "fix(ai): EditBoundaries unions manifest domains with on-disk directories at service scope"
```

---

### Task 5: Full package validation + golden-test impact check

**Files:** none (validation only)

**Interfaces:** none — this task only runs checks.

- [ ] **Step 1: Run the full `internal/ai` package test suite**

Run: `go test ./internal/ai/... -count=1 -v`
Expected: PASS, all tests including Tasks 1-4's additions and every pre-existing test (`TestSyncWritesAllTargets`, `TestSyncRefusesUnmanagedFile`, `TestSyncAppendsLocalNotes`, `TestEditBoundariesMonoService`, etc.).

- [ ] **Step 2: Check whether any golden fixture depends on `ai sync` output**

Run: `grep -rl "ai sync\|EditBoundaries\|ncgo:managed" internal/scaffold/mono/testdata/ 2>/dev/null; echo "exit=$?"`
Expected: no matches (`ncgo new`/scaffold golden tests exercise the initial scaffold generation path, not `ai sync`, which only runs against already-generated projects). If this unexpectedly finds matches, stop and re-scope this task — do not blindly regenerate golden fixtures.

- [ ] **Step 3: Run any CLI/MCP integration tests that touch `ai sync`**

Run: `go test ./internal/cli/... -run TestAI -v && go test ./internal/mcp/... -run TestAI -v`
Expected: PASS. If either package has no matching tests, that's fine (`go test` reports `no tests to run`, not a failure) — but read `internal/cli/ai_test.go` and `internal/mcp/tool_ai.go`'s test file first to confirm the `-run` pattern actually matches something; adjust the pattern if the real test names differ.

- [ ] **Step 4: Full repository validation**

Run: `go build ./... && go build . && go vet ./... && go test ./... -count=1`
Expected: all PASS, no new failures introduced outside `internal/ai`.

- [ ] **Step 5: Commit (only if Steps 1-4 required any fixes)**

```bash
git add -A
git commit -m "test(ai): confirm full validation after boundaries + anchor-merge fixes"
```

If no fixes were needed, skip this commit — there's nothing to commit.

---

### Task 6: Documentation + CHANGELOG

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `CHANGELOG.md`

**Interfaces:** none — docs only.

- [ ] **Step 1: Update `README.md`**

Find this paragraph (currently at `README.md:449-452`):

```markdown
Files contain `<!-- ncgo:managed -->`; existing files without the marker are
skipped unless `--force` is passed. Add project-specific notes in
`AGENTS.local.md`; they are appended to the long-form generated context files,
while `.claude/generated/project-context.md` stays deterministic.
```

Replace it with:

```markdown
Files contain `<!-- ncgo:managed -->`; existing files without the marker are
skipped unless `--force` is passed. Add project-specific notes in
`AGENTS.local.md`; they are appended to the long-form generated context files,
while `.claude/generated/project-context.md` stays deterministic.

For custom content placed directly inside a managed file (rather than in
`AGENTS.local.md`), wrap it in `<!-- ncgo:custom:<name>:start -->` /
`<!-- ncgo:custom:<name>:end -->` markers (`<name>` matches
`^[a-z][a-z0-9-]{0,62}$`). On the next `ai sync`, well-formed anchors are
preserved and re-appended under a `## Custom Notes` section; content left
outside any anchor is discarded on every sync, same as before. A malformed
anchor (missing end marker, duplicate name, invalid name) makes `ai sync`
refuse to overwrite that file — same as a missing `ncgo:managed` marker —
until you fix the anchor or pass `--force`.

> **Migration note:** content added directly to a managed file *before*
> this anchor mechanism existed, and not wrapped in `ncgo:custom` markers,
> is still overwritten once on the first `ai sync` after upgrading — there's
> no way to retroactively tell which pre-existing content was intentional.
> Wrap what you want to keep in `ncgo:custom` markers before that first sync.
```

- [ ] **Step 2: Update `README.zh-CN.md`**

Find this paragraph (currently at `README.zh-CN.md:388`):

```markdown
这些文件带有 `<!-- ncgo:managed -->` 标记。没有该标记的已有文件默认不会覆盖，除非传 `--force`。项目私有说明放在 `AGENTS.local.md`，会附加到长版上下文文件；`.claude/generated/project-context.md` 保持 deterministic。
```

Replace it with:

```markdown
这些文件带有 `<!-- ncgo:managed -->` 标记。没有该标记的已有文件默认不会覆盖，除非传 `--force`。项目私有说明放在 `AGENTS.local.md`，会附加到长版上下文文件；`.claude/generated/project-context.md` 保持 deterministic。

如果要在 managed 文件内部（而不是 `AGENTS.local.md`）直接插入自定义内容，请用 `<!-- ncgo:custom:<name>:start -->` / `<!-- ncgo:custom:<name>:end -->` 包裹（`<name>` 需匹配 `^[a-z][a-z0-9-]{0,62}$`）。下次 `ai sync` 时，格式正确的锚点内容会被保留，并重新追加到 `## Custom Notes` 小节；锚点之外的内容每次 sync 仍会被覆盖，行为与之前一致。若锚点格式有误（缺少结束标记、名字重复、名字不合法），`ai sync` 会拒绝覆盖该文件——与缺失 `ncgo:managed` 标记时的行为一致——直到你修复锚点或传入 `--force`。

> **迁移说明：** 在引入该锚点机制**之前**就已经直接写入 managed 文件、且没有用 `ncgo:custom` 包裹的自定义内容，在升级后的**第一次** `ai sync` 仍会被覆盖一次——没有办法回溯识别哪些旧内容是用户手工添加的。升级前，请先把想保留的内容包进 `ncgo:custom` 锚点，再执行 sync。
```

- [ ] **Step 3: Add a CHANGELOG entry**

In `CHANGELOG.md`, under the existing `## [Unreleased]` heading (currently empty, right after the `## [Unreleased]` line and before `## [1.0.3] - 2026-09-06`), add:

```markdown
## [Unreleased]

### Fixed

- **`ncgo ai sync`**: the edit-boundaries table (`## Boundaries` in generated CLAUDE.md/AGENTS.md) only reflected `manifest.Domains` — removing a domain from the manifest silently dropped its `internal/repository/<domain>/` and `internal/usecase/<domain>/` rows even when the directories still existed on disk (e.g. still referenced by hand-written code). Service-scope sync now unions the manifest domain list with directories actually present on disk and flags the extras as "not in manifest; verify manual usage".
- **`ncgo ai sync`**: managed files (`<!-- ncgo:managed -->`) were fully overwritten on every sync with no way to preserve hand-authored content placed directly inside them. Added `<!-- ncgo:custom:<name>:start/end -->` anchors: well-formed anchors in the previous file version are now preserved and re-appended under a `## Custom Notes` section; a malformed anchor makes sync refuse to overwrite (same as a missing managed marker) instead of silently dropping content. **One-time migration note:** pre-existing custom content not wrapped in these anchors is still overwritten once on the first sync after upgrading.
```

- [ ] **Step 4: Run markdown diagnostics**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')` (confirms no stray Go changes crept in) and visually diff the three markdown files to confirm no unintended whitespace/heading changes: `git diff README.md README.zh-CN.md CHANGELOG.md`.
Expected: `gofmt -l` prints nothing; the diff shows only the intended additions.

- [ ] **Step 5: Commit**

```bash
git add README.md README.zh-CN.md CHANGELOG.md
git commit -m "docs: document ncgo:custom anchors for managed-file content preservation (#114)"
```

---

## Post-Plan Summary (for the Delivery phase)

After all 6 tasks are committed:
- Run `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh` once more as the final PR-quality gate.
- The PR body must include `Closes #114`.
- Both bugs are independent; if a reviewer wants them split, Tasks 1-3 (anchors/Bug 2) and Task 4 (boundaries/Bug 1) can be cherry-picked into separate PRs — but per the design doc, delivering them together in one PR is acceptable since they share the same Issue.

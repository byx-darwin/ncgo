# Generate Standalone Reference Docs to User Projects

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `ncgo ai sync` generates standalone design-doc files to `docs/ncgo/` in user projects, with cross-link rewriting, so developers and AI agents can browse full documentation locally.

**Architecture:** A new `writeStandaloneDocs()` function in `internal/ai/sync.go` reads embedded assets, rewrites internal doc links to local relative paths, and writes them to `docs/ncgo/<profile>/<filename>.md`. Called after the existing target-writing loop in `Sync()`. Cross-profile design-docs (hertz↔kitex↔micro) are also generated so all links resolve.

**Tech Stack:** Go, `io/fs`, embedded assets (`internal/assets/_data/`), link rewriting via `strings.ReplaceAll`.

---

## File Structure

### Files to create:
- None (all logic in existing files)

### Files to modify:
- **`internal/ai/sync.go`** — add `writeStandaloneDocs()` function, `writeDocFile()` helper, call site in `Sync()`
- **`internal/ai/sync_test.go`** — add tests for standalone doc generation, link rewriting, dry-run compliance

### No changes needed:
- `internal/ai/render.go` — existing targets unchanged
- `internal/cli/ai.go` — CLI flags unchanged
- `internal/assets/_data/` — embedded source files unchanged

---

### Task 1: Implement writeStandaloneDocs in sync.go

**Files:**
- Modify: `internal/ai/sync.go`

- [ ] **Step 1: Write the failing test**

Add this test to `internal/ai/sync_test.go`:

```go
func TestSyncWritesStandaloneDocs(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	// Check docs/ncgo/hertz/design-doc.en.md exists
	p := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.en.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	body := string(b)
	// Must contain design-doc content
	if !strings.Contains(body, "Hertz Template Design Doc") {
		t.Errorf("standalone design-doc missing title; got %d bytes", len(body))
	}
	// Links must be rewritten to local relative paths
	if strings.Contains(body, "docs/hertz/") || strings.Contains(body, "../hertz/") {
		t.Errorf("standalone design-doc still contains original doc links, not rewritten")
	}
	// Must include cross-profile docs (kitex) for link resolution
	kp := filepath.Join(root, "docs", "ncgo", "kitex", "design-doc.en.md")
	if _, err := os.Stat(kp); os.IsNotExist(err) {
		t.Errorf("cross-profile kitex/design-doc.en.md not generated for hertz project")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestSyncWritesStandaloneDocs -v`
Expected: FAIL — `standalone design-doc missing title` or file not found

- [ ] **Step 3: Add the writeStandaloneDocs function to sync.go**

Add after the `readDesignDoc` function (around line 148):

```go
// writeStandaloneDocs generates standalone reference docs under docs/ncgo/
// in the user project. It reads the primary designDoc for the project's
// profile, the optional rate-limit-dynamic-design (hertz only), and the
// cross-profile design-docs referenced by internal links.
func writeStandaloneDocs(opts Options, res *Result, profile string, designDoc string) error {
	if opts.DryRun {
		return nil
	}
	// Map of source filename -> content for all docs to generate for this profile.
	docs := map[string]string{}

	// 1. Primary design-doc for this profile
	docs["design-doc."+opts.Lang+".md"] = designDoc

	// 2. rate-limit-dynamic-design (hertz profile only)
	if profile == manifest.KindHertz {
		rlEn, err := readAssetDoc("docs/hertz/rate-limit-dynamic-design."+opts.Lang+".md")
		if err != nil {
			return err
		}
		docs["rate-limit-dynamic-design."+opts.Lang+".md"] = rlEn
	}

	// 3. Cross-profile design-docs (hertz <-> kitex <-> micro)
	for _, p := range crossProfiles(profile) {
		dd, err := readDesignDoc(p, opts.Lang)
		if err != nil {
			return err
		}
		dir := filepath.Join(resolvedDocsDir(res), "docs", "ncgo", p)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("ai sync: mkdir %s: %w", dir, err)
		}
		outPath := filepath.Join(dir, "design-doc."+opts.Lang+".md")
		rewritten := rewriteDocLinks(dd, p)
		if err := os.WriteFile(outPath, []byte(rewritten), 0o644); err != nil {
			return fmt.Errorf("ai sync: write %s: %w", outPath, err)
		}
		res.Written = append(res.Written, outPath)
	}

	// Write the primary and rate-limit docs
	dir := filepath.Join(resolvedDocsDir(res), "docs", "ncgo", profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ai sync: mkdir %s: %w", dir, err)
	}
	for name, content := range docs {
		outPath := filepath.Join(dir, name)
		rewritten := rewriteDocLinks(content, profile)
		if err := os.WriteFile(outPath, []byte(rewritten), 0o644); err != nil {
			return fmt.Errorf("ai sync: write %s: %w", outPath, err)
		}
		res.Written = append(res.Written, outPath)
	}
	return nil
}
```

Wait, this approach has issues. Let me use a simpler, cleaner design:

```go
// docSpec describes one embedded doc file to generate into a user project.
type docSpec struct {
	AssetPath string // path inside embedded assets, e.g. "docs/hertz/design-doc.en.md"
	RelPath   string // relative output path from project root, e.g. "docs/ncgo/hertz/design-doc.en.md"
}

// writeStandaloneDocs generates standalone reference docs under docs/ncgo/
// in the user project so developers and AI agents can browse them locally.
func writeStandaloneDocs(opts Options, res *Result, profile string) error {
	if opts.DryRun {
		return nil
	}
	for _, spec := range listDocSpecs(profile, opts.Lang) {
		b, err := fs.ReadFile(assets.FS(), spec.AssetPath)
		if err != nil {
			return fmt.Errorf("ai sync: read embedded %s: %w", spec.AssetPath, err)
		}
		content := rewriteDocLinks(string(b), profile, opts.Lang)
		full := filepath.Join(opts.Root, spec.RelPath)
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

- [ ] **Step 4: Add listDocSpecs function**

Add after `writeStandaloneDocs`:

```go
// listDocSpecs returns all doc files to generate for a given profile and language.
// It includes the primary design-doc, rate-limit-dynamic-design (hertz only),
// and cross-profile design-docs referenced by internal links.
func listDocSpecs(profile, lang string) []docSpec {
	specs := []docSpec{
		{
			AssetPath: "docs/" + profile + "/design-doc." + lang + ".md",
			RelPath:   "docs/ncgo/" + profile + "/design-doc." + lang + ".md",
		},
	}
	if profile == manifest.KindHertz {
		specs = append(specs, docSpec{
			AssetPath: "docs/hertz/rate-limit-dynamic-design." + lang + ".md",
			RelPath:   "docs/ncgo/hertz/rate-limit-dynamic-design." + lang + ".md",
		})
	}
	for _, p := range crossProfiles(profile) {
		specs = append(specs, docSpec{
			AssetPath: "docs/" + p + "/design-doc." + lang + ".md",
			RelPath:   "docs/ncgo/" + p + "/design-doc." + lang + ".md",
		})
	}
	return specs
}
```

- [ ] **Step 5: Add crossProfiles function**

Add after `listDocSpecs`:

```go
// crossProfiles returns the set of other profiles whose design-docs are
// referenced by internal links from the given profile. This ensures all
// markdown links in docs/ncgo/ resolve to local files.
func crossProfiles(profile string) []string {
	switch profile {
	case manifest.KindHertz:
		return []string{manifest.KindKitex}
	case manifest.KindKitex:
		return []string{manifest.KindHertz}
	case manifest.ModeMicro:
		return []string{manifest.KindHertz, manifest.KindKitex}
	default:
		return nil
	}
}
```

- [ ] **Step 6: Add rewriteDocLinks function**

Add after `crossProfiles`:

```go
// rewriteDocLinks rewrites internal documentation links so they resolve to
// the local docs/ncgo/ directory structure. It handles two link patterns:
//   - Absolute-style: docs/<profile>/<file>  →  ./<profile>/<file>
//   - Relative:       ../<profile>/<file>    →  ./<profile>/<file>
//
// For rate-limit-dynamic-design links (hertz only), the target stays under
// the same profile directory: ./hertz/rate-limit-dynamic-design.<lang>.md
func rewriteDocLinks(content, profile, lang string) string {
	// Rewrite rate-limit-dynamic-design references (hertz profile)
	// Pattern: docs/hertz/rate-limit-dynamic-design.en.md or ../hertz/rate-limit-dynamic-design.en.md
	for _, origProfile := range []string{"hertz", "kitex", "micro"} {
		oldAbs := "docs/" + origProfile + "/"
		newAbs := "./" + origProfile + "/"
		content = strings.ReplaceAll(content, oldAbs, newAbs)

		oldRel := "../" + origProfile + "/"
		newRel := "./" + origProfile + "/"
		content = strings.ReplaceAll(content, oldRel, newRel)
	}
	return content
}
```

- [ ] **Step 7: Add call site in Sync()**

Modify the `Sync` function (line 136) to call `writeStandaloneDocs` after the target loop:

Change:
```go
	return res, nil
}
```
To:
```go
	profile := resolveProfile(source)
	if err := writeStandaloneDocs(opts, res, profile); err != nil {
		return res, err
	}
	return res, nil
}
```

- [ ] **Step 8: Add resolveProfile function**

Add after `crossProfiles`:

```go
// resolveProfile determines which embedded doc profile to use for a given
// sync source. This mirrors the logic in resolveSyncSource.
func resolveProfile(source syncSource) string {
	switch source.Scope {
	case syncScopeService:
		return source.Service.Service.Kind
	default:
		return manifest.ModeMicro
	}
}
```

- [ ] **Step 9: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestSyncWritesStandaloneDocs -v`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/ai/sync.go internal/ai/sync_test.go
git commit -m "feat(ai): generate standalone docs/ncgo/ in user projects via ai sync"
```

---

### Task 2: Add comprehensive doc generation tests

**Files:**
- Modify: `internal/ai/sync_test.go`

- [ ] **Step 1: Add kitex profile test**

```go
func TestSyncWritesStandaloneDocsForKitex(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindKitex)
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	// Primary kitex design-doc
	p := filepath.Join(root, "docs", "ncgo", "kitex", "design-doc.en.md")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Errorf("kitex design-doc not generated: %s", p)
	}
	// Cross-profile hertz design-doc (for links)
	hp := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.en.md")
	if _, err := os.Stat(hp); os.IsNotExist(err) {
		t.Errorf("cross-profile hertz design-doc not generated for kitex project")
	}
	// rate-limit-dynamic-design should NOT be generated for kitex
	rl := filepath.Join(root, "docs", "ncgo", "kitex", "rate-limit-dynamic-design.en.md")
	if _, err := os.Stat(rl); !os.IsNotExist(err) {
		t.Errorf("rate-limit-dynamic-design should not be generated for kitex profile")
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestSyncWritesStandaloneDocsForKitex -v`
Expected: PASS

- [ ] **Step 3: Add zh-CN doc test**

```go
func TestSyncWritesStandaloneDocsZhCN(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root, Lang: LangZhCN})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	p := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.zh-CN.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	body := string(b)
	if !strings.Contains(body, "Hertz 模板设计文档") {
		t.Errorf("zh-CN standalone doc missing Chinese title")
	}
	// Check links are rewritten
	if strings.Contains(body, "../kitex/") || strings.Contains(body, "docs/kitex/") {
		t.Errorf("zh-CN standalone doc still contains original kitex links")
	}
	if !strings.Contains(body, "./kitex/") {
		t.Errorf("zh-CN standalone doc missing rewritten kitex link")
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestSyncWritesStandaloneDocsZhCN -v`
Expected: PASS

- [ ] **Step 5: Add dry-run test for standalone docs**

```go
func TestSyncDryRunWritesNoStandaloneDocs(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Written) != 0 {
		t.Errorf("DryRun must not write; got %v", res.Written)
	}
	p := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.en.md")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("standalone doc should not exist after dry run")
	}
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestSyncDryRunWritesNoStandaloneDocs -v`
Expected: PASS

- [ ] **Step 7: Add rewriteDocLinks unit test**

```go
func TestRewriteDocLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute hertz link",
			input:    "see `docs/hertz/rate-limit-dynamic-design.en.md`",
			expected: "see `./hertz/rate-limit-dynamic-design.en.md`",
		},
		{
			name:     "relative kitex link",
			input:    "[kitex](../kitex/design-doc.en.md)",
			expected: "[kitex](./kitex/design-doc.en.md)",
		},
		{
			name:     "absolute kitex link in hertz doc",
			input:    "[kitex docs](docs/kitex/design-doc.en.md)",
			expected: "[kitex docs](./kitex/design-doc.en.md)",
		},
		{
			name:     "no links unchanged",
			input:    "plain text with no links",
			expected: "plain text with no links",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteDocLinks(tt.input, "hertz", "en")
			if got != tt.expected {
				t.Errorf("rewriteDocLinks() = %q, want %q", got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestRewriteDocLinks -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/ai/sync_test.go
git commit -m "test(ai): add comprehensive tests for standalone doc generation"
```

---

### Task 3: End-to-end validation and doc update

**Files:**
- Modify: `README.md`, `README.zh-CN.md`, `docs/examples.md`, `docs/examples.zh-CN.md`
- No new test files

- [ ] **Step 1: Run all existing ai tests to verify no regressions**

Run: `go test ./internal/ai/ -v -count=1`
Expected: ALL PASS (existing tests + new tests)

- [ ] **Step 2: Run full build and vet**

Run: `go build ./... && go vet ./...`
Expected: clean

- [ ] **Step 3: Verify with real project**

Run:
```bash
go build -o /tmp/ncgo-test .
/tmp/ncgo-test ai sync --root /Users/baoyx/Documents/workspace/github.com/acme/user-api --lang en
ls /Users/baoyx/Documents/workspace/github.com/acme/user-api/docs/ncgo/hertz/
```
Expected: `design-doc.en.md` and `rate-limit-dynamic-design.en.md` exist, plus `docs/ncgo/kitex/design-doc.en.md`

- [ ] **Step 4: Verify link rewriting in generated files**

Run:
```bash
grep -n "docs/hertz/\|docs/kitex/\|../hertz/\|../kitex/" /Users/baoyx/Documents/workspace/github.com/acme/user-api/docs/ncgo/hertz/design-doc.en.md
```
Expected: no output (all links rewritten to `./` form)

- [ ] **Step 5: Update documentation**

In `docs/examples.md`, add a section after the existing AI sync examples:

```markdown
### Standalone reference docs

`ncgo ai sync` also generates standalone documentation files under `docs/ncgo/`:

```bash
# English (default)
ncgo ai sync --root ./user-api

# Chinese
ncgo ai sync --root ./user-api --lang zh-CN
```

This produces:
- `docs/ncgo/hertz/design-doc.en.md` — Hertz architecture design doc
- `docs/ncgo/hertz/rate-limit-dynamic-design.en.md` — Dynamic rate-limit design doc
- `docs/ncgo/kitex/design-doc.en.md` — Kitex counterpart (for cross-references)

Cross-profile links are automatically rewritten to local relative paths.
```

In `docs/examples.zh-CN.md`, add the Chinese version:

```markdown
### 独立参考文档

`ncgo ai sync` 同时生成独立文档到 `docs/ncgo/` 目录：

```bash
# 英文（默认）
ncgo ai sync --root ./user-api

# 中文
ncgo ai sync --root ./user-api --lang zh-CN
```

产出文件：
- `docs/ncgo/hertz/design-doc.en.md` — Hertz 架构设计文档
- `docs/ncgo/hertz/rate-limit-dynamic-design.en.md` — 动态限流设计文档
- `docs/ncgo/kitex/design-doc.en.md` — Kitex 对应文档（用于交叉引用）

跨 profile 的链接会自动改写为本地相对路径。
```

- [ ] **Step 6: Run markdown diagnostics**

Run: `markdownlint README.md docs/examples.md docs/examples.zh-CN.md README.zh-CN.md 2>&1 | head -20`

- [ ] **Step 7: Commit**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs: add standalone docs/ncgo/ generation to examples"
```

---

## Summary of changes

| File | Change |
|------|--------|
| `internal/ai/sync.go` | +~80 lines: `writeStandaloneDocs`, `listDocSpecs`, `crossProfiles`, `resolveProfile`, `rewriteDocLinks`, `docSpec` type, call site in `Sync()` |
| `internal/ai/sync_test.go` | +~120 lines: 4 new tests for standalone doc generation |
| `README.md`, `README.zh-CN.md` | +~15 lines: document `docs/ncgo/` output |
| `docs/examples.md`, `docs/examples.zh-CN.md` | +~20 lines: usage examples for standalone docs |

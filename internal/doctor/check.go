package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scan"
)

// RunCheck validates AI context integrity and manifest consistency for the
// ncgo service rooted at root: it verifies that every usecase has paired
// // ncgo:methods anchors, that manifest domains match internal/usecase/*/
// directories, and that rendered AI context files' declared domains match
// the manifest. It returns an error only when root is not an ncgo service
// (e.g. missing or invalid manifest).
func RunCheck(root string) (*Report, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	s, err := scan.Scan(root)
	if err != nil {
		return nil, err
	}
	rep := &Report{Root: root, Scope: ScopeService}
	rep.Checks = append(rep.Checks, checkAnchors(s)...)
	rep.Checks = append(rep.Checks, checkConsistency(s)...)
	rep.Checks = append(rep.Checks, checkContextStale(root, m)...)
	rep.Summary = Summarize(rep.Checks)
	return rep, nil
}

func checkAnchors(s *scan.ScanResult) []Check {
	var out []Check
	bad := 0
	for _, d := range s.Domains {
		if d.UsecaseExists && !d.AnchorsOK {
			bad++
			out = append(out, Check{
				ID: "check.anchor", OK: false, Severity: SeverityError,
				Message: fmt.Sprintf("domain %s has unpaired method anchors", d.Name),
				Hint:    "run `ncgo add method <domain>.X` or fix the // ncgo:methods:start|end markers",
			})
		}
	}
	if bad == 0 {
		out = append(out, Check{
			ID: "check.anchor", OK: true, Severity: SeverityError,
			Message: "all usecase files have paired method anchors",
		})
	}
	return out
}

func checkConsistency(s *scan.ScanResult) []Check {
	var out []Check
	bad := 0
	for _, i := range s.Issues {
		if i.Kind != scan.IssueMissingUsecase && i.Kind != scan.IssueUndeclaredDomain {
			continue
		}
		bad++
		out = append(out, Check{
			ID: "check.manifest.consistency", OK: false, Severity: SeverityError,
			Message: i.Message, File: i.File,
		})
	}
	if bad == 0 {
		out = append(out, Check{
			ID: "check.manifest.consistency", OK: true, Severity: SeverityError,
			Message: "manifest domains match internal/usecase/*/ directories",
		})
	}
	return out
}

// checkContextStale compares the domains declared in a rendered context file
// (CLAUDE.md or AGENTS.md, whichever exists) against the current manifest. A
// mismatch means the AI context is stale (a domain was added/removed without
// re-running `ai sync`). Missing context files are skipped (not a failure).
func checkContextStale(root string, m *manifest.Manifest) []Check {
	path := ""
	for _, rel := range contextFileTargets() {
		if rel == ".claude/skills/ncgo-dev/SKILL.md" || rel == ".cursor/rules/ncgo.mdc" {
			continue // SKILL.md / .mdc do not carry the domains fact line
		}
		candidate := filepath.Join(root, rel)
		if pathExists(candidate) {
			path = candidate
			break
		}
	}
	if path == "" {
		return []Check{okContextCheck("no rendered context file present; nothing to compare")}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return []Check{{
			ID: "check.context.stale", OK: false, Severity: SeverityError,
			Message: fmt.Sprintf("read %s: %v", path, err), File: path,
		}}
	}
	rendered := parseContextDomains(string(body))
	if rendered == nil {
		return []Check{okContextCheck("context file has no domains fact line")}
	}
	if !sameStringSet(rendered, m.Domains) {
		return []Check{{
			ID: "check.context.stale", OK: false, Severity: SeverityError,
			Message: fmt.Sprintf("%s is stale: context declares domains %v, manifest has %v", filepath.Base(path), rendered, m.Domains),
			File:    path,
			Hint:    "run `ncgo ai sync --root .`",
		}}
	}
	return []Check{okContextCheck("AI context domains match manifest")}
}

func okContextCheck(msg string) Check {
	return Check{ID: "check.context.stale", OK: true, Severity: SeverityError, Message: msg}
}

// parseContextDomains extracts the domain list from a rendered context file's
// "- domains: [a, b]" fact line. Returns nil when the line is absent.
func parseContextDomains(content string) []string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- domains: `[") {
			continue
		}
		rest := strings.TrimPrefix(line, "- domains: `[")
		rest = strings.TrimSuffix(rest, "]`")
		if rest == "" {
			return []string{}
		}
		parts := strings.Split(rest, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// sameStringSet reports whether two slices contain the same strings
// (order-insensitive).
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

// contextFileTargets lists the context files ai sync renders and check audits.
func contextFileTargets() []string {
	return []string{
		"AGENTS.md",
		"CLAUDE.md",
		".claude/skills/ncgo-dev/SKILL.md",
		".claude/generated/project-context.md",
		".cursor/rules/ncgo.mdc",
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

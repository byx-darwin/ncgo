package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/doctor"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scan"
)

type checkOptions struct {
	root   string
	output string
}

// exitCodeError lets a command choose its process exit code explicitly.
type exitCodeError struct {
	code int
	msg  string
}

func (e *exitCodeError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("ncgo check: exited with code %d", e.code)
}

func newCheckCmd() *cobra.Command {
	opts := &checkOptions{}
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate AI context integrity and manifest consistency",
		Long: "Verify that every usecase has paired // ncgo:methods anchors, that " +
			"manifest domains match internal/usecase/*/ directories, and that rendered " +
			"AI context files' declared domains match the manifest. Exits 0 on pass, 1 on " +
			"check failure, 2 on command error (e.g. root is not an ncgo service).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Service root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runCheck(cmd *cobra.Command, opts *checkOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return &exitCodeError{code: 2, msg: fmt.Sprintf("check: unsupported --output %q; want text or json", opts.output)}
	}
	rep, err := buildCheckReport(opts.root)
	if err != nil {
		return &exitCodeError{code: 2, msg: err.Error()}
	}
	switch opts.output {
	case "json":
		err = doctor.WriteJSON(cmd.OutOrStdout(), rep)
	default:
		err = doctor.WriteText(cmd.OutOrStdout(), rep)
	}
	if err != nil {
		return &exitCodeError{code: 2, msg: err.Error()}
	}
	if !rep.OK() {
		return &exitCodeError{code: 1}
	}
	return nil
}

// buildCheckReport assembles a doctor.Report from a scan of the service root.
// It returns an error only when the root is not an ncgo service.
func buildCheckReport(root string) (*doctor.Report, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	s, err := scan.Scan(root)
	if err != nil {
		return nil, err
	}
	rep := &doctor.Report{Root: root, Scope: doctor.ScopeService}
	rep.Checks = append(rep.Checks, anchorChecks(s)...)
	rep.Checks = append(rep.Checks, consistencyChecks(s)...)
	rep.Checks = append(rep.Checks, contextStaleChecks(root, m)...)
	rep.Summary = doctor.Summarize(rep.Checks)
	return rep, nil
}

func anchorChecks(s *scan.ScanResult) []doctor.Check {
	var out []doctor.Check
	bad := 0
	for _, d := range s.Domains {
		if d.UsecaseExists && !d.AnchorsOK {
			bad++
			out = append(out, doctor.Check{
				ID: "check.anchor", OK: false, Severity: doctor.SeverityError,
				Message: fmt.Sprintf("domain %s has unpaired method anchors", d.Name),
				Hint:    "run `ncgo add method <domain>.X` or fix the // ncgo:methods:start|end markers",
			})
		}
	}
	if bad == 0 {
		out = append(out, doctor.Check{
			ID: "check.anchor", OK: true, Severity: doctor.SeverityError,
			Message: "all usecase files have paired method anchors",
		})
	}
	return out
}

func consistencyChecks(s *scan.ScanResult) []doctor.Check {
	var out []doctor.Check
	bad := 0
	for _, i := range s.Issues {
		if i.Kind != scan.IssueMissingUsecase && i.Kind != scan.IssueUndeclaredDomain {
			continue
		}
		bad++
		out = append(out, doctor.Check{
			ID: "check.manifest.consistency", OK: false, Severity: doctor.SeverityError,
			Message: i.Message, File: i.File,
		})
	}
	if bad == 0 {
		out = append(out, doctor.Check{
			ID: "check.manifest.consistency", OK: true, Severity: doctor.SeverityError,
			Message: "manifest domains match internal/usecase/*/ directories",
		})
	}
	return out
}

// contextStaleChecks compares the domains declared in a rendered context file
// (CLAUDE.md or AGENTS.md, whichever exists) against the current manifest. A
// mismatch means the AI context is stale (a domain was added/removed without
// re-running `ai sync`). Missing context files are skipped (not a failure).
func contextStaleChecks(root string, m *manifest.Manifest) []doctor.Check {
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
		return []doctor.Check{okContextCheck("no rendered context file present; nothing to compare")}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return []doctor.Check{{
			ID: "check.context.stale", OK: false, Severity: doctor.SeverityError,
			Message: fmt.Sprintf("read %s: %v", path, err), File: path,
		}}
	}
	rendered := parseContextDomains(string(body))
	if rendered == nil {
		return []doctor.Check{okContextCheck("context file has no domains fact line")}
	}
	if !sameStringSet(rendered, m.Domains) {
		return []doctor.Check{{
			ID: "check.context.stale", OK: false, Severity: doctor.SeverityError,
			Message: fmt.Sprintf("%s is stale: context declares domains %v, manifest has %v", filepath.Base(path), rendered, m.Domains),
			File:    path,
			Hint:    "run `ncgo ai sync --root .`",
		}}
	}
	return []doctor.Check{okContextCheck("AI context domains match manifest")}
}

func okContextCheck(msg string) doctor.Check {
	return doctor.Check{ID: "check.context.stale", OK: true, Severity: doctor.SeverityError, Message: msg}
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

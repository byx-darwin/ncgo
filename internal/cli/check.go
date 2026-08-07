package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/ai"
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
			"AI context files are not older than the manifest. Exits 0 on pass, 1 on " +
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

// contextStaleChecks compares each rendered context file's generated-at marker
// against the manifest timestamp. Missing files are skipped (not a failure).
func contextStaleChecks(root string, m *manifest.Manifest) []doctor.Check {
	var out []doctor.Check
	stale := 0
	for _, rel := range contextFileTargets() {
		path := filepath.Join(root, rel)
		if !pathExists(path) {
			continue
		}
		ts, ok := ai.ReadGeneratedAt(path)
		if ok && !ts.Before(m.GeneratedAt) {
			continue
		}
		stale++
		out = append(out, doctor.Check{
			ID: "check.context.stale", OK: false, Severity: doctor.SeverityError,
			Message: fmt.Sprintf("%s is stale (context older than manifest)", rel),
			File:    path,
			Hint:    "run `ncgo ai sync --root .`",
		})
	}
	if stale == 0 {
		out = append(out, doctor.Check{
			ID: "check.context.stale", OK: true, Severity: doctor.SeverityError,
			Message: "AI context files are up to date",
		})
	}
	return out
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

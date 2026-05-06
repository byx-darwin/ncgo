// Package doctor implements `ncgo doctor`: it inspects the host and the
// project, returning a structured Report consumable by humans, CI, and AI
// agents (PRD §8).
//
// v0.1 covers four categories:
//   - tools: hz / kitex presence on PATH and minimum version
//   - manifest: .ncgo/manifest.yaml loads and validates
//   - data: template/data.json values agree with the manifest
//   - proto: manifest.service.idl passes Proto I/O lint checks
//
// Static layer-rule checks (handler→repo, usecase→hertz, SQL strings,
// RequestContext leaks) are scoped to v0.2 because they need an AST scanner;
// the report schema is forward-compatible.
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// Scope classifies what kind of target doctor inspected.
type Scope string

const (
	ScopeHost      Scope = "host"
	ScopeService   Scope = "service"
	ScopeWorkspace Scope = "workspace"
	ScopeUnknown   Scope = "unknown"
)

// Severity classifies the consequence of a failed check.
type Severity string

const (
	SeverityError Severity = "error" // blocks: missing tool, invalid manifest
	SeverityWarn  Severity = "warn"  // non-blocking: drift, soft inconsistency
)

// Check is one assertion about the host or project.
type Check struct {
	ID       string   `json:"id"`
	OK       bool     `json:"ok"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Rule     string   `json:"rule,omitempty"`
}

// ReportSummary is the aggregate count view of a doctor run.
type ReportSummary struct {
	CheckCount   int `json:"checkCount"`
	PassedCount  int `json:"passedCount"`
	FailedCount  int `json:"failedCount"`
	ErrorCount   int `json:"errorCount"`
	WarningCount int `json:"warningCount"`
}

// Report is the aggregate output of a doctor run.
type Report struct {
	Root    string        `json:"root,omitempty"`
	Scope   Scope         `json:"scope,omitempty"`
	Summary ReportSummary `json:"summary"`
	Checks  []Check       `json:"checks"`
}

// OK reports whether every error-severity check passed.
func (r *Report) OK() bool {
	for _, c := range r.Checks {
		if !c.OK && c.Severity == SeverityError {
			return false
		}
	}
	return true
}

// Options configures Run.
type Options struct {
	Root   string      // project root; "" means skip project checks
	Runner exec.Runner // injected for tool probes; nil → exec.NewDefault()
}

// Run executes every check in a stable order. It does not return early on
// failure; callers inspect Report for full results.
func Run(ctx context.Context, opts Options) *Report {
	r := &Report{Root: opts.Root, Scope: ScopeHost}
	runner := opts.Runner
	if runner == nil {
		runner = exec.NewDefault()
	}
	r.Checks = append(r.Checks, checkTool(ctx, runner, "hz", []string{"--version"}, exec.MinHzVersion))
	r.Checks = append(r.Checks, checkTool(ctx, runner, "kitex", []string{"-version"}, exec.MinKitexVersion))
	if opts.Root != "" {
		checks, scope := projectChecks(ctx, opts.Root)
		r.Scope = scope
		r.Checks = append(r.Checks, checks...)
	}
	r.Summary = summarizeChecks(r.Checks)
	return r
}

func projectChecks(ctx context.Context, root string) ([]Check, Scope) {
	var out []Check
	m, mc := loadManifestCheck(root)
	if m != nil {
		out = append(out, mc)
		out = append(out, dataJSONCheck(root, m))
		out = append(out, protoLintChecks(ctx, root, m)...)
		out = append(out, scanLayers(root, m)...)
		return out, ScopeService
	}
	w, wc := loadWorkspaceCheck(root)
	if w != nil {
		out = append(out, wc)
		out = append(out, workspaceProtoLintChecks(ctx, root, w)...)
		return out, ScopeWorkspace
	}
	return append(out, mc), ScopeUnknown
}

func summarizeChecks(checks []Check) ReportSummary {
	var s ReportSummary
	s.CheckCount = len(checks)
	for _, c := range checks {
		if c.OK {
			s.PassedCount++
			continue
		}
		s.FailedCount++
		if c.Severity == SeverityWarn {
			s.WarningCount++
		} else {
			s.ErrorCount++
		}
	}
	return s
}

func loadManifestCheck(root string) (*manifest.Manifest, Check) {
	c := Check{ID: "manifest.load", Severity: SeverityError}
	m, err := manifest.Load(root)
	if err != nil {
		c.OK = false
		c.Message = err.Error()
		c.Hint = "run `ncgo new` to scaffold a project, or check that you are in the project root"
		c.File = manifest.Path(root)
		return nil, c
	}
	c.OK = true
	c.Message = fmt.Sprintf("manifest loaded: %s (%s)", m.Service.Name, m.Mode)
	c.File = manifest.Path(root)
	return m, c
}

func loadWorkspaceCheck(root string) (*manifest.Workspace, Check) {
	c := Check{ID: "workspace.load", Severity: SeverityError}
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		c.OK = false
		c.Message = err.Error()
		c.Hint = "run `ncgo new --mode micro` to scaffold a workspace, or check that you are in the workspace root"
		c.File = manifest.WorkspacePath(root)
		return nil, c
	}
	c.OK = true
	c.Message = fmt.Sprintf("workspace loaded: %s (%s, services=%d)", w.Name, w.Mode, len(w.Services))
	c.File = manifest.WorkspacePath(root)
	return w, c
}

// dataJSONCheck verifies that the values hz reads from template/data.json
// match the manifest's source-of-truth fields.
func dataJSONCheck(root string, m *manifest.Manifest) Check {
	path := filepath.Join(root, "template", "data.json")
	c := Check{ID: "manifest.data.consistent", Severity: SeverityWarn, File: path}
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.OK = true
			c.Message = "template/data.json not present (skipped)"
			return c
		}
		c.OK = false
		c.Message = "read template/data.json: " + err.Error()
		return c
	}
	var parsed map[string]map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		c.OK = false
		c.Message = "parse template/data.json: " + err.Error()
		return c
	}
	star := parsed["*"]
	var diffs []string
	if v, _ := star["GoModule"].(string); v != m.Module {
		diffs = append(diffs, fmt.Sprintf("GoModule=%q != manifest.module=%q", v, m.Module))
	}
	if v, _ := star["ServiceName"].(string); v != m.Service.Name {
		diffs = append(diffs, fmt.Sprintf("ServiceName=%q != manifest.service.name=%q", v, m.Service.Name))
	}
	if v, _ := star["WithDatabase"].(bool); v != m.Service.WithDatabase {
		diffs = append(diffs, fmt.Sprintf("WithDatabase=%v != manifest.service.with_database=%v", v, m.Service.WithDatabase))
	}
	if len(diffs) == 0 {
		c.OK = true
		c.Message = "template/data.json agrees with manifest"
		return c
	}
	c.OK = false
	c.Message = "template/data.json drift: " + strings.Join(diffs, ", ")
	c.Hint = "regenerate via `ncgo new --module ... --no-generate` or hand-edit template/data.json"
	return c
}

// Package orchestrator provides a shared service layer that wraps internal
// domain packages (doctor, protolint, scaffold, etc.) so that both the CLI
// and MCP server can call the same thin orchestration functions.
//
// Each operation has its own file with Options, Result, and a Run* function.
// The orchestrator is a thin wrapper: it does NOT change the internal packages
// it wraps.
package orchestrator

import (
	"context"

	"github.com/byx-darwin/ncgo/internal/doctor"
	"github.com/byx-darwin/ncgo/internal/exec"
)

// DoctorOptions configures a doctor run.
type DoctorOptions struct {
	Root   string      // project root; "" means skip project checks
	Runner exec.Runner // injected for tool probes; nil means use default
}

// DoctorResult is the structured result of a doctor run.
type DoctorResult struct {
	Root    string               `json:"root,omitempty"`
	Scope   string               `json:"scope,omitempty"`
	OK      bool                 `json:"ok"`
	Summary doctor.ReportSummary `json:"summary"`
	Checks  []doctor.Check       `json:"checks"`
	// Report holds the raw doctor.Report for formatting by CLI/MCP layers.
	Report *doctor.Report `json:"-"`
}

// RunDoctor executes a doctor run and returns a structured result.
// It wraps doctor.Run without changing its public API.
func RunDoctor(ctx context.Context, opts DoctorOptions) (*DoctorResult, error) {
	report := doctor.Run(ctx, doctor.Options{Root: opts.Root, Runner: opts.Runner})
	return &DoctorResult{
		Root:    report.Root,
		Scope:   string(report.Scope),
		OK:      report.OK(),
		Summary: report.Summary,
		Checks:  report.Checks,
		Report:  report,
	}, nil
}

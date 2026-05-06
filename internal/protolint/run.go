package protolint

import (
	"context"
	"fmt"
	"sort"
)

// Summary is the aggregated execution summary for a lint run.
type Summary struct {
	FilesScanned     int `json:"filesScanned"`
	RPCsScanned      int `json:"rpcsScanned"`
	DiagnosticsCount int `json:"diagnosticsCount"`
	ErrorCount       int `json:"errorCount"`
	WarningCount     int `json:"warningCount"`
}

// Result is the structured outcome of a lint run.
type Result struct {
	Root        string       `json:"root"`
	Files       []string     `json:"files"`
	RulesRun    []string     `json:"rulesRun"`
	OK          bool         `json:"ok"`
	Summary     Summary      `json:"summary"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// RunOptions configures a complete lint run.
type RunOptions struct {
	Root    string
	Files   []string
	RuleIDs []string
}

// Run loads the target proto files, executes the selected rules, and returns
// a structured result.
func Run(ctx context.Context, opts RunOptions) (*Result, error) {
	rules, err := resolveRules(opts.RuleIDs)
	if err != nil {
		return nil, err
	}
	model, err := Load(ctx, LoadOptions{Root: opts.Root, Files: opts.Files})
	if err != nil {
		return nil, err
	}
	diags := Check(model, CheckOptions{Rules: rules})
	if diags == nil {
		diags = []Diagnostic{}
	}
	errorCount, warningCount := countDiagnostics(diags)
	res := &Result{
		Root:        opts.Root,
		Files:       append([]string(nil), opts.Files...),
		RulesRun:    ruleIDs(rules),
		OK:          errorCount == 0,
		Diagnostics: diags,
		Summary: Summary{
			FilesScanned:     len(model.Files),
			RPCsScanned:      len(model.RPCs()),
			DiagnosticsCount: len(diags),
			ErrorCount:       errorCount,
			WarningCount:     warningCount,
		},
	}
	return res, nil
}

func countDiagnostics(diags []Diagnostic) (int, int) {
	var errors, warnings int
	for _, d := range diags {
		switch d.Level {
		case LevelWarning:
			warnings++
		default:
			errors++
		}
	}
	return errors, warnings
}

func resolveRules(ruleIDs []string) ([]Rule, error) {
	all := DefaultRules()
	if len(ruleIDs) == 0 {
		return all, nil
	}
	index := make(map[string]Rule, len(all))
	for _, rule := range all {
		index[rule.Meta().ID] = rule
	}
	out := make([]Rule, 0, len(ruleIDs))
	seen := map[string]struct{}{}
	for _, id := range ruleIDs {
		rule, ok := index[id]
		if !ok {
			return nil, fmt.Errorf("protolint: unknown rule %q", id)
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, rule)
	}
	return out, nil
}

func ruleIDs(rules []Rule) []string {
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rule.Meta().ID)
	}
	sort.Strings(out)
	return out
}

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
	SuppressedCount  int `json:"suppressedCount,omitempty"`
}

// Result is the structured outcome of a lint run.
type Result struct {
	Root         string       `json:"root"`
	Files        []string     `json:"files"`
	RulesRun     []string     `json:"rulesRun"`
	IgnoredRules []string     `json:"ignoredRules,omitempty"`
	IgnoredFiles []string     `json:"ignoredFiles,omitempty"`
	OK           bool         `json:"ok"`
	Summary      Summary      `json:"summary"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}

// RunOptions configures a complete lint run.
type RunOptions struct {
	Root          string
	Files         []string
	RuleIDs       []string
	IgnoreRuleIDs []string
	IgnoreFiles   []string
}

// Run loads the target proto files, executes the selected rules, and returns
// a structured result.
func Run(ctx context.Context, opts RunOptions) (*Result, error) {
	rules, err := resolveRules(opts.RuleIDs)
	if err != nil {
		return nil, err
	}
	files := append([]string(nil), opts.Files...)
	var (
		diags        []Diagnostic
		filesScanned int
		rpcsScanned  int
	)
	if len(files) > 0 {
		diags, filesScanned, rpcsScanned, err = runSingleRoot(ctx, opts.Root, files, rules)
		if err != nil {
			return nil, err
		}
	} else if files, err = discoverServiceFiles(opts.Root); err == nil {
		diags, filesScanned, rpcsScanned, err = runSingleRoot(ctx, opts.Root, files, rules)
		if err != nil {
			return nil, err
		}
	} else if targets, werr := discoverWorkspaceTargets(opts.Root); werr == nil {
		files = files[:0]
		for _, target := range targets {
			serviceDiags, serviceFilesScanned, serviceRPCsScanned, runErr := runSingleRoot(ctx, target.Root, []string{target.File}, rules)
			if runErr != nil {
				return nil, runErr
			}
			for i := range serviceDiags {
				serviceDiags[i].File = prefixWorkspacePath(target.Prefix, serviceDiags[i].File)
			}
			diags = append(diags, serviceDiags...)
			files = append(files, target.Path)
			filesScanned += serviceFilesScanned
			rpcsScanned += serviceRPCsScanned
		}
		sortDiagnostics(diags)
	} else {
		switch {
		case hasManifestMetadata(opts.Root):
			return nil, err
		case hasWorkspaceMetadata(opts.Root):
			return nil, werr
		default:
			return nil, missingFilesDiscoveryError()
		}
	}
	if diags == nil {
		diags = []Diagnostic{}
	}
	filteredDiags, suppressedCount, ignoredRules, ignoredFiles := applyIgnores(opts.Root, diags, opts.IgnoreRuleIDs, opts.IgnoreFiles)
	errorCount, warningCount := countDiagnostics(filteredDiags)
	res := &Result{
		Root:         opts.Root,
		Files:        files,
		RulesRun:     ruleIDs(rules),
		IgnoredRules: ignoredRules,
		IgnoredFiles: ignoredFiles,
		OK:           errorCount == 0,
		Diagnostics:  filteredDiags,
		Summary: Summary{
			FilesScanned:     filesScanned,
			RPCsScanned:      rpcsScanned,
			DiagnosticsCount: len(filteredDiags),
			ErrorCount:       errorCount,
			WarningCount:     warningCount,
			SuppressedCount:  suppressedCount,
		},
	}
	return res, nil
}

func runSingleRoot(ctx context.Context, root string, files []string, rules []Rule) ([]Diagnostic, int, int, error) {
	model, err := Load(ctx, LoadOptions{Root: root, Files: files})
	if err != nil {
		return nil, 0, 0, err
	}
	diags := Check(model, CheckOptions{Rules: rules})
	if diags == nil {
		diags = []Diagnostic{}
	}
	return diags, len(model.Files), len(model.RPCs()), nil
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

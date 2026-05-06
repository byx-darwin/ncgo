package protolint

import "sort"

// Rule checks a model and returns zero or more diagnostics.
type Rule interface {
	Meta() RuleMeta
	Check(*Model) []Diagnostic
}

// CheckOptions configures which rules run.
type CheckOptions struct {
	Rules   []Rule
	RuleIDs []string
	Levels  []Level
	Phases  []Phase
}

// Check executes the selected rules and returns sorted diagnostics.
func Check(model *Model, opts CheckOptions) []Diagnostic {
	rules := opts.Rules
	if len(rules) == 0 {
		rules = DefaultRules()
	}
	var out []Diagnostic
	for _, rule := range rules {
		meta := rule.Meta()
		if !allowRule(meta, opts) {
			continue
		}
		out = append(out, rule.Check(model)...)
	}
	sortDiagnostics(out)
	return out
}

// DefaultRules returns the built-in rules that are currently implemented.
func DefaultRules() []Rule {
	return []Rule{
		rulePIO101{},
		rulePIO102{},
		rulePIO103{},
		rulePIO111{},
		rulePIO112{},
		rulePIO113{},
		rulePIO211{},
		rulePIO212{},
		rulePIO302{},
		rulePIO303{},
		rulePIO401{},
		rulePIO201{},
		rulePIO202{},
		rulePIO203{},
		rulePIO204{},
		rulePIO205{},
		rulePIO206{},
		rulePIO301{},
	}
}

func allowRule(meta RuleMeta, opts CheckOptions) bool {
	if len(opts.RuleIDs) > 0 && !containsString(opts.RuleIDs, meta.ID) {
		return false
	}
	if len(opts.Levels) > 0 && !containsLevel(opts.Levels, meta.Level) {
		return false
	}
	if len(opts.Phases) > 0 && !containsPhase(opts.Phases, meta.Phase) {
		return false
	}
	return true
}

func sortDiagnostics(diags []Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		if a.Service != b.Service {
			return a.Service < b.Service
		}
		if a.RPC != b.RPC {
			return a.RPC < b.RPC
		}
		if a.Message != b.Message {
			return a.Message < b.Message
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		return a.Summary < b.Summary
	})
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func containsLevel(xs []Level, want Level) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func containsPhase(xs []Phase, want Phase) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

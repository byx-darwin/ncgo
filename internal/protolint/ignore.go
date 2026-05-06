package protolint

import (
	"path/filepath"
	"sort"
	"strings"
)

func applyIgnores(root string, diags []Diagnostic, ruleIDs, files []string) ([]Diagnostic, int, []string, []string) {
	ignoreRules := normalizeRuleIDs(ruleIDs)
	ignoreFiles := normalizeIgnoreFiles(root, files)
	if len(ignoreRules) == 0 && len(ignoreFiles) == 0 {
		return diags, 0, sortedIgnoreKeys(ignoreRules), sortedIgnoreKeys(ignoreFiles)
	}
	out := make([]Diagnostic, 0, len(diags))
	suppressed := 0
	for _, d := range diags {
		if _, ok := ignoreRules[d.RuleID]; ok {
			suppressed++
			continue
		}
		if _, ok := ignoreFiles[normalizeDiagnosticFile(d.File)]; ok {
			suppressed++
			continue
		}
		out = append(out, d)
	}
	return out, suppressed, sortedIgnoreKeys(ignoreRules), sortedIgnoreKeys(ignoreFiles)
}

func normalizeRuleIDs(ruleIDs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ruleIDs))
	for _, id := range ruleIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func normalizeIgnoreFiles(root string, files []string) map[string]struct{} {
	out := make(map[string]struct{}, len(files))
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		out[normalizeInputFile(root, file)] = struct{}{}
	}
	return out
}

func normalizeInputFile(root, file string) string {
	cleaned := filepath.Clean(file)
	if root != "" && filepath.IsAbs(cleaned) {
		rootAbs, err := filepath.Abs(root)
		if err == nil {
			if rel, err := filepath.Rel(rootAbs, cleaned); err == nil && isSubpath(rel) {
				cleaned = rel
			}
		}
	}
	return filepath.ToSlash(cleaned)
}

func normalizeDiagnosticFile(file string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(file)))
}

func isSubpath(rel string) bool {
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func sortedIgnoreKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

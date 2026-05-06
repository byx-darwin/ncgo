package doctor

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/protolint"
)

func protoLintChecks(ctx context.Context, root string, m *manifest.Manifest) []Check {
	if m == nil {
		return nil
	}
	idl := strings.TrimSpace(m.Service.IDL)
	if idl == "" {
		return nil
	}
	res, err := protolint.Run(ctx, protolint.RunOptions{
		Root:  root,
		Files: []string{idl},
	})
	file := absOrJoin(root, idl)
	if err != nil {
		return []Check{{
			ID:       "protolint.load",
			OK:       false,
			Severity: SeverityError,
			Message:  "proto lint failed: " + err.Error(),
			Hint:     "check manifest.service.idl, proto imports, and proto syntax",
			File:     file,
		}}
	}
	if res.OK {
		return []Check{{
			ID:       "protolint",
			OK:       true,
			Severity: SeverityError,
			Message:  fmt.Sprintf("proto lint passed: %s (rules=%d)", idl, len(res.RulesRun)),
			File:     file,
		}}
	}
	out := make([]Check, 0, len(res.Diagnostics))
	for i, d := range res.Diagnostics {
		out = append(out, Check{
			ID:       fmt.Sprintf("protolint.%s.%d", strings.ToLower(d.RuleID), i+1),
			OK:       false,
			Severity: severityForProtoLevel(d.Level),
			Message:  d.Summary,
			Hint:     d.Hint,
			File:     absOrJoin(root, d.File),
			Line:     d.Line,
			Rule:     d.RuleID,
		})
	}
	return out
}

func workspaceProtoLintChecks(ctx context.Context, root string, w *manifest.Workspace) []Check {
	if w == nil {
		return nil
	}
	res, err := protolint.Run(ctx, protolint.RunOptions{Root: root})
	if err != nil {
		return []Check{{
			ID:       "workspace.protolint.load",
			OK:       false,
			Severity: SeverityError,
			Message:  "workspace proto lint failed: " + err.Error(),
			Hint:     "check ncgo.workspace, listed service manifests, and service proto imports",
			File:     manifest.WorkspacePath(root),
		}}
	}
	if res.OK {
		return []Check{{
			ID:       "workspace.protolint",
			OK:       true,
			Severity: SeverityError,
			Message:  fmt.Sprintf("workspace proto lint passed: services=%d files=%d (rules=%d)", len(w.Services), len(res.Files), len(res.RulesRun)),
			File:     manifest.WorkspacePath(root),
		}}
	}
	out := make([]Check, 0, len(res.Diagnostics))
	for i, d := range res.Diagnostics {
		out = append(out, Check{
			ID:       fmt.Sprintf("workspace.protolint.%s.%d", strings.ToLower(d.RuleID), i+1),
			OK:       false,
			Severity: severityForProtoLevel(d.Level),
			Message:  d.Summary,
			Hint:     d.Hint,
			File:     absOrJoin(root, d.File),
			Line:     d.Line,
			Rule:     d.RuleID,
		})
	}
	return out
}

func severityForProtoLevel(level protolint.Level) Severity {
	switch level {
	case protolint.LevelWarning:
		return SeverityWarn
	default:
		return SeverityError
	}
}

func absOrJoin(root, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

package mcp

import (
	"strings"

	"github.com/byx-darwin/ncgo/internal/ai"
	planpkg "github.com/byx-darwin/ncgo/internal/scaffold/plan"
)

type mcpEffect struct {
	Kind   string `json:"kind"`
	Action string `json:"action"`
	Status string `json:"status"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func withEffects(fields map[string]any, effects []mcpEffect) map[string]any {
	if effects == nil {
		effects = []mcpEffect{}
	}
	fields["effects"] = effects
	return fields
}

func setResultEffects(result map[string]any, effects []mcpEffect) {
	if effects == nil {
		effects = []mcpEffect{}
	}
	result["effects"] = effects
	if envelope, ok := result["structuredContent"].(map[string]any); ok {
		envelope["effects"] = effects
	}
}

func effectsFromPlan(items []planpkg.Item, dryRun bool) []mcpEffect {
	effects := make([]mcpEffect, 0, len(items))
	for _, item := range items {
		if item.Kind == "next_step" {
			continue
		}
		status := "applied"
		if dryRun {
			status = "planned"
		}
		if item.Action == "skip" || strings.HasPrefix(item.Action, "already_") {
			status = "skipped"
		}
		effects = append(effects, mcpEffect{
			Kind:   item.Kind,
			Action: item.Action,
			Status: status,
			Path:   item.Path,
			Detail: item.Detail,
		})
	}
	return effects
}

func fileEffects(paths []string, dryRun bool, action string) []mcpEffect {
	status := "applied"
	if dryRun {
		status = "planned"
	}
	effects := make([]mcpEffect, 0, len(paths))
	for _, path := range paths {
		effects = append(effects, mcpEffect{Kind: "file", Action: action, Status: status, Path: path})
	}
	return effects
}

func aiEffects(res *ai.Result, dryRun bool) []mcpEffect {
	if res == nil {
		return []mcpEffect{}
	}
	effects := fileEffects(res.Written, false, "write")
	for _, skipped := range res.Skipped {
		status := "skipped"
		if dryRun && skipped.Reason == "dry-run" {
			status = "planned"
		}
		effects = append(effects, mcpEffect{
			Kind:   "file",
			Action: "write",
			Status: status,
			Path:   skipped.Path,
			Detail: skipped.Reason,
		})
	}
	return effects
}

func newEffects(res *newResult) []mcpEffect {
	effects := []mcpEffect{{Kind: "directory", Action: "create", Status: "applied", Path: res.Dir, Detail: res.Mode + " scaffold"}}
	if res.RanGenerate != nil {
		status := "skipped"
		if *res.RanGenerate {
			status = "applied"
		}
		effects = append(effects, mcpEffect{Kind: "process", Action: "execute_generator", Status: status})
	}
	for _, step := range res.AutoSteps {
		status := "failed"
		switch step.Status {
		case "succeeded":
			status = "applied"
		case "skipped":
			status = "skipped"
		}
		effects = append(effects, mcpEffect{Kind: "process", Action: step.Name, Status: status, Detail: step.Detail})
	}
	return effects
}

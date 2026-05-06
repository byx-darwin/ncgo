package mcp

import (
	"encoding/json"

	"github.com/byx-darwin/ncgo/internal/ai"
)

func callAISync(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Lang   string `json:"lang"`
		Force  bool   `json:"force"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	res, err := ai.Sync(ai.Options{Root: args.Root, Lang: args.Lang, Force: args.Force, DryRun: args.DryRun})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return textResult(string(b), false), nil
}

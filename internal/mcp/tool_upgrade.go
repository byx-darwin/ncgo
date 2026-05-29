package mcp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/upgrade"
)

func callUpgrade(raw json.RawMessage, ncgoVersion, assetsVersion string) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Root == "" {
		args.Root = "."
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}

	res, err := upgrade.Run(upgrade.Options{
		Root:          args.Root,
		NCGOVersion:   ncgoVersion,
		AssetsVersion: assetsVersion,
		Plan:          true, // Always plan mode from MCP: read-only, never writes files.
	})
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	output, err := resolveMCPOutput("upgrade", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	text, err := formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			fmt.Fprintf(w, "upgrade plan for %s (%s)\n", res.Root, res.Mode)
			writeUpgradeItem(w, "metadata", res.Path, res.Changed, res.OldVersion, res.NewVersion, res.OldAssets, res.NewAssets)
			for _, svc := range res.ServiceUpdates {
				label := "service " + svc.Name
				writeUpgradeItem(w, label, svc.Path, svc.Changed, svc.OldVersion, svc.NewVersion, svc.OldAssets, svc.NewAssets)
			}
			if res.Changed {
				fmt.Fprintln(w, "\nnext step: rerun without --plan to write metadata updates")
			} else {
				fmt.Fprintln(w, "\nnext step: no metadata updates needed")
			}
			return nil
		},
		mcpOutputJSON: func(w io.Writer) error {
			return writeJSONOutput(w, buildUpgradePlan(res))
		},
	})
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	return buildMCPResult(text, false, buildUpgradePlan(res)), nil
}

func buildUpgradePlan(res *upgrade.Result) map[string]any {
	items := []map[string]any{{
		"label":      "metadata",
		"path":       res.Path,
		"changed":    res.Changed,
		"oldVersion": res.OldVersion,
		"newVersion": res.NewVersion,
		"oldAssets":  res.OldAssets,
		"newAssets":  res.NewAssets,
	}}
	for _, svc := range res.ServiceUpdates {
		items = append(items, map[string]any{
			"label":      "service " + svc.Name,
			"path":       svc.Path,
			"changed":    svc.Changed,
			"oldVersion": svc.OldVersion,
			"newVersion": svc.NewVersion,
			"oldAssets":  svc.OldAssets,
			"newAssets":  svc.NewAssets,
		})
	}
	return map[string]any{
		"upToDate": !res.Changed,
		"root":     res.Root,
		"mode":     res.Mode,
		"path":     res.Path,
		"plan":     true,
		"items":    items,
	}
}

func writeUpgradeItem(w io.Writer, label, path string, changed bool, oldVersion, newVersion, oldAssets, newAssets string) {
	status := "unchanged"
	if changed {
		status = "change"
	}
	fmt.Fprintf(w, "  - [%s] %s %s\n", status, label, path)
	fmt.Fprintf(w, "      ncgo: %s -> %s\n", oldVersion, newVersion)
	fmt.Fprintf(w, "      assets: %s -> %s\n", oldAssets, newAssets)
}

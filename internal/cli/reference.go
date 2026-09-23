package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// ReferenceCommand is the minimal command metadata used by the generated
// CLI-to-MCP capability matrix. It is derived from the real Cobra tree so new
// runnable commands automatically appear as CLI-only until they are mapped.
type ReferenceCommand struct {
	Path        string
	Description string
}

// ReferenceCommands returns every runnable command registered by ncgo.
func ReferenceCommands() []ReferenceCommand {
	root := newRootCmd()
	root.InitDefaultHelpCmd()
	var out []ReferenceCommand
	var visit func(*cobra.Command, []string)
	visit = func(parent *cobra.Command, prefix []string) {
		for _, cmd := range parent.Commands() {
			path := append(append([]string(nil), prefix...), cmd.Name())
			if cmd.Runnable() {
				out = append(out, ReferenceCommand{Path: strings.Join(path, " "), Description: cmd.Short})
			}
			visit(cmd, path)
		}
	}
	visit(root, []string{"ncgo"})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

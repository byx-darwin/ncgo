package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/ai"
)

func newAICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI collaboration helpers (sync agent context files, etc.)",
	}
	cmd.AddCommand(newAISyncCmd())
	return cmd
}

type aiSyncOptions struct {
	root   string
	lang   string
	force  bool
	dryRun bool
}

func newAISyncCmd() *cobra.Command {
	opts := &aiSyncOptions{}
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Render AGENTS.md, CLAUDE.md and .cursor/rules/ncgo.mdc",
		Long: "Generate the AI collaboration artifacts described in docs/prd.md \u00a76 from " +
			".ncgo/manifest.yaml and the embedded ncgo design doc. Existing files without " +
			"the `<!-- ncgo:managed -->` marker are skipped unless --force is set.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAISync(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.lang, "lang", ai.LangEN, "Design-doc language: en | zh-CN")
	f.BoolVar(&opts.force, "force", false, "Overwrite files that lack the ncgo:managed marker")
	f.BoolVar(&opts.dryRun, "dry-run", false, "Report intended actions without writing files")
	return cmd
}

func runAISync(cmd *cobra.Command, opts *aiSyncOptions) error {
	res, err := ai.Sync(ai.Options{
		Root:   opts.root,
		Lang:   opts.lang,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	for _, p := range res.Written {
		fmt.Fprintf(out, "wrote %s\n", p)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(out, "skipped %s (%s)\n", s.Path, s.Reason)
	}
	if len(res.Written) == 0 && len(res.Skipped) == 0 {
		fmt.Fprintln(out, "(nothing to do)")
	}
	return nil
}

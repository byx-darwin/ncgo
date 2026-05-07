package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/ai"
)

func newAICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI collaboration helpers (sync generated facts, bootstrap .claude, etc.)",
	}
	cmd.AddCommand(newAISyncCmd(), newAIInitCmd())
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
		Short: "Render AGENTS.md, CLAUDE.md, .claude/generated/project-context.md, and Cursor rules",
		Long: "Generate the AI collaboration artifacts described in docs/prd.md §6 from " +
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

type aiInitClaudeOptions struct {
	root   string
	preset string
	force  bool
	dryRun bool
}

func newAIInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap hand-authored AI starter files",
	}
	cmd.AddCommand(newAIInitClaudeCmd())
	return cmd
}

func newAIInitClaudeCmd() *cobra.Command {
	opts := &aiInitClaudeOptions{}
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Bootstrap the .claude starter set",
		Long: "Create the hand-authored `.claude` starter files for a repository. " +
			"Use `--preset minimal|team` to control the starter set. Existing files are skipped unless --force is set. Generated project facts still " +
			"belong to `ncgo ai sync`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIInitClaude(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Repository root where .claude/ should be bootstrapped")
	f.StringVar(&opts.preset, "preset", ai.InitPresetMinimal, "Starter preset: minimal | team")
	f.BoolVar(&opts.force, "force", false, "Overwrite existing starter files")
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
	printAIResult(cmd, res)
	return nil
}

func runAIInitClaude(cmd *cobra.Command, opts *aiInitClaudeOptions) error {
	res, err := ai.InitClaude(ai.InitOptions{
		Root:   opts.root,
		Preset: opts.preset,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	printAIResult(cmd, res)
	if !opts.dryRun {
		root := opts.root
		if root == "" {
			root = "."
		}
		fmt.Fprintf(cmd.OutOrStdout(), "next: run ncgo ai sync --root %s --lang en\n", root)
	}
	return nil
}

func printAIResult(cmd *cobra.Command, res *ai.Result) {
	out := cmd.OutOrStdout()
	for _, note := range res.Notes {
		fmt.Fprintf(out, "info: %s\n", note)
	}
	for _, p := range res.Written {
		fmt.Fprintf(out, "wrote %s\n", p)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(out, "skipped %s (%s)\n", s.Path, s.Reason)
	}
	if len(res.Written) == 0 && len(res.Skipped) == 0 {
		fmt.Fprintln(out, "(nothing to do)")
	}
}

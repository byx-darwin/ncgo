package cli

import (
	"encoding/json"
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
	output string
}

func newAISyncCmd() *cobra.Command {
	opts := &aiSyncOptions{}
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Render AGENTS.md, CLAUDE.md, .claude/generated/project-context.md, and Cursor rules",
		Long: "Generate the AI collaboration artifacts described in docs/prd.md §6 from " +
			"a service manifest (`.ncgo/manifest.yaml`) or micro workspace metadata (`ncgo.workspace`) and the embedded ncgo design doc. Existing files without " +
			"the `<!-- ncgo:managed -->` marker are skipped unless --force is set.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAISync(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Service root with .ncgo/manifest.yaml or micro workspace root with ncgo.workspace")
	f.StringVar(&opts.lang, "lang", ai.LangEN, "Design-doc language: en | zh-CN")
	f.BoolVar(&opts.force, "force", false, "Overwrite files that lack the ncgo:managed marker")
	f.BoolVar(&opts.dryRun, "dry-run", false, "Report intended actions without writing files")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

type aiInitClaudeOptions struct {
	root   string
	preset string
	force  bool
	dryRun bool
	output string
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
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runAISync(cmd *cobra.Command, opts *aiSyncOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return fmt.Errorf("ai sync: unsupported --output %q; want text or json", opts.output)
	}
	res, err := ai.Sync(ai.Options{
		Root:   opts.root,
		Lang:   opts.lang,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	if opts.output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	printAIResult(cmd, res)
	return nil
}

func runAIInitClaude(cmd *cobra.Command, opts *aiInitClaudeOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return fmt.Errorf("ai init claude: unsupported --output %q; want text or json", opts.output)
	}
	res, err := ai.InitClaude(ai.InitOptions{
		Root:   opts.root,
		Preset: opts.preset,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	if opts.output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	printAIResult(cmd, res)
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
	for _, step := range res.NextSteps {
		fmt.Fprintf(out, "next: %s\n", step)
	}
	if len(res.Written) == 0 && len(res.Skipped) == 0 {
		fmt.Fprintln(out, "(nothing to do)")
	}
}

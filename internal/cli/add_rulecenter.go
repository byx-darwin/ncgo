package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/scaffold/rulecenter"
)

type addRuleCenterOptions struct {
	root   string
	addr   string
	force  bool
	dryRun bool
	plan   bool
	output string
}

func newAddRuleCenterCmd() *cobra.Command {
	opts := &addRuleCenterOptions{}
	cmd := &cobra.Command{
		Use:   "rule-center",
		Short: "Add rule-center gRPC client for rate-limit rule queries",
		Long: "Generate a gRPC client that connects to a rule-center service " +
			"for rate-limit rule queries and wire it into the Hertz rate-limit " +
			"middleware. Requires a running rule-center Kitex service.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddRuleCenter(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.addr, "addr", "", "Rule-center gRPC address (e.g., localhost:8888)")
	f.BoolVar(&opts.force, "force", false, "Overwrite existing generated files")
	f.BoolVar(&opts.dryRun, "dry-run", false, "Preview intended writes without modifying files")
	f.BoolVar(&opts.plan, "plan", false, "Shorthand for --dry-run --output json")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	_ = cmd.MarkFlagRequired("addr")
	return cmd
}

func runAddRuleCenter(cmd *cobra.Command, opts *addRuleCenterOptions) error {
	if opts.plan {
		opts.dryRun = true
		opts.output = "json"
	}
	if err := validateAddOutput("add rule-center", opts.output); err != nil {
		return err
	}
	res, err := rulecenter.Add(rulecenter.Options{
		Root:   opts.root,
		Addr:   opts.addr,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if opts.output == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	writeVerb := "wrote"
	if res.DryRun {
		writeVerb = "would write"
	}
	for _, p := range res.WrittenPaths {
		fmt.Fprintf(out, "%s %s\n", writeVerb, p)
	}
	if res.DryRun {
		fmt.Fprintln(out, "(dry-run: no files were written)")
	}
	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range res.NextSteps {
		fmt.Fprintf(out, "  - %s\n", s)
	}
	return nil
}

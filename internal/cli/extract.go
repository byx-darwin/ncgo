package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/extract"
)

type extractDomainOptions struct {
	root   string
	to     string
	asJSON bool
	apply  bool
}

func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "extract", Short: "Plan or perform mono-to-micro extractions"}
	cmd.AddCommand(newExtractDomainCmd())
	return cmd
}

func newExtractDomainCmd() *cobra.Command {
	opts := &extractDomainOptions{}
	cmd := &cobra.Command{
		Use:   "domain <name>",
		Short: "Plan or apply extraction of a mono domain into a micro RPC service",
		Long: "Validate a mono domain and print the files/imports that need to move. " +
			"With --apply, copy the planned files into an existing Kitex target service and rewrite domain-local imports.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtractDomain(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Mono project root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.to, "to", "", "Target service directory relative to root; default services/<name>-rpc")
	f.BoolVar(&opts.asJSON, "json", false, "Emit machine-readable extraction plan")
	f.BoolVar(&opts.apply, "apply", false, "Copy planned files into the existing target Kitex service")
	return cmd
}

func runExtractDomain(cmd *cobra.Command, name string, opts *extractDomainOptions) error {
	var (
		plan *extract.DomainPlan
		err  error
	)
	domainOpts := extract.DomainOptions{Root: opts.root, Name: name, To: opts.to}
	if opts.apply {
		plan, err = extract.ApplyDomain(domainOpts)
	} else {
		plan, err = extract.PlanDomain(domainOpts)
	}
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if opts.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}
	if plan.Applied {
		fmt.Fprintf(out, "applied extraction for domain %s -> %s\n", plan.Name, plan.To)
	} else {
		fmt.Fprintf(out, "planned extraction for domain %s -> %s\n", plan.Name, plan.To)
	}
	fmt.Fprintf(out, "target module: %s\n\n", plan.TargetModule)
	fmt.Fprintln(out, "files:")
	for _, f := range plan.Sources {
		fmt.Fprintf(out, "  - [%s] %s -> %s\n", f.Role, f.From, f.To)
	}
	if len(plan.Written) > 0 {
		fmt.Fprintln(out, "\nwritten:")
		for _, p := range plan.Written {
			fmt.Fprintf(out, "  - %s\n", p)
		}
	}
	fmt.Fprintln(out, "\nnext steps:")
	for _, step := range plan.NextSteps {
		fmt.Fprintf(out, "  - %s\n", step)
	}
	return nil
}

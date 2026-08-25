package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/scaffold/kitexclient"
)

type addKitexClientOptions struct {
	root    string
	module  string
	service string
	idl     string
	force   bool
	dryRun  bool
	plan    bool
	output  string
}

func newAddKitexClientCmd() *cobra.Command {
	opts := &addKitexClientOptions{}
	cmd := &cobra.Command{
		Use:   "kitex-client <name>",
		Short: "Add Kitex client wrapper in an RPC service",
		Long:  "Generate a Kitex client wrapper and kitex_gen/ types in the RPC service that owns the proto. Run this from the RPC service directory — not a BFF.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddKitexClient(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root")
	f.StringVar(&opts.module, "module", "", "Go module path (auto-detected from go.mod when empty)")
	f.StringVar(&opts.service, "service", "", "RPC service name")
	f.StringVar(&opts.idl, "idl", "", "Path to proto file")
	f.BoolVar(&opts.force, "force", false, "Overwrite existing files")
	f.BoolVar(&opts.dryRun, "dry-run", false, "Preview without writing")
	f.BoolVar(&opts.plan, "plan", false, "Shorthand for --dry-run --output json")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	_ = cmd.MarkFlagRequired("service")
	_ = cmd.MarkFlagRequired("idl")
	return cmd
}

func runAddKitexClient(cmd *cobra.Command, name string, opts *addKitexClientOptions) error {
	if opts.plan {
		opts.dryRun = true
		opts.output = "json"
	}
	if err := validateAddOutput("add kitex-client", opts.output); err != nil {
		return err
	}
	res, err := kitexclient.Add(cmd.Context(), kitexclient.Options{
		Root:    opts.root,
		Name:    name,
		Service: opts.service,
		IDL:     opts.idl,
		Module:  opts.module,
		Force:   opts.force,
		DryRun:  opts.dryRun,
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
	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/scaffold/bff"
	"github.com/byx-darwin/ncgo/internal/scaffold/domain"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/method"
	"github.com/byx-darwin/ncgo/internal/scaffold/rpc"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a feature (infra/domain/rpc/bff) to an existing ncgo project",
	}
	cmd.AddCommand(newAddInfraCmd())
	cmd.AddCommand(newAddDomainCmd())
	cmd.AddCommand(newAddRPCCmd())
	cmd.AddCommand(newAddBFFCmd())
	cmd.AddCommand(newAddMethodCmd())
	return cmd
}

type addInfraOptions struct {
	root   string
	force  bool
	wire   bool
	dryRun bool
	output string
}

func newAddInfraCmd() *cobra.Command {
	opts := &addInfraOptions{}
	cmd := &cobra.Command{
		Use:   "infra <kind>",
		Short: "Install an optional infrastructure add-on",
		Long: "Copy the embedded nc-skills add-on for <kind> into the appropriate " +
			"internal/base package and record it in .ncgo/manifest.yaml. Supported: " +
			strings.Join(infra.SupportedKinds(), ", ") + ".",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddInfra(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing .ncgo/manifest.yaml")
	f.BoolVar(&opts.force, "force", false, "Overwrite existing generated add-on file")
	f.BoolVar(&opts.wire, "wire", false, "Opt-in: update generated server/client wiring when supported")
	f.BoolVar(&opts.dryRun, "dry-run", false, "Preview intended add-on writes and --wire changes without modifying files")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runAddInfra(cmd *cobra.Command, kind string, opts *addInfraOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return fmt.Errorf("add infra: unsupported --output %q; want text or json", opts.output)
	}
	res, err := infra.Add(infra.Options{
		Root:   opts.root,
		Kind:   kind,
		Force:  opts.force,
		Wire:   opts.wire,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if opts.output == "json" {
		return writeAddInfraJSON(out, res)
	}
	writeVerb, wireVerb := "wrote", "wired"
	if res.DryRun {
		writeVerb, wireVerb = "would write", "would wire"
	}
	for _, p := range writtenInfraPaths(res) {
		fmt.Fprintf(out, "%s %s\n", writeVerb, p)
	}
	for _, p := range res.WiredPaths {
		fmt.Fprintf(out, "%s %s\n", wireVerb, p)
	}
	if res.DryRun && res.Updated {
		fmt.Fprintln(out, "(dry-run: manifest would be updated)")
	} else if !res.Updated {
		fmt.Fprintln(out, "(manifest already lists this infra)")
	}
	if res.DryRun {
		fmt.Fprintln(out, "(dry-run: no files were written)")
	}
	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range res.NextSteps {
		fmt.Fprintf(out, "  $ %s\n", s)
	}
	return nil
}

type addInfraJSONResult struct {
	DryRun       bool             `json:"dryRun"`
	Updated      bool             `json:"updated"`
	WrittenPath  string           `json:"writtenPath,omitempty"`
	WrittenPaths []string         `json:"writtenPaths"`
	WiredPaths   []string         `json:"wiredPaths"`
	NextSteps    []string         `json:"nextSteps"`
	Plan         []infra.PlanItem `json:"plan"`
}

func writeAddInfraJSON(out io.Writer, res *infra.Result) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(addInfraJSONResult{
		DryRun:       res.DryRun,
		Updated:      res.Updated,
		WrittenPath:  res.WrittenPath,
		WrittenPaths: res.WrittenPaths,
		WiredPaths:   res.WiredPaths,
		NextSteps:    res.NextSteps,
		Plan:         res.Plan,
	})
}

func writtenInfraPaths(res *infra.Result) []string {
	if len(res.WrittenPaths) > 0 {
		return res.WrittenPaths
	}
	return []string{res.WrittenPath}
}

type addDomainOptions struct {
	root  string
	force bool
}

func newAddDomainCmd() *cobra.Command {
	opts := &addDomainOptions{}
	cmd := &cobra.Command{
		Use:   "domain <name>",
		Short: "Generate a domain (usecase + repository + DI register)",
		Long: "Create internal/usecase/<name>/, internal/repository/<name>/, and " +
			"internal/base/data/<name>_register.go, and record the domain in " +
			".ncgo/manifest.yaml. Names must match ^[a-z][a-z0-9_]{0,62}$.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddDomain(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing .ncgo/manifest.yaml")
	f.BoolVar(&opts.force, "force", false, "Overwrite existing generated files")
	return cmd
}

func runAddDomain(cmd *cobra.Command, name string, opts *addDomainOptions) error {
	res, err := domain.Add(domain.Options{
		Root:  opts.root,
		Name:  name,
		Force: opts.force,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	for _, p := range res.WrittenPaths {
		fmt.Fprintf(out, "wrote %s\n", p)
	}
	if !res.Updated {
		fmt.Fprintln(out, "(manifest already lists this domain)")
	}
	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range res.NextSteps {
		fmt.Fprintf(out, "  - %s\n", s)
	}
	return nil
}

type addRPCOptions struct {
	root       string
	module     string
	dir        string
	noGenerate bool
}

func newAddRPCCmd() *cobra.Command {
	opts := &addRPCOptions{}
	cmd := &cobra.Command{
		Use:   "rpc <name>",
		Short: "Generate a Kitex RPC service in a micro workspace",
		Long: "Create services/<name>/ by reusing the Kitex mono scaffold, then " +
			"record the service in the root ncgo.workspace. By default the service " +
			"module is <workspace.module>/services/<name>.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddRPC(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Micro workspace root containing ncgo.workspace")
	f.StringVar(&opts.module, "module", "", "Go module path for the RPC service; default <workspace.module>/<service dir>")
	f.StringVar(&opts.dir, "dir", "", "Service directory relative to root; default services/<name>")
	f.BoolVar(&opts.noGenerate, "no-generate", false, "Skip kitex invocation; only write service manifest + template/ + idl placeholder")
	return cmd
}

func runAddRPC(cmd *cobra.Command, name string, opts *addRPCOptions) error {
	res, err := rpc.Add(cmd.Context(), rpc.Options{
		Root:          opts.root,
		Name:          name,
		Module:        opts.module,
		Dir:           opts.dir,
		AssetsVersion: assets.Version(),
		NCGOVersion:   version,
		NoGenerate:    opts.noGenerate,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "wrote %s\n", res.ServiceDir)
	if !res.Updated {
		fmt.Fprintln(out, "(workspace already lists this rpc service)")
	}
	if !res.RanGenerate {
		fmt.Fprintln(out, "(generator not invoked; --no-generate set)")
	}
	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range res.NextSteps {
		fmt.Fprintf(out, "  $ %s\n", s)
	}
	return nil
}

type addBFFOptions struct {
	root       string
	module     string
	dir        string
	noGenerate bool
}

func newAddBFFCmd() *cobra.Command {
	opts := &addBFFOptions{}
	cmd := &cobra.Command{
		Use:   "bff <name>",
		Short: "Generate a Hertz BFF service in a micro workspace",
		Long: "Create services/<name>/ by reusing the Hertz mono scaffold, then " +
			"record the service in the root ncgo.workspace. By default the service " +
			"module is <workspace.module>/services/<name>.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddBFF(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Micro workspace root containing ncgo.workspace")
	f.StringVar(&opts.module, "module", "", "Go module path for the BFF service; default <workspace.module>/<service dir>")
	f.StringVar(&opts.dir, "dir", "", "Service directory relative to root; default services/<name>")
	f.BoolVar(&opts.noGenerate, "no-generate", false, "Skip hz invocation; only write service manifest + template/ + idl placeholder")
	return cmd
}

func runAddBFF(cmd *cobra.Command, name string, opts *addBFFOptions) error {
	res, err := bff.Add(cmd.Context(), bff.Options{
		Root:          opts.root,
		Name:          name,
		Module:        opts.module,
		Dir:           opts.dir,
		AssetsVersion: assets.Version(),
		NCGOVersion:   version,
		NoGenerate:    opts.noGenerate,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "wrote %s\n", res.ServiceDir)
	if !res.Updated {
		fmt.Fprintln(out, "(workspace already lists this bff service)")
	}
	if !res.RanGenerate {
		fmt.Fprintln(out, "(generator not invoked; --no-generate set)")
	}
	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range res.NextSteps {
		fmt.Fprintf(out, "  $ %s\n", s)
	}
	return nil
}

type addMethodOptions struct {
	root  string
	layer string
}

func newAddMethodCmd() *cobra.Command {
	opts := &addMethodOptions{}
	cmd := &cobra.Command{
		Use:   "method <domain.Method>",
		Short: "Insert a method stub at ncgo anchor markers",
		Long: "Insert a method stub between // ncgo:methods:start and " +
			"// ncgo:methods:end in a generated domain file. Currently supports " +
			"--in usecase only.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddMethod(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.layer, "in", method.LayerUsecase, "Target layer: usecase")
	return cmd
}

func runAddMethod(cmd *cobra.Command, spec string, opts *addMethodOptions) error {
	res, err := method.Add(method.Options{Root: opts.root, Spec: spec, Layer: opts.layer})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "inserted %s.%s into %s\n", res.Domain, res.Method, res.Path)
	fmt.Fprintln(out, "\nnext steps:")
	fmt.Fprintln(out, "  - replace the generated stub body with domain logic")
	return nil
}

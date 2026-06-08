package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/bff"
	"github.com/byx-darwin/ncgo/internal/scaffold/domain"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/method"
	planpkg "github.com/byx-darwin/ncgo/internal/scaffold/plan"
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
	cmd.AddCommand(newAddRuleCenterCmd())
	return cmd
}

type addInfraOptions struct {
	root   string
	force  bool
	wire   bool
	dryRun bool
	plan   bool
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
	f.BoolVar(&opts.plan, "plan", false, "Shorthand for --dry-run --output json")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runAddInfra(cmd *cobra.Command, kind string, opts *addInfraOptions) error {
	if opts.plan {
		opts.dryRun = true
		opts.output = "json"
	}
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
		return infra.WriteAddResultJSON(out, res)
	}
	fmt.Fprintln(out, infra.FormatAddResultText(res))
	return nil
}

type addDomainOptions struct {
	root   string
	force  bool
	dryRun bool
	plan   bool
	wire   bool
	output string
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
	f.BoolVar(&opts.dryRun, "dry-run", false, "Preview intended domain writes without modifying files")
	f.BoolVar(&opts.plan, "plan", false, "Shorthand for --dry-run --output json")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runAddDomain(cmd *cobra.Command, name string, opts *addDomainOptions) error {
	if opts.plan {
		opts.dryRun = true
		opts.output = "json"
	}
	if err := validateAddOutput("add domain", opts.output); err != nil {
		return err
	}
	res, err := domain.Add(domain.Options{
		Root:   opts.root,
		Name:   name,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if opts.output == "json" {
		return writeAddDomainJSON(out, res)
	}
	writeVerb := "wrote"
	if res.DryRun {
		writeVerb = "would write"
	}
	for _, p := range res.WrittenPaths {
		fmt.Fprintf(out, "%s %s\n", writeVerb, p)
	}
	if res.DryRun && res.Updated {
		fmt.Fprintln(out, "(dry-run: manifest would be updated)")
	} else if !res.Updated {
		fmt.Fprintln(out, "(manifest already lists this domain)")
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

type addRPCOptions struct {
	root       string
	module     string
	dir        string
	noGenerate bool
	dryRun     bool
	plan       bool
	output     string
	preset     string
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
	f.BoolVar(&opts.dryRun, "dry-run", false, "Preview intended RPC service writes without modifying files")
	f.BoolVar(&opts.plan, "plan", false, "Shorthand for --dry-run --output json")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	f.StringVar(&opts.preset, "preset", "", "Preset template to use (e.g., rule-center)")
	return cmd
}

func runAddRPC(cmd *cobra.Command, name string, opts *addRPCOptions) error {
	if opts.plan {
		opts.dryRun = true
		opts.output = "json"
	}
	if err := validateAddOutput("add rpc", opts.output); err != nil {
		return err
	}
	res, err := rpc.Add(cmd.Context(), rpc.Options{
		Root:          opts.root,
		Name:          name,
		Module:        opts.module,
		Dir:           opts.dir,
		AssetsVersion: assets.Version(),
		NCGOVersion:   Version,
		NoGenerate:    opts.noGenerate,
		DryRun:        opts.dryRun,
		Preset:        opts.preset,
	})
	if err != nil {
		return err
	}
	// Update .claude/ for the newly added service
	if !opts.dryRun {
		_ = ai.WriteServiceClaudeDirs(opts.root, name, manifest.KindKitex)
	}
	out := cmd.OutOrStdout()
	if opts.output == "json" {
		return writeAddServiceJSON(out, res.ServiceDir, res.ServiceRel, res.Module, res.DryRun, res.Updated, res.RanGenerate, res.NextSteps, res.Plan)
	}
	writeVerb := "wrote"
	if res.DryRun {
		writeVerb = "would write"
	}
	fmt.Fprintf(out, "%s %s\n", writeVerb, res.ServiceDir)
	if res.DryRun && res.Updated {
		fmt.Fprintln(out, "(dry-run: workspace would be updated)")
	} else if !res.Updated {
		fmt.Fprintln(out, "(workspace already lists this rpc service)")
	}
	if res.DryRun && opts.noGenerate {
		fmt.Fprintln(out, "(generator not invoked; --no-generate set)")
	} else if res.DryRun {
		fmt.Fprintln(out, "(dry-run: generator would be invoked)")
	} else if !res.RanGenerate {
		fmt.Fprintln(out, "(generator not invoked; --no-generate set)")
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

type addBFFOptions struct {
	root       string
	module     string
	dir        string
	noGenerate bool
	dryRun     bool
	plan       bool
	output     string
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
	f.BoolVar(&opts.dryRun, "dry-run", false, "Preview intended BFF service writes without modifying files")
	f.BoolVar(&opts.plan, "plan", false, "Shorthand for --dry-run --output json")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runAddBFF(cmd *cobra.Command, name string, opts *addBFFOptions) error {
	if opts.plan {
		opts.dryRun = true
		opts.output = "json"
	}
	if err := validateAddOutput("add bff", opts.output); err != nil {
		return err
	}
	res, err := bff.Add(cmd.Context(), bff.Options{
		Root:          opts.root,
		Name:          name,
		Module:        opts.module,
		Dir:           opts.dir,
		AssetsVersion: assets.Version(),
		NCGOVersion:   Version,
		NoGenerate:    opts.noGenerate,
		DryRun:        opts.dryRun,
	})
	if err != nil {
		return err
	}
	// Update .claude/ for the newly added service
	if !opts.dryRun {
		_ = ai.WriteServiceClaudeDirs(opts.root, name, manifest.KindHertz)
	}
	out := cmd.OutOrStdout()
	if opts.output == "json" {
		return writeAddServiceJSON(out, res.ServiceDir, res.ServiceRel, res.Module, res.DryRun, res.Updated, res.RanGenerate, res.NextSteps, res.Plan)
	}
	writeVerb := "wrote"
	if res.DryRun {
		writeVerb = "would write"
	}
	fmt.Fprintf(out, "%s %s\n", writeVerb, res.ServiceDir)
	if res.DryRun && res.Updated {
		fmt.Fprintln(out, "(dry-run: workspace would be updated)")
	} else if !res.Updated {
		fmt.Fprintln(out, "(workspace already lists this bff service)")
	}
	if res.DryRun && opts.noGenerate {
		fmt.Fprintln(out, "(generator not invoked; --no-generate set)")
	} else if res.DryRun {
		fmt.Fprintln(out, "(dry-run: generator would be invoked)")
	} else if !res.RanGenerate {
		fmt.Fprintln(out, "(generator not invoked; --no-generate set)")
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

func validateAddOutput(cmdName, output string) error {
	if output == "" || output == "text" || output == "json" {
		return nil
	}
	return fmt.Errorf("%s: unsupported --output %q; want text or json", cmdName, output)
}

func writeAddDomainJSON(out io.Writer, res *domain.Result) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		DryRun       bool           `json:"dryRun"`
		Updated      bool           `json:"updated"`
		WrittenPaths []string       `json:"writtenPaths"`
		NextSteps    []string       `json:"nextSteps"`
		Plan         []planpkg.Item `json:"plan"`
	}{
		DryRun:       res.DryRun,
		Updated:      res.Updated,
		WrittenPaths: res.WrittenPaths,
		NextSteps:    res.NextSteps,
		Plan:         res.Plan,
	})
}

func writeAddServiceJSON(out io.Writer, serviceDir, serviceRel, module string, dryRun, updated, ranGenerate bool, next []string, plan []planpkg.Item) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		DryRun      bool           `json:"dryRun"`
		Updated     bool           `json:"updated"`
		ServiceDir  string         `json:"serviceDir"`
		ServiceRel  string         `json:"serviceRel"`
		Module      string         `json:"module"`
		RanGenerate bool           `json:"ranGenerate"`
		NextSteps   []string       `json:"nextSteps"`
		Plan        []planpkg.Item `json:"plan"`
	}{
		DryRun:      dryRun,
		Updated:     updated,
		ServiceDir:  serviceDir,
		ServiceRel:  serviceRel,
		Module:      module,
		RanGenerate: ranGenerate,
		NextSteps:   next,
		Plan:        plan,
	})
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

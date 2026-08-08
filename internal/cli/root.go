package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/cli/interactive"
	goexec "github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/micro"
	"github.com/byx-darwin/ncgo/internal/scaffold/mono"
)

var (
	Version      = "0.1.0-dev"
	BuildVersion = "dev"
	BuildTime    = "unknown"
)

func Main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		var ec *exitCodeError
		if errors.As(err, &ec) {
			if ec.msg != "" {
				fmt.Fprintln(os.Stderr, ec.msg)
			}
			os.Exit(ec.code)
		}
		if _, silent := err.(silentErr); !silent {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ncgo",
		Short:         "AI-friendly scaffold for Go microservices",
		Long:          "ncgo generates Hertz/Kitex services aligned with the nc-skills-golang conventions and exposes them to AI agents through CLI and MCP.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newNewCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newAICmd())
	cmd.AddCommand(newI18NCmd())
	cmd.AddCommand(newProtolintCmd())
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newMCPCmd())
	cmd.AddCommand(newUpgradeCmd())
	cmd.AddCommand(newExtractCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newTestCmd())
	cmd.AddCommand(newImportCmd())
	cmd.AddCommand(newCompletionCmd())
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print ncgo, build, and embedded assets versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			buildVersion, buildTime := effectiveBuildInfo(BuildVersion, BuildTime)
			fmt.Fprintln(cmd.OutOrStdout(), versionLine(Version, assets.Version(), buildVersion, buildTime))
			return nil
		},
	}
}

func effectiveBuildInfo(buildVersion, buildTime string) (string, string) {
	settings := readBuildSettings()
	return resolveBuildInfo(buildVersion, buildTime, settings)
}

func readBuildSettings() map[string]string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	return settings
}

func resolveBuildInfo(buildVersion, buildTime string, settings map[string]string) (string, string) {
	if isDefaultBuildVersion(buildVersion) {
		if rev := settings["vcs.revision"]; rev != "" {
			buildVersion = shortRevision(rev)
			if settings["vcs.modified"] == "true" {
				buildVersion += "-dirty"
			}
		}
	}
	if isDefaultBuildTime(buildTime) {
		if t := settings["vcs.time"]; t != "" {
			buildTime = t
		}
	}
	return buildVersion, buildTime
}

func isDefaultBuildVersion(value string) bool {
	return value == "" || value == "dev" || value == "unknown"
}

func isDefaultBuildTime(value string) bool {
	return value == "" || value == "unknown"
}

func shortRevision(rev string) string {
	rev = strings.TrimSpace(rev)
	if len(rev) <= 7 {
		return rev
	}
	return rev[:7]
}

func versionLine(ncgoVersion, assetsVersion, buildVersion, buildTime string) string {
	return fmt.Sprintf("ncgo %s (build: %s, built: %s, assets: %s)", nonEmpty(ncgoVersion, "unknown"), nonEmpty(buildVersion, "unknown"), nonEmpty(buildTime, "unknown"), nonEmpty(assetsVersion, "unknown"))
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

type newOptions struct {
	module         string
	mode           string
	kind           string
	db             string
	infra          []string
	preset         string
	idl            string
	dir            string
	noGenerate     bool
	ruleCenterAddr string // rule-center gRPC address (e.g., rule-center:8888)
	templateDir    string // mono template package directory replacing embedded code templates and the IDL placeholder
}

func newNewCmd() *cobra.Command {
	opts := &newOptions{}
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new service or workspace",
		Long:  "Generate a new Go service or micro workspace aligned with nc-skills-golang. Mono mode writes manifest + template/ + idl placeholder, then calls the matching code generator (hz for hertz, kitex for kitex) unless --no-generate is set. Micro mode writes the root ncgo.workspace and services/ skeleton.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args[0], opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.module, "module", "", "Go module path, e.g. github.com/acme/user-api (required)")
	f.StringVar(&opts.mode, "mode", "mono", "Project mode: mono | micro")
	f.StringVar(&opts.kind, "kind", manifest.KindHertz, "Mono service kind: hertz | kitex")
	f.StringVar(&opts.db, "db", "none", "Mono database: postgres | none")
	f.StringSliceVar(&opts.infra, "infra", nil, "Mono infra add-ons to scaffold at creation time (currently: redis)")
	f.StringVar(&opts.preset, "preset", "", "Mono preset name: rule-center (Kitex with rate-limiting)")
	f.StringVar(&opts.idl, "idl", "", "Mono IDL path, default idl/app/<name>.proto (hertz) or idl/<service>.proto (kitex)")
	f.StringVar(&opts.dir, "dir", "", "Target directory, default ./<name>")
	f.BoolVar(&opts.noGenerate, "no-generate", false, "Mono only: skip the generator invocation; only write manifest + template/ + idl placeholder")
	f.StringVar(&opts.ruleCenterAddr, "rule-center-addr", "", "Rule-center gRPC address for rate-limit rule queries (e.g., localhost:8888)")
	f.StringVar(&opts.templateDir, "template-dir", "", "Mono template package directory replacing embedded code templates and the IDL placeholder")
	return cmd
}

func runNew(cmd *cobra.Command, name string, opts *newOptions) error {
	if opts.module == "" {
		if isTerminal() {
			result, err := interactive.Run(name)
			if err != nil {
				return err
			}
			if result == nil {
				return silentErr("cancelled")
			}
			opts.module = result.Module
			opts.kind = result.Kind
			opts.db = "none"
			if result.WithDB {
				opts.db = "postgres"
			}
		} else {
			return errors.New("--module is required")
		}
	}
	switch opts.mode {
	case manifest.ModeMono:
		return runNewMono(cmd, name, opts)
	case manifest.ModeMicro:
		return runNewMicro(cmd, name, opts)
	default:
		return fmt.Errorf("--mode %q is invalid (mono|micro)", opts.mode)
	}
}

func runNewMono(cmd *cobra.Command, name string, opts *newOptions) error {
	switch opts.kind {
	case manifest.KindHertz, manifest.KindKitex:
	default:
		return fmt.Errorf("--kind %q is invalid (hertz|kitex)", opts.kind)
	}
	switch opts.db {
	case "none", "postgres":
	default:
		return fmt.Errorf("--db %q is invalid (postgres|none)", opts.db)
	}
	if err := preflightTools(cmd.Context(), opts.kind, opts.noGenerate, cmd.OutOrStdout(), cmd.InOrStdin()); err != nil {
		return err
	}
	dir := opts.dir
	if dir == "" {
		dir = filepath.Join(".", name)
	}
	res, err := mono.Generate(cmd.Context(), mono.Options{
		Name:           name,
		Module:         opts.module,
		Kind:           opts.kind,
		Dir:            dir,
		WithDatabase:   opts.db == "postgres",
		Infra:          opts.infra,
		Preset:         opts.preset,
		IDL:            opts.idl,
		RuleCenterAddr: opts.ruleCenterAddr,
		TemplateDir:    opts.templateDir,
		AssetsVersion:  assets.Version(),
		NCGOVersion:    Version,
		NoGenerate:     opts.noGenerate,
	})
	if err != nil {
		var nf *goexec.NotFoundError
		if errors.As(err, &nf) {
			fmt.Fprintf(cmd.ErrOrStderr(), "scaffold prepared at %s but %s is not on PATH.\n", dirOrEmpty(res), nf.Name)
			fmt.Fprintf(cmd.ErrOrStderr(), "install: %s\n", goexec.InstallHint(nf.Name))
			fmt.Fprintln(cmd.ErrOrStderr(), "or rerun with --no-generate to skip the generator step.")
		}
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "scaffolded %s at %s\n", name, res.Dir)
	if !res.RanGenerate {
		fmt.Fprintln(out, "(generator not invoked; --no-generate set)")
	}
	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range res.NextSteps {
		fmt.Fprintf(out, "  $ %s\n", s)
	}
	return nil
}

func runNewMicro(cmd *cobra.Command, name string, opts *newOptions) error {
	if opts.kind != manifest.KindHertz {
		return errors.New("--kind is only supported with --mode mono")
	}
	if opts.idl != "" {
		return errors.New("--idl is only supported with --mode mono")
	}
	if opts.db != "none" {
		return errors.New("--db is only supported with --mode mono")
	}
	if len(opts.infra) > 0 {
		return errors.New("--infra is only supported with --mode mono")
	}
	dir := opts.dir
	if dir == "" {
		dir = filepath.Join(".", name)
	}
	res, err := micro.Generate(micro.Options{
		Name:          name,
		Module:        opts.module,
		Dir:           dir,
		AssetsVersion: assets.Version(),
		NCGOVersion:   Version,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "scaffolded micro workspace %s at %s\n", name, res.Dir)
	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range res.NextSteps {
		fmt.Fprintf(out, "  - %s\n", s)
	}
	return nil
}

func dirOrEmpty(r *mono.Result) string {
	if r == nil {
		return ""
	}
	return r.Dir
}

// toolPreflight describes a missing generator tool.
type toolPreflight struct {
	name       string
	minVersion string
	installCmd string
}

// preflightTools checks whether the required generator tools are on PATH.
// If any are missing, it lists them and asks the user for confirmation to
// auto-install. Returns nil when all tools are present (or user confirmed
// and installation succeeded). Returns an error when the user declines or
// installation fails.
func preflightTools(ctx context.Context, kind string, noGenerate bool, w io.Writer, r io.Reader) error {
	if noGenerate {
		return nil
	}
	return preflightToolsWith(ctx, requiredTools(kind), w, r, goexec.Install)
}

// preflightToolsWith is the testable core of preflightTools that accepts
// an injectable install function.
func preflightToolsWith(ctx context.Context, missing []toolPreflight, w io.Writer, r io.Reader, install func(context.Context, string) error) error {
	if len(missing) == 0 {
		return nil
	}

	fmt.Fprintln(w, "The following generator tools are required but not found on PATH:")
	for _, t := range missing {
		fmt.Fprintf(w, "  • %s (>= %s) — %s\n", t.name, t.minVersion, t.installCmd)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Auto-install missing tools with 'go install'? [Y/n] ")

	answer := readLine(r)
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		fmt.Fprintln(w, "Aborted. Please install the tools manually and rerun.")
		return fmt.Errorf("user declined to install tools")
	}

	for _, t := range missing {
		fmt.Fprintf(w, "Installing %s...\n", t.name)
		if err := install(ctx, t.name); err != nil {
			fmt.Fprintf(w, "Failed to install %s: %v\n", t.name, err)
			fmt.Fprintf(w, "Please install manually: %s\n", t.installCmd)
			return fmt.Errorf("install %s: %w", t.name, err)
		}
		fmt.Fprintf(w, "Successfully installed %s\n", t.name)
	}

	return nil
}

// requiredTools returns the list of missing generator tools for the given kind.
func requiredTools(kind string) []toolPreflight {
	var need []toolPreflight

	switch kind {
	case manifest.KindHertz, "":
		if _, err := goexec.LookPath("hz"); err != nil {
			need = append(need, toolPreflight{
				name:       "hz",
				minVersion: goexec.MinHzVersion,
				installCmd: goexec.InstallHint("hz"),
			})
		}
	case manifest.KindKitex:
		if _, err := goexec.LookPath("kitex"); err != nil {
			need = append(need, toolPreflight{
				name:       "kitex",
				minVersion: goexec.MinKitexVersion,
				installCmd: goexec.InstallHint("kitex"),
			})
		}
	}

	return need
}

// readLine reads a single line from r, stripping the trailing newline.
func readLine(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}

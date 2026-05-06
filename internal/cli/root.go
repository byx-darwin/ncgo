package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/exec"
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
	cmd.AddCommand(newMCPCmd())
	cmd.AddCommand(newUpgradeCmd())
	cmd.AddCommand(newExtractCmd())
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

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

type newOptions struct {
	module     string
	mode       string
	kind       string
	db         string
	idl        string
	dir        string
	noGenerate bool
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
	f.StringVar(&opts.idl, "idl", "", "Mono IDL path, default idl/app/<name>.proto (hertz) or idl/<service>.proto (kitex)")
	f.StringVar(&opts.dir, "dir", "", "Target directory, default ./<name>")
	f.BoolVar(&opts.noGenerate, "no-generate", false, "Mono only: skip the generator invocation; only write manifest + template/ + idl placeholder")
	return cmd
}

func runNew(cmd *cobra.Command, name string, opts *newOptions) error {
	if opts.module == "" {
		return errors.New("--module is required")
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
	dir := opts.dir
	if dir == "" {
		dir = filepath.Join(".", name)
	}
	res, err := mono.Generate(cmd.Context(), mono.Options{
		Name:          name,
		Module:        opts.module,
		Kind:          opts.kind,
		Dir:           dir,
		WithDatabase:  opts.db == "postgres",
		IDL:           opts.idl,
		AssetsVersion: assets.Version(),
		NCGOVersion:   Version,
		NoGenerate:    opts.noGenerate,
	})
	if err != nil {
		var nf *exec.NotFoundError
		if errors.As(err, &nf) {
			fmt.Fprintf(cmd.ErrOrStderr(), "scaffold prepared at %s but %s is not on PATH.\n", dirOrEmpty(res), nf.Name)
			fmt.Fprintf(cmd.ErrOrStderr(), "install: %s\n", exec.InstallHint(nf.Name))
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

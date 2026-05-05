package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/upgrade"
)

type upgradeOptions struct {
	root   string
	dryRun bool
	plan   bool
}

func newUpgradeCmd() *cobra.Command {
	opts := &upgradeOptions{}
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade ncgo project metadata to the current CLI/assets version",
		Long: "Update .ncgo/manifest.yaml or ncgo.workspace version metadata. " +
			"The MVP does not rewrite generated source files.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project or micro workspace root")
	f.BoolVar(&opts.dryRun, "dry-run", false, "Report planned metadata updates without writing files")
	f.BoolVar(&opts.plan, "plan", false, "Print a detailed metadata upgrade plan without writing files")
	return cmd
}

func runUpgrade(cmd *cobra.Command, opts *upgradeOptions) error {
	res, err := upgrade.Run(upgrade.Options{Root: opts.root, NCGOVersion: Version, AssetsVersion: assets.Version(), DryRun: opts.dryRun, Plan: opts.plan})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if res.Plan {
		printUpgradePlan(out, res)
		return nil
	}
	verb := "upgraded"
	if res.DryRun {
		verb = "would upgrade"
	}
	if !res.Changed {
		fmt.Fprintf(out, "%s is already current (%s, assets %s)\n", res.Path, res.NewVersion, res.NewAssets)
		return nil
	}
	fmt.Fprintf(out, "%s %s from ncgo %s/assets %s to ncgo %s/assets %s\n", verb, res.Path, res.OldVersion, res.OldAssets, res.NewVersion, res.NewAssets)
	for _, svc := range res.ServiceUpdates {
		if svc.Changed {
			fmt.Fprintf(out, "%s service %s manifest %s from ncgo %s/assets %s\n", verb, svc.Name, svc.Path, svc.OldVersion, svc.OldAssets)
		}
	}
	return nil
}

func printUpgradePlan(out interface{ Write([]byte) (int, error) }, res *upgrade.Result) {
	fmt.Fprintf(out, "upgrade plan for %s (%s)\n", res.Root, res.Mode)
	printMetaPlan(out, "metadata", res.Path, res.Changed, res.OldVersion, res.NewVersion, res.OldAssets, res.NewAssets)
	for _, svc := range res.ServiceUpdates {
		label := "service " + svc.Name
		printMetaPlan(out, label, svc.Path, svc.Changed, svc.OldVersion, svc.NewVersion, svc.OldAssets, svc.NewAssets)
	}
	if res.Changed {
		fmt.Fprintln(out, "next step: rerun without --plan to write metadata updates")
	} else {
		fmt.Fprintln(out, "next step: no metadata updates needed")
	}
}

func printMetaPlan(out interface{ Write([]byte) (int, error) }, label, path string, changed bool, oldVersion, newVersion, oldAssets, newAssets string) {
	status := "unchanged"
	if changed {
		status = "change"
	}
	fmt.Fprintf(out, "  - [%s] %s %s\n", status, label, path)
	fmt.Fprintf(out, "      ncgo: %s -> %s\n", oldVersion, newVersion)
	fmt.Fprintf(out, "      assets: %s -> %s\n", oldAssets, newAssets)
}

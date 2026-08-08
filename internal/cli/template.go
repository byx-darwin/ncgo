package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	goexec "github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/registry"
)

type templateOptions struct {
	registry string
}

func newTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage official template registry packages",
	}
	cmd.AddCommand(newTemplateListCmd(), newTemplatePullCmd())
	return cmd
}

func newTemplateListCmd() *cobra.Command {
	opts := &templateOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List template packages available in the registry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTemplateList(cmd, opts, nil)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.registry, "registry", "", "Template registry URL (default: NCGO_REGISTRY env or official registry)")
	return cmd
}

func newTemplatePullCmd() *cobra.Command {
	opts := &templateOptions{}
	cmd := &cobra.Command{
		Use:   "pull <name>",
		Short: "Fetch a template package into the local registry cache",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTemplatePull(cmd, args[0], opts, nil)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.registry, "registry", "", "Template registry URL (default: NCGO_REGISTRY env or official registry)")
	return cmd
}

// runTemplateList prints one line per registry entry as name<TAB>kind<TAB>description.
func runTemplateList(cmd *cobra.Command, opts *templateOptions, client *registry.Client) error {
	if client == nil {
		client = registry.NewClient(registry.ResolveURL(opts.registry), goexec.NewDefault())
	}
	entries, err := client.List(cmd.Context())
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(entries) == 0 {
		fmt.Fprintln(out, "no templates in registry")
		return nil
	}
	for _, e := range entries {
		fmt.Fprintf(out, "%s\t%s\t%s\n", e.Name, e.Kind, e.Description)
	}
	return nil
}

// runTemplatePull fetches the named template package into the registry cache.
func runTemplatePull(cmd *cobra.Command, name string, opts *templateOptions, client *registry.Client) error {
	if client == nil {
		client = registry.NewClient(registry.ResolveURL(opts.registry), goexec.NewDefault())
	}
	dir, err := client.Pull(cmd.Context(), name)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "pulled %s -> %s\n", name, dir)
	return nil
}

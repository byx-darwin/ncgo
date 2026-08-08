package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/template"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export code templates from an existing ncgo project",
	}
	cmd.AddCommand(newExportTemplatesCmd())
	return cmd
}

type exportTemplatesOptions struct {
	root string
	kind string
}

func newExportTemplatesCmd() *cobra.Command {
	opts := &exportTemplatesOptions{}
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Export code templates from an existing ncgo project",
		Long: "Scan the current project for ncgo-managed source files, replace " +
			"module paths and service names with template variables, and write " +
			"YAML templates to template/<kind>-template/. The output is compatible " +
			"with the respective code generator (kitex -template-dir or hertz overlay).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExportTemplates(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.kind, "kind", "", "Service kind: hertz | kitex (default: read from manifest)")
	return cmd
}

func runExportTemplates(cmd *cobra.Command, opts *exportTemplatesOptions) error {
	m, err := manifest.Load(opts.root)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	kind := opts.kind
	if kind == "" {
		kind = m.Service.Kind
	}
	switch kind {
	case manifest.KindHertz, manifest.KindKitex:
	default:
		return fmt.Errorf("--kind %q is invalid (hertz|kitex)", kind)
	}

	result, err := template.Export(template.ExportOptions{
		Root:        opts.root,
		Kind:        kind,
		Module:      m.Module,
		ServiceName: m.Service.Name,
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(result.IDLs) > 0 {
		fmt.Fprintf(out, "exported %d templates and %d IDL files to %s/\n", len(result.Templates), len(result.IDLs), result.OutputDir)
	} else {
		fmt.Fprintf(out, "exported %d templates to %s/\n", len(result.Templates), result.OutputDir)
	}
	for _, t := range result.Templates {
		fmt.Fprintf(out, "  - %s\n", t)
	}
	for _, f := range result.IDLs {
		fmt.Fprintf(out, "  - %s\n", f)
	}
	return nil
}

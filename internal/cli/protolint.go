package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/protolint"
)

type protolintOptions struct {
	root        string
	files       []string
	rules       []string
	ignoreRules []string
	ignoreFiles []string
	output      string
}

func newProtolintCmd() *cobra.Command {
	opts := &protolintOptions{}
	cmd := &cobra.Command{
		Use:   "protolint",
		Short: "Lint .proto files with ncgo's Proto I/O rules",
		Long: "Compile and lint selected .proto files using ncgo's built-in Proto I/O rules. " +
			"Use --output json or --output sarif for structured diagnostics that can be consumed by AI agents or CI.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProtolint(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Import root used to resolve the target proto files")
	f.StringArrayVar(&opts.files, "file", nil, "Proto file to lint, relative to --root; repeat for multiple entry files. If omitted, ncgo will try to discover proto targets from the root manifest or workspace")
	f.StringArrayVar(&opts.rules, "rule", nil, "Rule ID to run, e.g. PIO201; repeat for multiple rules")
	f.StringArrayVar(&opts.ignoreRules, "ignore-rule", nil, "Rule ID to suppress from diagnostics, e.g. PIO212; repeat for multiple rules")
	f.StringArrayVar(&opts.ignoreFiles, "ignore-file", nil, "Proto file whose diagnostics should be suppressed, relative to --root; repeat for multiple files")
	f.StringVar(&opts.output, "output", "text", "Output format: text, json, or sarif")
	return cmd
}

func runProtolint(cmd *cobra.Command, opts *protolintOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" && opts.output != "sarif" {
		return fmt.Errorf("protolint: unsupported --output %q; want text, json, or sarif", opts.output)
	}
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	res, err := protolint.Run(ctx, protolint.RunOptions{
		Root:          root,
		Files:         opts.files,
		RuleIDs:       opts.rules,
		IgnoreRuleIDs: opts.ignoreRules,
		IgnoreFiles:   opts.ignoreFiles,
	})
	if err != nil {
		return err
	}
	res.Root = root
	switch opts.output {
	case "json":
		if err := writeProtolintJSON(cmd.OutOrStdout(), res); err != nil {
			return err
		}
	case "sarif":
		if err := writeProtolintSARIF(cmd.OutOrStdout(), res); err != nil {
			return err
		}
	default:
		if err := writeProtolintText(cmd.OutOrStdout(), res); err != nil {
			return err
		}
	}
	if !res.OK {
		return errSilentFailure
	}
	return nil
}

func writeProtolintJSON(out io.Writer, res *protolint.Result) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func writeProtolintText(out io.Writer, res *protolint.Result) error {
	return protolint.WriteText(out, res)
}

func writeProtolintSARIF(out io.Writer, res *protolint.Result) error {
	return protolint.WriteSARIF(out, res)
}

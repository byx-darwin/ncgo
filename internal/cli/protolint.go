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
	root   string
	files  []string
	rules  []string
	output string
}

func newProtolintCmd() *cobra.Command {
	opts := &protolintOptions{}
	cmd := &cobra.Command{
		Use:   "protolint",
		Short: "Lint .proto files with ncgo's Proto I/O rules",
		Long: "Compile and lint selected .proto files using ncgo's built-in Proto I/O rules. " +
			"Use --output json for structured diagnostics that can be consumed by AI agents or CI.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProtolint(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Import root used to resolve the target proto files")
	f.StringArrayVar(&opts.files, "file", nil, "Proto file to lint, relative to --root; repeat for multiple entry files")
	f.StringArrayVar(&opts.rules, "rule", nil, "Rule ID to run, e.g. PIO201; repeat for multiple rules")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runProtolint(cmd *cobra.Command, opts *protolintOptions) error {
	if len(opts.files) == 0 {
		return fmt.Errorf("protolint: at least one --file is required")
	}
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return fmt.Errorf("protolint: unsupported --output %q; want text or json", opts.output)
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
		Root:    root,
		Files:   opts.files,
		RuleIDs: opts.rules,
	})
	if err != nil {
		return err
	}
	res.Root = root
	if opts.output == "json" {
		if err := writeProtolintJSON(cmd.OutOrStdout(), res); err != nil {
			return err
		}
	} else {
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

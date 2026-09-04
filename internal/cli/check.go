package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

type checkOptions struct {
	root   string
	output string
}

// exitCodeError lets a command choose its process exit code explicitly.
type exitCodeError struct {
	code int
	msg  string
}

func (e *exitCodeError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("ncgo check: exited with code %d", e.code)
}

func newCheckCmd() *cobra.Command {
	opts := &checkOptions{}
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate AI context integrity and manifest consistency",
		Long: "Verify that every usecase has paired // ncgo:methods anchors, that " +
			"manifest domains match internal/usecase/*/ directories, and that rendered " +
			"AI context files' declared domains match the manifest. Exits 0 on pass, 1 on " +
			"check failure, 2 on command error (e.g. root is not an ncgo service).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Service root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runCheck(cmd *cobra.Command, opts *checkOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return &exitCodeError{code: 2, msg: fmt.Sprintf("check: unsupported --output %q; want text or json", opts.output)}
	}
	rep, err := doctor.RunCheck(opts.root)
	if err != nil {
		return &exitCodeError{code: 2, msg: err.Error()}
	}
	switch opts.output {
	case "json":
		err = doctor.WriteJSON(cmd.OutOrStdout(), rep)
	default:
		err = doctor.WriteText(cmd.OutOrStdout(), rep)
	}
	if err != nil {
		return &exitCodeError{code: 2, msg: err.Error()}
	}
	if !rep.OK() {
		return &exitCodeError{code: 1}
	}
	return nil
}

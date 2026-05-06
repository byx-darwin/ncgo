package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

type doctorOptions struct {
	root   string
	asJSON bool
	output string
}

var runDoctorReport = doctor.Run

func newDoctorCmd() *cobra.Command {
	opts := &doctorOptions{}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose host tools and the current ncgo project",
		Long: "Verify hz/kitex are on PATH at supported versions, then (if run inside a project) " +
			"check that .ncgo/manifest.yaml loads, template/data.json agrees with it, and manifest.service.idl passes the default proto lint checks. " +
			"Use --output json or --output sarif for structured results consumed by AI agents, CI, or code scanning tools. --json remains as a compatibility alias for --output json.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root to inspect; pass '' to skip project checks")
	f.BoolVar(&opts.asJSON, "json", false, "Emit machine-readable JSON (compatibility alias for --output json)")
	f.StringVar(&opts.output, "output", "text", "Output format: text, json, or sarif")
	return cmd
}

func runDoctor(cmd *cobra.Command, opts *doctorOptions) error {
	output, err := resolveDoctorOutput(opts)
	if err != nil {
		return err
	}
	rep := runDoctorReport(cmd.Context(), doctor.Options{Root: opts.root})
	switch output {
	case "json":
		err = doctor.WriteJSON(cmd.OutOrStdout(), rep)
	case "sarif":
		err = doctor.WriteSARIF(cmd.OutOrStdout(), rep)
	default:
		err = doctor.WriteText(cmd.OutOrStdout(), rep)
	}
	if err != nil {
		return err
	}
	if !rep.OK() {
		return errSilentFailure
	}
	return nil
}

func resolveDoctorOutput(opts *doctorOptions) (string, error) {
	output := strings.TrimSpace(opts.output)
	if output == "" {
		output = "text"
	}
	if opts.asJSON {
		switch output {
		case "", "text", "json":
			output = "json"
		default:
			return "", fmt.Errorf("doctor: --json cannot be combined with --output %q", output)
		}
	}
	if output != "text" && output != "json" && output != "sarif" {
		return "", fmt.Errorf("doctor: unsupported --output %q; want text, json, or sarif", output)
	}
	return output, nil
}

// errSilentFailure signals a non-zero exit without re-printing the error to
// stderr; the report itself already explains every failure.
var errSilentFailure = silentErr("doctor: one or more checks failed")

type silentErr string

func (e silentErr) Error() string { return string(e) }

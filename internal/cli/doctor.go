package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

type doctorOptions struct {
	root   string
	asJSON bool
}

func newDoctorCmd() *cobra.Command {
	opts := &doctorOptions{}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose host tools and the current ncgo project",
		Long: "Verify hz/kitex are on PATH at supported versions, then (if run inside a project) " +
			"check that .ncgo/manifest.yaml loads and template/data.json agrees with it. " +
			"Use --json to emit the structured report consumed by AI agents and CI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root to inspect; pass '' to skip project checks")
	f.BoolVar(&opts.asJSON, "json", false, "Emit machine-readable JSON instead of the human report")
	return cmd
}

func runDoctor(cmd *cobra.Command, opts *doctorOptions) error {
	rep := doctor.Run(cmd.Context(), doctor.Options{Root: opts.root})
	out := cmd.OutOrStdout()
	if opts.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		writeHumanReport(out, rep)
	}
	if !rep.OK() {
		return errSilentFailure
	}
	return nil
}

// errSilentFailure signals a non-zero exit without re-printing the error to
// stderr; the report itself already explains every failure.
var errSilentFailure = silentErr("doctor: one or more checks failed")

type silentErr string

func (e silentErr) Error() string { return string(e) }

func writeHumanReport(w io.Writer, rep *doctor.Report) {
	for _, c := range rep.Checks {
		mark := "✓"
		if !c.OK {
			if c.Severity == doctor.SeverityWarn {
				mark = "!"
			} else {
				mark = "✗"
			}
		}
		fmt.Fprintf(w, "%s [%s] %s\n", mark, c.ID, c.Message)
		if c.Hint != "" && !c.OK {
			fmt.Fprintf(w, "    hint: %s\n", c.Hint)
		}
	}
	if rep.OK() {
		fmt.Fprintln(w, "\nall checks passed")
	} else {
		fmt.Fprintln(w, "\none or more checks failed")
	}
}

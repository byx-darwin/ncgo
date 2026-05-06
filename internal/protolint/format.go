package protolint

import (
	"fmt"
	"io"
)

// WriteText writes a human-readable lint result.
func WriteText(w io.Writer, res *Result) error {
	if res == nil {
		return nil
	}
	for _, d := range res.Diagnostics {
		mark := "✗"
		if d.Level == LevelWarning {
			mark = "!"
		}
		if _, err := fmt.Fprintf(w, "%s [%s] %s:%d:%d %s/%s %s\n", mark, d.RuleID, d.File, d.Line, d.Column, nonEmpty(d.Service, "-"), nonEmpty(d.RPC, "-"), d.Summary); err != nil {
			return err
		}
		if d.Hint != "" {
			if _, err := fmt.Fprintf(w, "    hint: %s\n", d.Hint); err != nil {
				return err
			}
		}
	}
	status := "ok"
	if !res.OK {
		status = "failed"
	}
	_, err := fmt.Fprintf(w, "protolint: %s (files=%d rpcs=%d diagnostics=%d errors=%d warnings=%d rules=%d)\n", status, res.Summary.FilesScanned, res.Summary.RPCsScanned, res.Summary.DiagnosticsCount, res.Summary.ErrorCount, res.Summary.WarningCount, len(res.RulesRun))
	return err
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

package doctor

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON writes the structured doctor report as JSON.
func WriteJSON(w io.Writer, rep *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// WriteText writes a human-readable doctor report.
func WriteText(w io.Writer, rep *Report) error {
	if rep == nil {
		return nil
	}
	for _, c := range rep.Checks {
		mark := "✓"
		if !c.OK {
			if c.Severity == SeverityWarn {
				mark = "!"
			} else {
				mark = "✗"
			}
		}
		if _, err := fmt.Fprintf(w, "%s [%s] %s\n", mark, c.ID, c.Message); err != nil {
			return err
		}
		if c.Hint != "" && !c.OK {
			if _, err := fmt.Fprintf(w, "    hint: %s\n", c.Hint); err != nil {
				return err
			}
		}
	}
	if rep.OK() {
		_, err := fmt.Fprintln(w, "\nall checks passed")
		return err
	}
	_, err := fmt.Fprintln(w, "\none or more checks failed")
	return err
}

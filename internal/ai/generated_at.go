package ai

import (
	"os"
	"strings"
	"time"

	"github.com/byx-darwin/ncgo/internal/scan"
)

// stampGeneratedAt injects a generated-at marker line immediately after the
// managed-marker line in rendered output. Returns rendered unchanged when the
// managed marker is absent (should not happen for ai sync targets).
func stampGeneratedAt(rendered string, ts time.Time) string {
	marker := scan.GeneratedAtMarker + ts.UTC().Format(time.RFC3339) + " -->"
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if strings.Contains(line, ManagedMarker) {
			lines = append(lines[:i+1], append([]string{marker}, lines[i+1:]...)...)
			break
		}
	}
	return strings.Join(lines, "\n")
}

// ReadGeneratedAt parses the `<!-- ncgo:generated-at: <RFC3339> -->` marker
// from a rendered context file. It reports (zero time, false) when the marker
// is absent or malformed.
func ReadGeneratedAt(path string) (time.Time, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, scan.GeneratedAtMarker) {
			continue
		}
		rest := strings.TrimPrefix(line, scan.GeneratedAtMarker)
		rest = strings.TrimSuffix(rest, " -->")
		rest = strings.TrimSuffix(rest, "-->")
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(rest))
		if err != nil {
			return time.Time{}, false
		}
		return ts, true
	}
	return time.Time{}, false
}

package ai

import (
	"strings"
	"testing"
	"time"
)

func TestStampGeneratedAtOnLine2(t *testing.T) {
	input := "<!-- ncgo:managed -->\n\n# Title\n\nContent here.\n"
	got := stampGeneratedAt(input, time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC))
	lines := strings.Split(got, "\n")
	if !strings.Contains(lines[0], "ncgo:managed") {
		t.Errorf("line 0 should be managed marker: %q", lines[0])
	}
	if !strings.Contains(lines[1], "ncgo:generated-at") {
		t.Errorf("line 1 should be generated-at marker: %q", lines[1])
	}
}

package protolint

import (
	"context"
	"path/filepath"
	"testing"
)

// TestGoldenScaffoldProtoLintsClean locks the self-consistency contract
// between the scaffold generators and the lint rules: the default proto a
// fresh `ncgo new` writes must pass ncgo's own protolint through the same
// manifest-driven discovery path doctor uses, with zero error-level
// diagnostics. Warnings (e.g. advisory PGV hints) are allowed.
func TestGoldenScaffoldProtoLintsClean(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "scaffold", "mono", "testdata", "mono-default"))
	res, err := Run(context.Background(), RunOptions{Root: root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var errs []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Level == LevelError {
			errs = append(errs, d)
		}
	}
	for _, d := range errs {
		t.Errorf("error diagnostic %s: %s (%s:%d)", d.RuleID, d.Summary, d.File, d.Line)
	}
	if len(errs) > 0 {
		t.Fatalf("fresh scaffold proto produced %d error-level diagnostics, want 0", len(errs))
	}
}

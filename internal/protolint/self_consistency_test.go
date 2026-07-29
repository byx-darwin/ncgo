package protolint

import (
	"context"
	"path/filepath"
	"testing"
)

// TestGoldenScaffoldProtoLintsClean locks the self-consistency contract
// between the scaffold generators and the lint rules: the default proto a
// fresh `ncgo new` writes must pass ncgo's own protolint through the same
// manifest-driven discovery path doctor uses, with zero error-level AND zero
// warning-level diagnostics. Every Hertz fixture that ships the Ping proto is
// covered so the PIO402 advisory stays cleared on all of them.
func TestGoldenScaffoldProtoLintsClean(t *testing.T) {
	cases := []struct {
		name string
		root string
	}{
		{"mono-default", filepath.Join("..", "scaffold", "mono", "testdata", "mono-default")},
		{"mono-with-database", filepath.Join("..", "scaffold", "mono", "testdata", "mono-with-database")},
		{"mono-with-rulecenter", filepath.Join("..", "scaffold", "mono", "testdata", "mono-with-rulecenter")},
		{"bff-default", filepath.Join("..", "scaffold", "bff", "testdata", "bff-default")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Clean(tc.root)
			res, err := Run(context.Background(), RunOptions{Root: root})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			for _, d := range res.Diagnostics {
				if d.Level == LevelError || d.Level == LevelWarning {
					t.Errorf("%s diagnostic %s: %s (%s:%d field=%s)", d.Level, d.RuleID, d.Summary, d.File, d.Line, d.Field)
				}
			}
			if res.Summary.ErrorCount != 0 {
				t.Errorf("fresh scaffold proto produced %d error-level diagnostics, want 0", res.Summary.ErrorCount)
			}
			if res.Summary.WarningCount != 0 {
				t.Errorf("fresh scaffold proto produced %d warning-level diagnostics, want 0", res.Summary.WarningCount)
			}
		})
	}
}

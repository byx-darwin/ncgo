package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// scriptedRunner returns canned responses keyed on Cmd.Name. Missing keys
// produce a NotFoundError so tests can mix presence and absence per binary.
type scriptedRunner struct {
	out map[string]string
	err map[string]error
}

func (s *scriptedRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	if e, ok := s.err[c.Name]; ok {
		return exec.Result{}, e
	}
	if v, ok := s.out[c.Name]; ok {
		return exec.Result{Stdout: []byte(v)}, nil
	}
	return exec.Result{}, &exec.NotFoundError{Name: c.Name}
}

func findCheck(t *testing.T, r *Report, id string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("check %q not found in report; have: %+v", id, r.Checks)
	return Check{}
}

func TestRunReportsHzKitexOK(t *testing.T) {
	r := Run(context.Background(), Options{Runner: &scriptedRunner{
		out: map[string]string{"hz": "hz version v0.9.7", "kitex": "v0.16.1"},
	}})
	hz := findCheck(t, r, "tool.hz")
	if !hz.OK {
		t.Errorf("hz not OK: %+v", hz)
	}
	kx := findCheck(t, r, "tool.kitex")
	if !kx.OK {
		t.Errorf("kitex not OK: %+v", kx)
	}
	if !r.OK() {
		t.Errorf("report not OK: %+v", r.Checks)
	}
}

func TestRunFailsWhenHzAbsent(t *testing.T) {
	r := Run(context.Background(), Options{Runner: &scriptedRunner{
		out: map[string]string{"kitex": "v0.16.1"},
	}})
	hz := findCheck(t, r, "tool.hz")
	if hz.OK {
		t.Errorf("hz should fail when absent")
	}
	if hz.Severity != SeverityError {
		t.Errorf("hz severity = %s, want error", hz.Severity)
	}
	if !strings.Contains(hz.Hint, "go install github.com/cloudwego/hertz") {
		t.Errorf("missing install hint: %+v", hz)
	}
	if r.OK() {
		t.Errorf("report should not be OK when hz absent")
	}
}

func TestRunFailsWhenHzTooOld(t *testing.T) {
	r := Run(context.Background(), Options{Runner: &scriptedRunner{
		out: map[string]string{"hz": "hz version v0.9.6", "kitex": "v0.16.1"},
	}})
	hz := findCheck(t, r, "tool.hz")
	if hz.OK {
		t.Errorf("hz v0.9.6 should fail >= v0.9.7")
	}
	if !strings.Contains(hz.Message, "below minimum") {
		t.Errorf("expected 'below minimum' in message: %s", hz.Message)
	}
}

func TestRunVersionUnparsableIsWarn(t *testing.T) {
	r := Run(context.Background(), Options{Runner: &scriptedRunner{
		out: map[string]string{"hz": "garbage", "kitex": "v0.16.1"},
	}})
	hz := findCheck(t, r, "tool.hz")
	if hz.OK || hz.Severity != SeverityWarn {
		t.Errorf("expected warn-severity unparsable; got %+v", hz)
	}
	// Warns must not block report.OK().
	if !r.OK() {
		t.Errorf("warn should not flip report.OK to false; checks=%+v", r.Checks)
	}
}

func seedProject(t *testing.T, withDB bool, dataJSON string) string {
	t.Helper()
	root := t.TempDir()
	m := &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: "github.com/x/demo",
		Service: manifest.Service{
			Name: "demo", Kind: manifest.KindHertz, WithDatabase: withDB,
			IDL: "idl/app/demo.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if dataJSON != "" {
		dir := filepath.Join(root, "template")
		_ = os.MkdirAll(dir, 0o755)
		if err := os.WriteFile(filepath.Join(dir, "data.json"), []byte(dataJSON), 0o644); err != nil {
			t.Fatalf("seed data.json: %v", err)
		}
	}
	return root
}

func TestRunProjectChecksHappyPath(t *testing.T) {
	root := seedProject(t, false, `{"*":{"GoModule":"github.com/x/demo","ServiceName":"demo","WithDatabase":false}}`)
	r := Run(context.Background(), Options{
		Root: root,
		Runner: &scriptedRunner{out: map[string]string{
			"hz": "hz version v0.9.7", "kitex": "v0.16.1",
		}},
	})
	for _, id := range []string{"manifest.load", "manifest.data.consistent"} {
		c := findCheck(t, r, id)
		if !c.OK {
			t.Errorf("%s not OK: %+v", id, c)
		}
	}
}

func TestRunDetectsDataDrift(t *testing.T) {
	root := seedProject(t, false, `{"*":{"GoModule":"github.com/wrong/mod","ServiceName":"demo","WithDatabase":false}}`)
	r := Run(context.Background(), Options{Root: root, Runner: &scriptedRunner{out: map[string]string{
		"hz": "hz version v0.9.7", "kitex": "v0.16.1",
	}}})
	c := findCheck(t, r, "manifest.data.consistent")
	if c.OK {
		t.Errorf("expected drift to fail check: %+v", c)
	}
	if c.Severity != SeverityWarn {
		t.Errorf("severity = %s, want warn", c.Severity)
	}
	if !strings.Contains(c.Message, "GoModule") {
		t.Errorf("message missing GoModule diff: %s", c.Message)
	}
}

func TestRunMissingManifestIsError(t *testing.T) {
	root := t.TempDir()
	r := Run(context.Background(), Options{Root: root, Runner: &scriptedRunner{out: map[string]string{
		"hz": "hz version v0.9.7", "kitex": "v0.16.1",
	}}})
	c := findCheck(t, r, "manifest.load")
	if c.OK {
		t.Errorf("expected missing manifest to fail")
	}
	if r.OK() {
		t.Errorf("report should not be OK when manifest missing")
	}
}

func TestSemverCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.9.7", "v0.9.7", 0},
		{"v0.9.6", "v0.9.7", -1},
		{"v0.9.8", "v0.9.7", 1},
		{"v1.0.0", "v0.99.99", 1},
		{"v0.9.7-rc1", "v0.9.7", 0}, // pre-release suffix ignored
	}
	for _, tc := range cases {
		got, err := semverCompare(tc.a, tc.b)
		if err != nil {
			t.Errorf("semverCompare(%q,%q) error: %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("semverCompare(%q,%q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
	if _, err := semverCompare("not-semver", "v1.0.0"); err == nil {
		t.Errorf("expected error for invalid semver")
	}
}

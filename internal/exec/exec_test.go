package exec

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCapturesStdout(t *testing.T) {
	r := NewDefault()
	res, err := r.Run(context.Background(), Cmd{Name: "go", Args: []string{"env", "GOOS"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if got := strings.TrimSpace(string(res.Stdout)); got != runtime.GOOS {
		t.Errorf("stdout = %q, want %q", got, runtime.GOOS)
	}
}

func TestRunHonorsWorkingDir(t *testing.T) {
	r := NewDefault()
	dir := t.TempDir()
	res, err := r.Run(context.Background(), Cmd{Name: "pwd", Dir: dir})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.TrimSpace(string(res.Stdout))
	// macOS resolves /var/folders/... -> /private/var/folders/...; tolerate suffix match.
	if !strings.HasSuffix(got, dir) {
		t.Errorf("pwd = %q, expected suffix %q", got, dir)
	}
}

func TestRunNonZeroExitReturnsExitError(t *testing.T) {
	r := NewDefault()
	res, err := r.Run(context.Background(), Cmd{Name: "go", Args: []string{"env", "--definitely-not-a-flag"}})
	if err == nil {
		t.Fatalf("expected ExitError, got nil; result=%+v", res)
	}
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("error type = %T, want *ExitError", err)
	}
	if ee.Result.ExitCode == 0 {
		t.Errorf("ExitError.ExitCode = 0, want non-zero")
	}
	if len(ee.Result.Stderr) == 0 {
		t.Errorf("expected non-empty stderr in ExitError")
	}
	if !strings.Contains(ee.Error(), "exit ") {
		t.Errorf("error message %q should mention exit code", ee.Error())
	}
}

func TestRunMissingBinary(t *testing.T) {
	r := NewDefault()
	_, err := r.Run(context.Background(), Cmd{Name: "ncgo-this-binary-does-not-exist-xyz"})
	if err == nil {
		t.Fatalf("expected NotFoundError")
	}
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("error type = %T, want *NotFoundError", err)
	}
	if !strings.Contains(nf.Error(), "not found") {
		t.Errorf("error message %q should mention 'not found'", nf.Error())
	}
}

func TestInstallUnknownToolReturnsError(t *testing.T) {
	err := Install(context.Background(), "definitely-not-a-tool")
	if err == nil {
		t.Fatalf("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "no install path") {
		t.Errorf("error = %q, want 'no install path'", err.Error())
	}
}

func TestInstallErrorFormatTrailingColon(t *testing.T) {
	// Install with a deliberately invalid module path.
	// This will fail, and we verify the error message shape.
	err := Install(context.Background(), "hz")
	if err == nil {
		// If hz was already installed and go install succeeds, skip.
		t.Skip("hz installed successfully; cannot test failure format")
	}
	if strings.HasSuffix(err.Error(), ": ") {
		t.Errorf("error has trailing ': ': %q", err.Error())
	}
	if !strings.Contains(err.Error(), "install hz") {
		t.Errorf("error = %q, should mention 'install hz'", err.Error())
	}
}

func TestRunRespectsContextCancel(t *testing.T) {
	r := NewDefault()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := r.Run(ctx, Cmd{Name: "sleep", Args: []string{"5"}})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected error from cancelled context")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Run did not cancel promptly: elapsed=%v", elapsed)
	}
}

func TestRunRejectsEmptyName(t *testing.T) {
	r := NewDefault()
	if _, err := r.Run(context.Background(), Cmd{}); err == nil {
		t.Fatalf("expected error for empty Cmd.Name")
	}
}

func TestTailTrimsAndCollapses(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"  hello\nworld  \n", 100, "hello world"},
		{strings.Repeat("a", 600), 100, "..." + strings.Repeat("a", 100)},
		{"", 10, ""},
	}
	for _, tc := range cases {
		got := tail([]byte(tc.in), tc.n)
		if got != tc.want {
			t.Errorf("tail(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

// fakeRunner verifies the Runner interface is usable for substitution.
type fakeRunner struct {
	last Cmd
	out  Result
	err  error
}

func (f *fakeRunner) Run(_ context.Context, c Cmd) (Result, error) {
	f.last = c
	return f.out, f.err
}

func TestHZAndKitexUseRunner(t *testing.T) {
	f := &fakeRunner{out: Result{Stdout: []byte("ok")}}
	if _, err := HZ(context.Background(), f, "/tmp", "new", "--mod=x"); err != nil {
		t.Fatalf("HZ: %v", err)
	}
	if f.last.Name != "hz" || f.last.Dir != "/tmp" || len(f.last.Args) != 2 {
		t.Errorf("HZ recorded cmd = %+v", f.last)
	}
	if _, err := Kitex(context.Background(), f, "/tmp", "-type", "protobuf"); err != nil {
		t.Fatalf("Kitex: %v", err)
	}
	if f.last.Name != "kitex" {
		t.Errorf("Kitex recorded name = %q", f.last.Name)
	}
}

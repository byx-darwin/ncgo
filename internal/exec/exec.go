// Package exec is a thin wrapper around the os/exec package used by ncgo
// to invoke external code generators such as `hz` and `kitex`.
//
// The wrapper exists so that:
//   - higher-level scaffold logic depends on a Runner interface and can be
//     unit-tested with a fake;
//   - command failures carry a structured error with the tail of stderr,
//     which is what AI agents and humans actually need to debug a generator
//     run;
//   - context cancellation, working directory, and extra env vars are all
//     handled in one place.
package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	osexec "os/exec"
	"strings"
)

// Minimum versions of upstream code generators ncgo expects on PATH. They
// are the source of truth for `ncgo doctor` reports and for the install hints
// printed when `ncgo new` cannot find a binary.
const (
	MinHzVersion    = "v0.9.7"
	MinKitexVersion = "v0.16.1"
)

// InstallHint returns a human/agent-readable suggestion for installing the
// missing binary, or an empty string when name is unknown.
func InstallHint(name string) string {
	switch name {
	case "hz":
		return "go install github.com/cloudwego/hertz/cmd/hz@latest  # >= " + MinHzVersion
	case "kitex":
		return "go install github.com/cloudwego/kitex/tool/cmd/kitex@latest  # >= " + MinKitexVersion
	}
	return ""
}

var installPaths = map[string]string{
	"hz":    "github.com/cloudwego/hertz/cmd/hz@latest",
	"kitex": "github.com/cloudwego/kitex/tool/cmd/kitex@latest",
}

// Install runs `go install <path>@latest` for the named tool. It returns
// an error with the command output if installation fails.
func Install(ctx context.Context, name string) error {
	path := installPaths[name]
	if path == "" {
		return fmt.Errorf("exec: no install path known for %q", name)
	}
	cmd := osexec.CommandContext(ctx, "go", "install", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec: install %s: %w: %s", name, err, bytes.TrimSpace(out))
	}
	return nil
}

// Cmd describes a single external command invocation.
type Cmd struct {
	Name string   // binary name, looked up on PATH (e.g. "hz", "kitex")
	Args []string // arguments
	Dir  string   // working directory; empty means current
	Env  []string // additional env entries appended to os.Environ()
}

// Result carries the captured output of a command. ExitCode is set even on
// success (it will be 0).
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Runner runs a Cmd. The default production implementation shells out;
// tests substitute a fake.
type Runner interface {
	Run(ctx context.Context, c Cmd) (Result, error)
}

// Default is the production Runner backed by os/exec.
type Default struct{}

// NewDefault returns a Default runner.
func NewDefault() *Default { return &Default{} }

// Run executes c and captures stdout/stderr. It returns a Result even on
// failure so the caller can inspect partial output. Non-zero exit codes are
// surfaced as *ExitError.
func (d *Default) Run(ctx context.Context, c Cmd) (Result, error) {
	if c.Name == "" {
		return Result{}, errors.New("exec: Cmd.Name is empty")
	}
	bin, err := osexec.LookPath(c.Name)
	if err != nil {
		return Result{}, &NotFoundError{Name: c.Name, Err: err}
	}
	cmd := osexec.CommandContext(ctx, bin, c.Args...)
	cmd.Dir = c.Dir
	if len(c.Env) > 0 {
		cmd.Env = append(cmd.Environ(), c.Env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runErr != nil {
		var ee *osexec.ExitError
		if errors.As(runErr, &ee) {
			return res, &ExitError{Cmd: c, Result: res, Err: runErr}
		}
		return res, fmt.Errorf("exec: %s: %w", c.Name, runErr)
	}
	return res, nil
}

// LookPath reports whether a binary is available on PATH and returns its
// resolved path. It is a thin proxy over os/exec.LookPath kept here so
// higher layers do not import os/exec directly.
func LookPath(name string) (string, error) { return osexec.LookPath(name) }

// HZ runs the hz CLI from dir with args. It is a convenience over a Runner.
func HZ(ctx context.Context, r Runner, dir string, args ...string) (Result, error) {
	return r.Run(ctx, Cmd{Name: "hz", Args: args, Dir: dir})
}

// Kitex runs the kitex CLI from dir with args.
func Kitex(ctx context.Context, r Runner, dir string, args ...string) (Result, error) {
	return r.Run(ctx, Cmd{Name: "kitex", Args: args, Dir: dir})
}

// NotFoundError is returned when the requested binary is not on PATH.
type NotFoundError struct {
	Name string
	Err  error
}

func (e *NotFoundError) Error() string {
	if hint := InstallHint(e.Name); hint != "" {
		return fmt.Sprintf("exec: %q not found on PATH: %v; install with: %s", e.Name, e.Err, hint)
	}
	return fmt.Sprintf("exec: %q not found on PATH: %v", e.Name, e.Err)
}

func (e *NotFoundError) Unwrap() error { return e.Err }

// ExitError describes a command that ran but exited non-zero. The error
// message includes the tail of stderr so it is directly useful to AI agents
// and humans without needing to inspect Result.
type ExitError struct {
	Cmd    Cmd
	Result Result
	Err    error
}

func (e *ExitError) Error() string {
	return fmt.Sprintf(
		"exec: %s %s: exit %d: %s",
		e.Cmd.Name,
		strings.Join(e.Cmd.Args, " "),
		e.Result.ExitCode,
		tail(e.Result.Stderr, 500),
	)
}

func (e *ExitError) Unwrap() error { return e.Err }

// tail returns the last n bytes of b as a string with trailing whitespace
// trimmed and inner newlines collapsed to spaces, so it fits cleanly in a
// single-line error message.
func tail(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		s = "..." + s[len(s)-n:]
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.TrimSpace(s)
}

package doctor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/byx-darwin/ncgo/internal/exec"
)

// versionRE pulls the first vMAJOR.MINOR.PATCH token out of a tool's --version
// output. Optional pre-release / build suffixes are tolerated and ignored.
var versionRE = regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)

// checkTool probes a binary by running `<bin> <args...>` and parsing the first
// vX.Y.Z token. The Check ID is namespaced as `tool.<name>`.
//
// It returns three classes of failure:
//   - not on PATH (severity error, install hint)
//   - present but version below minVer (severity error, upgrade hint)
//   - present, parsable, but at/above minVer (OK)
//
// A non-zero exit from the probe is treated as "version unknown", with a
// warning rather than a hard error so that future tool flag changes do not
// break doctor catastrophically.
func checkTool(ctx context.Context, r exec.Runner, name string, args []string, minVer string) Check {
	c := Check{ID: "tool." + name, Severity: SeverityError}
	res, err := r.Run(ctx, exec.Cmd{Name: name, Args: args})
	if err != nil {
		var nf *exec.NotFoundError
		if errors.As(err, &nf) {
			c.OK = false
			c.Message = name + " not found on PATH"
			c.Hint = exec.InstallHint(name)
			return c
		}
		// Probe ran but failed; downgrade to warn (the tool may exist with a
		// changed flag).
		c.OK = false
		c.Severity = SeverityWarn
		c.Message = fmt.Sprintf("%s present but probe failed: %v", name, err)
		c.Hint = exec.InstallHint(name)
		return c
	}
	out := string(res.Stdout) + "\n" + string(res.Stderr)
	got := versionRE.FindString(out)
	if got == "" {
		c.OK = false
		c.Severity = SeverityWarn
		c.Message = name + " version unparsable: " + truncate(out, 80)
		c.Hint = "expected output to contain a vX.Y.Z token"
		return c
	}
	cmp, err := semverCompare(got, minVer)
	if err != nil {
		c.OK = false
		c.Severity = SeverityWarn
		c.Message = fmt.Sprintf("%s: %v", name, err)
		return c
	}
	if cmp < 0 {
		c.OK = false
		c.Message = fmt.Sprintf("%s %s is below minimum %s", name, got, minVer)
		c.Hint = exec.InstallHint(name)
		return c
	}
	c.OK = true
	c.Message = fmt.Sprintf("%s %s (>= %s)", name, got, minVer)
	return c
}

// semverCompare compares two strict vMAJOR.MINOR.PATCH strings.
// Returns -1, 0, +1 like strings.Compare. Pre-release/build suffixes after the
// patch number are ignored, which matches the comparison semantics we need
// for "is this binary at least as new as MinHzVersion?".
func semverCompare(a, b string) (int, error) {
	av, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	bv, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		switch {
		case av[i] < bv[i]:
			return -1, nil
		case av[i] > bv[i]:
			return 1, nil
		}
	}
	return 0, nil
}

func parseSemver(s string) ([3]int, error) {
	var v [3]int
	t := strings.TrimPrefix(s, "v")
	// Strip pre-release / build metadata so e.g. v1.2.3-rc1 still parses.
	if i := strings.IndexAny(t, "-+"); i >= 0 {
		t = t[:i]
	}
	parts := strings.Split(t, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("not a semver: %q", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, fmt.Errorf("not a semver: %q", s)
		}
		v[i] = n
	}
	return v, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

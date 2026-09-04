package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// seedCheckProject builds a healthy mono service: manifest + one domain with
// a usecase file carrying anchors.
func seedCheckProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:    manifest.Meta{Version: "0.1.0-test", AssetsVersion: "test"},
		Mode:    manifest.ModeMono,
		Module:  "github.com/x/demo",
		Service: manifest.Service{Name: "demo", Kind: manifest.KindHertz},
		Domains: []string{"device"},
	}); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	usecase := filepath.Join(root, "internal", "usecase", "device", "device.go")
	if err := os.MkdirAll(filepath.Dir(usecase), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `package device

type UseCase struct{}

// ncgo:methods:start
// ncgo:methods:end
`
	if err := os.WriteFile(usecase, []byte(body), 0o644); err != nil {
		t.Fatalf("write usecase: %v", err)
	}
	return root
}

func TestRunCheckExitZeroOnHealthyProject(t *testing.T) {
	root := seedCheckProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runCheck(cmd, &checkOptions{root: root}); err != nil {
		t.Fatalf("runCheck: %v", err)
	}
	if !strings.Contains(out.String(), "all checks passed") {
		t.Fatalf("output missing success line:\n%s", out.String())
	}
}

func TestRunCheckExitOneOnBrokenAnchors(t *testing.T) {
	root := seedCheckProject(t)
	p := filepath.Join(root, "internal", "usecase", "device", "device.go")
	b, _ := os.ReadFile(p)
	_ = os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "// ncgo:methods:start\n", "")), 0o644)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runCheck(cmd, &checkOptions{root: root})
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.code != 1 {
		t.Fatalf("err = %v, want exitCodeError code 1", err)
	}
}

func TestRunCheckExitOneOnStaleDomains(t *testing.T) {
	root := seedCheckProject(t)
	// manifest has domain device; the context file claims device + ghost, so the
	// rendered domains fact no longer matches the manifest -> stale.
	claude := filepath.Join(root, "CLAUDE.md")
	content := "<!-- ncgo:managed -->\n# Project Context for Claude Code\n\n## Project Facts\n\n- domains: `[device, ghost]`\n"
	if err := os.WriteFile(claude, []byte(content), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runCheck(cmd, &checkOptions{root: root})
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.code != 1 {
		t.Fatalf("err = %v, want exitCodeError code 1 (context claims ghost domain)", err)
	}
}

func TestRunCheckExitOneOnStaleContext(t *testing.T) {
	root := seedCheckProject(t)
	// manifest has domain device; the context file declares an empty domain list,
	// so the rendered domains fact no longer matches the manifest -> stale.
	claude := filepath.Join(root, "CLAUDE.md")
	content := "<!-- ncgo:managed -->\n# Project Context for Claude Code\n\n## Project Facts\n\n- domains: `[]`\n"
	if err := os.WriteFile(claude, []byte(content), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runCheck(cmd, &checkOptions{root: root})
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.code != 1 {
		t.Fatalf("err = %v, want exitCodeError code 1", err)
	}
}

func TestRunCheckExitTwoOnMissingManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runCheck(cmd, &checkOptions{root: root})
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("err = %v, want exitCodeError code 2", err)
	}
}

func TestRunCheckJSONOutput(t *testing.T) {
	root := seedCheckProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runCheck(cmd, &checkOptions{root: root, output: "json"}); err != nil {
		t.Fatalf("runCheck: %v", err)
	}
	var got struct {
		Summary struct {
			CheckCount int `json:"checkCount"`
		} `json:"summary"`
		Checks []struct {
			ID string `json:"id"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if got.Summary.CheckCount == 0 {
		t.Fatalf("summary empty: %+v", got)
	}
	found := map[string]bool{}
	for _, c := range got.Checks {
		found[c.ID] = true
	}
	for _, id := range []string{"check.anchor", "check.manifest.consistency", "check.context.stale"} {
		if !found[id] {
			t.Errorf("json missing check %s: %+v", id, got.Checks)
		}
	}
}

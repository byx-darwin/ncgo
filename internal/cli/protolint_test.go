package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestRunProtolintJSON(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{
		root:   root,
		files:  []string{"invalid.proto"},
		rules:  []string{"PIO301"},
		output: "json",
	})
	if !errors.Is(err, errSilentFailure) {
		t.Fatalf("err = %v, want errSilentFailure", err)
	}
	var got struct {
		Root     string   `json:"root"`
		OK       bool     `json:"ok"`
		RulesRun []string `json:"rulesRun"`
		Summary  struct {
			FilesScanned     int `json:"filesScanned"`
			RPCsScanned      int `json:"rpcsScanned"`
			DiagnosticsCount int `json:"diagnosticsCount"`
		} `json:"summary"`
		Diagnostics []struct {
			RuleID  string `json:"ruleId"`
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if got.Root != root {
		t.Fatalf("root = %q, want %q", got.Root, root)
	}
	if got.OK {
		t.Fatalf("expected ok=false: %+v", got)
	}
	if len(got.RulesRun) != 1 || got.RulesRun[0] != "PIO301" {
		t.Fatalf("rulesRun = %v", got.RulesRun)
	}
	if got.Summary.FilesScanned != 1 || got.Summary.RPCsScanned != 2 || got.Summary.DiagnosticsCount != 1 {
		t.Fatalf("summary = %+v", got.Summary)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].RuleID != "PIO301" || got.Diagnostics[0].Message != "GetUserResp" {
		t.Fatalf("diagnostics = %+v", got.Diagnostics)
	}
}

func TestRunProtolintSARIF(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{
		root:   root,
		files:  []string{"invalid.proto"},
		rules:  []string{"PIO301"},
		output: "sarif",
	})
	if !errors.Is(err, errSilentFailure) {
		t.Fatalf("err = %v, want errSilentFailure", err)
	}
	var got struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid sarif json: %v\n%s", err, out.String())
	}
	if got.Schema == "" || got.Version != "2.1.0" || len(got.Runs) != 1 {
		t.Fatalf("sarif header = %+v", got)
	}
	if got.Runs[0].Tool.Driver.Name != "ncgo protolint" {
		t.Fatalf("driver.name = %q", got.Runs[0].Tool.Driver.Name)
	}
	if len(got.Runs[0].Tool.Driver.Rules) != 1 || got.Runs[0].Tool.Driver.Rules[0].ID != "PIO301" {
		t.Fatalf("driver.rules = %+v", got.Runs[0].Tool.Driver.Rules)
	}
	if len(got.Runs[0].Results) != 1 {
		t.Fatalf("results = %+v", got.Runs[0].Results)
	}
	res := got.Runs[0].Results[0]
	if res.RuleID != "PIO301" || res.Level != "error" || !strings.Contains(res.Message.Text, "transport envelope") {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Locations) != 1 || res.Locations[0].PhysicalLocation.ArtifactLocation.URI != "invalid.proto" {
		t.Fatalf("locations = %+v", res.Locations)
	}
}

func TestRunProtolintTextSuccess(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "scaffold", "mono", "testdata", "mono-default", "idl")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{
		root:   root,
		files:  []string{"app/demo.proto"},
		rules:  []string{"PIO101", "PIO102"},
		output: "text",
	})
	if err != nil {
		t.Fatalf("runProtolint text: %v", err)
	}
	if !strings.Contains(out.String(), "protolint: ok") {
		t.Fatalf("unexpected text output:\n%s", out.String())
	}
	if strings.Contains(out.String(), "✗ [") {
		t.Fatalf("unexpected diagnostics in text output:\n%s", out.String())
	}
}

func TestRunProtolintWarningsAreNonBlocking(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "protolint", "testdata", "phase2warnings")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{
		root:   root,
		files:  []string{"invalid.proto"},
		rules:  []string{"PIO111", "PIO112", "PIO113"},
		output: "text",
	})
	if err != nil {
		t.Fatalf("runProtolint warnings: %v", err)
	}
	if !strings.Contains(out.String(), "protolint: ok") {
		t.Fatalf("unexpected text output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "! [PIO111]") || !strings.Contains(out.String(), "! [PIO112]") || !strings.Contains(out.String(), "! [PIO113]") {
		t.Fatalf("warning output missing expected markers:\n%s", out.String())
	}
}

func TestRunProtolintCanIgnoreRule(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{
		root:        root,
		files:       []string{"invalid.proto"},
		rules:       []string{"PIO301"},
		ignoreRules: []string{"PIO301"},
		output:      "text",
	})
	if err != nil {
		t.Fatalf("runProtolint ignore rule: %v", err)
	}
	if strings.Contains(out.String(), "PIO301") {
		t.Fatalf("ignored diagnostic still rendered:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "protolint: ok") || !strings.Contains(out.String(), "suppressed=1") {
		t.Fatalf("unexpected text output:\n%s", out.String())
	}
}

func TestRunProtolintAutoDiscoversWorkspace(t *testing.T) {
	root := seedCLIProtoWorkspace(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{root: root, rules: []string{"PIO301"}, output: "text"})
	if !errors.Is(err, errSilentFailure) {
		t.Fatalf("err = %v, want errSilentFailure", err)
	}
	text := out.String()
	if !strings.Contains(text, "services/order-rpc/idl/orderrpc.proto") {
		t.Fatalf("workspace output missing aggregated file path:\n%s", text)
	}
	if !strings.Contains(text, "PIO301") {
		t.Fatalf("workspace output missing PIO301:\n%s", text)
	}
}

func TestRunProtolintRejectsInvalidOutput(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	cmd := &cobra.Command{}
	err := runProtolint(cmd, &protolintOptions{root: root, files: []string{"invalid.proto"}, output: "yaml"})
	if err == nil || !strings.Contains(err.Error(), "unsupported --output") {
		t.Fatalf("err = %v, want unsupported --output", err)
	}
}

func TestRunProtolintRequiresFiles(t *testing.T) {
	cmd := &cobra.Command{}
	err := runProtolint(cmd, &protolintOptions{root: ".", output: "text"})
	if err == nil || !strings.Contains(err.Error(), "at least one --file is required unless --root points to an ncgo service or micro workspace") {
		t.Fatalf("err = %v, want auto-discovery guidance", err)
	}
}

func protolintFixtureRoot(t *testing.T, elems ...string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join(elems...))
	if err != nil {
		t.Fatalf("fixture abs path: %v", err)
	}
	return root
}

func seedCLIProtoWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMicro,
		Name:   "commerce",
		Module: "github.com/x/commerce",
		Services: []manifest.WorkspaceService{
			{Name: "user-rpc", Kind: manifest.KindKitex, Dir: "services/user-rpc"},
			{Name: "order-rpc", Kind: manifest.KindKitex, Dir: "services/order-rpc"},
		},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	seedCLIProtoService(t, root, "services/user-rpc", "user-rpc", "idl/userrpc.proto", `syntax = "proto3";

package user;

message PingReq {}
message PingResp {}

service User {
  rpc Ping(PingReq) returns (PingResp) {}
}
`)
	seedCLIProtoService(t, root, "services/order-rpc", "order-rpc", "idl/orderrpc.proto", `syntax = "proto3";

package order;

message GetOrderReq {}
message GetOrderResp {
  int32 code = 1;
  string msg = 2;
  bool success = 3;
  string order_id = 4;
}

service Order {
  rpc GetOrder(GetOrderReq) returns (GetOrderResp) {}
}
`)
	return root
}

func seedCLIProtoService(t *testing.T, workspaceRoot, serviceRel, name, idl, body string) {
	t.Helper()
	serviceRoot := filepath.Join(workspaceRoot, serviceRel)
	if err := manifest.Save(serviceRoot, &manifest.Manifest{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:        manifest.ModeMono,
		Module:      "github.com/x/commerce/" + filepath.ToSlash(serviceRel),
		Service:     manifest.Service{Name: name, Kind: manifest.KindKitex, IDL: idl},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Save manifest: %v", err)
	}
	protoPath := filepath.Join(serviceRoot, filepath.FromSlash(idl))
	if err := os.MkdirAll(filepath.Dir(protoPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(protoPath, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
)

func TestRunAddInfraJSONDryRun(t *testing.T) {
	root := seedAddInfraProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddInfra(cmd, infra.KindRedis, &addInfraOptions{root: root, dryRun: true, output: "json"})
	if err != nil {
		t.Fatalf("runAddInfra json dry-run: %v", err)
	}
	var got struct {
		DryRun       bool             `json:"dryRun"`
		Updated      bool             `json:"updated"`
		WrittenPaths []string         `json:"writtenPaths"`
		WiredPaths   []string         `json:"wiredPaths"`
		NextSteps    []string         `json:"nextSteps"`
		Plan         []infra.PlanItem `json:"plan"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid json: %v\n%s", err, out.String())
	}
	if !got.DryRun || !got.Updated {
		t.Fatalf("dryRun/updated = %v/%v, want true/true", got.DryRun, got.Updated)
	}
	for _, wantPath := range []string{
		filepath.Join(root, "internal", "base", "data", "redis.go"),
		filepath.Join(root, "internal", "base", "data", "redis_shared.go"),
		filepath.Join(root, "conf", "dev", "conf.yaml"),
	} {
		if !containsPath(got.WrittenPaths, wantPath) {
			t.Fatalf("writtenPaths = %v, want to contain %s", got.WrittenPaths, wantPath)
		}
	}
	if len(got.WiredPaths) != 0 {
		t.Fatalf("wiredPaths = %v, want empty", got.WiredPaths)
	}
	if !planHas(got.Plan, "file", "create") || !planHas(got.Plan, "manifest", "add") || !planHas(got.Plan, "next_step", "run") {
		t.Fatalf("plan missing expected items: %+v", got.Plan)
	}
	for _, path := range []string{
		filepath.Join(root, "internal", "base", "data", "redis.go"),
		filepath.Join(root, "internal", "base", "data", "redis_shared.go"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote file %s: stat err = %v", path, err)
		}
	}
}

func TestRunAddInfraPlanShorthand(t *testing.T) {
	root := seedAddInfraProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddInfra(cmd, infra.KindRedis, &addInfraOptions{root: root, plan: true})
	if err != nil {
		t.Fatalf("runAddInfra --plan: %v", err)
	}
	var got struct {
		DryRun bool             `json:"dryRun"`
		Plan   []infra.PlanItem `json:"plan"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("--plan output is not valid json: %v\n%s", err, out.String())
	}
	if !got.DryRun {
		t.Fatalf("dryRun = false, want true")
	}
	if !planHas(got.Plan, "file", "create") || !planHas(got.Plan, "manifest", "add") {
		t.Fatalf("plan missing expected items: %+v", got.Plan)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis.go")); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote redis file: stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis_shared.go")); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote redis helper: stat err = %v", err)
	}
}

func TestRunAddInfraDefaultTextOutput(t *testing.T) {
	root := seedAddInfraProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddInfra(cmd, infra.KindRedis, &addInfraOptions{root: root})
	if err != nil {
		t.Fatalf("runAddInfra text: %v", err)
	}
	if !strings.Contains(out.String(), "wrote ") || !strings.Contains(out.String(), "next steps:") {
		t.Fatalf("text output missing expected content:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis.go")); err != nil {
		t.Fatalf("redis file was not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis_shared.go")); err != nil {
		t.Fatalf("redis helper was not written: %v", err)
	}
}

func TestRunAddInfraRejectsInvalidOutputBeforeWriting(t *testing.T) {
	root := seedAddInfraProject(t)
	cmd := &cobra.Command{}
	err := runAddInfra(cmd, infra.KindRedis, &addInfraOptions{root: root, output: "yaml"})
	if err == nil || !strings.Contains(err.Error(), "unsupported --output") {
		t.Fatalf("err = %v, want unsupported output", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis.go")); !os.IsNotExist(err) {
		t.Fatalf("invalid output wrote redis file: stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis_shared.go")); !os.IsNotExist(err) {
		t.Fatalf("invalid output wrote redis helper: stat err = %v", err)
	}
}

func TestRunAddDomainPlanShorthand(t *testing.T) {
	root := seedAddInfraProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddDomain(cmd, "device", &addDomainOptions{root: root, plan: true})
	if err != nil {
		t.Fatalf("runAddDomain --plan: %v", err)
	}
	var got struct {
		DryRun       bool             `json:"dryRun"`
		WrittenPaths []string         `json:"writtenPaths"`
		Plan         []infra.PlanItem `json:"plan"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("domain --plan output is not valid json: %v\n%s", err, out.String())
	}
	if !got.DryRun || len(got.WrittenPaths) != 3 {
		t.Fatalf("dryRun/writtenPaths = %v/%v", got.DryRun, got.WrittenPaths)
	}
	if !planHas(got.Plan, "file", "create") || !planHas(got.Plan, "manifest", "add") {
		t.Fatalf("plan missing expected items: %+v", got.Plan)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "usecase", "device", "device.go")); !os.IsNotExist(err) {
		t.Fatalf("domain --plan wrote file: stat err = %v", err)
	}
}

func TestRunAddRPCPlanShorthand(t *testing.T) {
	root := seedAddWorkspace(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddRPC(cmd, "user-rpc", &addRPCOptions{root: root, plan: true})
	if err != nil {
		t.Fatalf("runAddRPC --plan: %v", err)
	}
	var got struct {
		DryRun      bool             `json:"dryRun"`
		ServiceRel  string           `json:"serviceRel"`
		RanGenerate bool             `json:"ranGenerate"`
		Plan        []infra.PlanItem `json:"plan"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("rpc --plan output is not valid json: %v\n%s", err, out.String())
	}
	if !got.DryRun || got.RanGenerate || got.ServiceRel != "services/user-rpc" {
		t.Fatalf("rpc plan result = %+v", got)
	}
	if !planHas(got.Plan, "directory", "create") || !planHas(got.Plan, "workspace", "add") || !planHas(got.Plan, "generator", "run") {
		t.Fatalf("plan missing expected items: %+v", got.Plan)
	}
	if _, err := os.Stat(filepath.Join(root, "services", "user-rpc")); !os.IsNotExist(err) {
		t.Fatalf("rpc --plan created service dir: stat err = %v", err)
	}
}

func TestRunAddBFFPlanShorthand(t *testing.T) {
	root := seedAddWorkspace(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddBFF(cmd, "web-bff", &addBFFOptions{root: root, plan: true})
	if err != nil {
		t.Fatalf("runAddBFF --plan: %v", err)
	}
	var got struct {
		DryRun      bool             `json:"dryRun"`
		ServiceRel  string           `json:"serviceRel"`
		RanGenerate bool             `json:"ranGenerate"`
		Plan        []infra.PlanItem `json:"plan"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("bff --plan output is not valid json: %v\n%s", err, out.String())
	}
	if !got.DryRun || got.RanGenerate || got.ServiceRel != "services/web-bff" {
		t.Fatalf("bff plan result = %+v", got)
	}
	if !planHas(got.Plan, "directory", "create") || !planHas(got.Plan, "workspace", "add") || !planHas(got.Plan, "generator", "run") {
		t.Fatalf("plan missing expected items: %+v", got.Plan)
	}
	if _, err := os.Stat(filepath.Join(root, "services", "web-bff")); !os.IsNotExist(err) {
		t.Fatalf("bff --plan created service dir: stat err = %v", err)
	}
}

func seedAddInfraProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	m := &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: "github.com/x/demo",
		Service: manifest.Service{
			Name: "demo", Kind: manifest.KindHertz, IDL: "idl/app/demo.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return root
}

func seedAddWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	w := &manifest.Workspace{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:        manifest.ModeMicro,
		Name:        "demo",
		Module:      "github.com/x/demo",
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.SaveWorkspace(root, w); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return root
}

func planHas(plan []infra.PlanItem, kind, action string) bool {
	for _, item := range plan {
		if item.Kind == kind && item.Action == action {
			return true
		}
	}
	return false
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

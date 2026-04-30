package main

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
	wantPath := filepath.Join(root, "internal", "base", "data", "redis.go")
	if len(got.WrittenPaths) != 1 || got.WrittenPaths[0] != wantPath {
		t.Fatalf("writtenPaths = %v, want [%s]", got.WrittenPaths, wantPath)
	}
	if len(got.WiredPaths) != 0 {
		t.Fatalf("wiredPaths = %v, want empty", got.WiredPaths)
	}
	if !planHas(got.Plan, "file", "create") || !planHas(got.Plan, "manifest", "add") || !planHas(got.Plan, "next_step", "run") {
		t.Fatalf("plan missing expected items: %+v", got.Plan)
	}
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote redis file: stat err = %v", err)
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

func planHas(plan []infra.PlanItem, kind, action string) bool {
	for _, item := range plan {
		if item.Kind == kind && item.Action == action {
			return true
		}
	}
	return false
}

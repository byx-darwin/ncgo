package mono

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

type fakeRunner struct {
	calls []exec.Cmd
}

func (f *fakeRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	f.calls = append(f.calls, c)
	return exec.Result{}, nil
}

func baseOpts(t *testing.T) Options {
	t.Helper()
	return Options{
		Name:          "demo",
		Module:        "github.com/x/demo",
		Dir:           filepath.Join(t.TempDir(), "demo"),
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.0.0-test",
		Now:           time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		NoGenerate:    true,
	}
}

func TestGenerateNoGenerateProducesGoldenTree(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.RanGenerate {
		t.Errorf("RanGenerate = true, want false (NoGenerate set)")
	}
	got := walk(t, res.Dir)
	want := []string{
		".ncgo/manifest.yaml",
		"idl/app/demo.proto",
		"template/data.json",
		"template/layout.yaml",
		"template/package.yaml",
	}
	if !equal(got, want) {
		t.Errorf("tree mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestGenerateHertzTemplateIncludesSafeOptionalWiringAnchors(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "template", "layout.yaml"))
	if err != nil {
		t.Fatalf("read hertz layout: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"Optional structured logging wiring",
		"import \"{{.GoModule}}/internal/base/logging\"",
		"h.Use(logging.HertzRecovery())",
		"h.Use(logging.HertzRequestID())",
		"h.Use(logging.HertzAccessLog())",
		"Optional release canary wiring",
		"import \"{{.GoModule}}/internal/base/release\"",
		"h.Use(release.HertzTraffic())",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz layout missing optional wiring anchor %q", want)
		}
	}
}

func TestGenerateRendersDataJSON(t *testing.T) {
	opts := baseOpts(t)
	opts.WithDatabase = true
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(res.Dir, "template", "data.json"))
	if err != nil {
		t.Fatalf("read data.json: %v", err)
	}
	var parsed map[string]map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("parse data.json: %v", err)
	}
	star := parsed["*"]
	if star["GoModule"] != "github.com/x/demo" {
		t.Errorf("GoModule = %v, want github.com/x/demo", star["GoModule"])
	}
	if star["ServiceName"] != "demo" {
		t.Errorf("ServiceName = %v", star["ServiceName"])
	}
	if star["WithDatabase"] != true {
		t.Errorf("WithDatabase = %v, want true", star["WithDatabase"])
	}
}

func TestGenerateWritesManifest(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(res.Dir, ".ncgo", "manifest.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		"mode: mono",
		"module: github.com/x/demo",
		"name: demo",
		"kind: hertz",
		"idl: idl/app/demo.proto",
		"version: 0.0.0-test",
		"assets_version: test-assets",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("manifest missing %q\n---\n%s", want, s)
		}
	}
}

func TestGenerateInvokesHZViaRunner(t *testing.T) {
	opts := baseOpts(t)
	opts.NoGenerate = false
	r := &fakeRunner{}
	opts.Runner = r
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.RanGenerate {
		t.Errorf("RanGenerate = false, want true")
	}
	if strings.Contains(strings.Join(res.NextSteps, "\n"), "go mod init") {
		t.Errorf("post-generate next steps must not include go mod init: %v", res.NextSteps)
	}
	if len(r.calls) != 1 || r.calls[0].Name != "hz" {
		t.Fatalf("expected one hz call, got %+v", r.calls)
	}
	if r.calls[0].Dir != res.Dir {
		t.Errorf("hz call Dir = %q, want %q", r.calls[0].Dir, res.Dir)
	}
	args := strings.Join(r.calls[0].Args, " ")
	for _, want := range []string{
		"new", "--mod=github.com/x/demo", "--idl=idl/app/demo.proto",
		"--customize_layout=template/layout.yaml",
		"--customize_layout_data_path=template/data.json",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("hz args missing %q in %q", want, args)
		}
	}
}

func TestGenerateRejectsNonEmptyDir(t *testing.T) {
	opts := baseOpts(t)
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(opts.Dir, "stray"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := Generate(context.Background(), opts); err == nil {
		t.Fatalf("expected error for non-empty dir")
	}
}

func TestGenerateKitexNoGenerateProducesTree(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got := walk(t, res.Dir)
	for _, want := range []string{
		".ncgo/manifest.yaml",
		"idl/demo.proto",
		"template/kitex-template/main.yaml",
		"template/kitex-template/server.yaml",
		"template/kitex-template/handler.yaml",
		"template/kitex-template/makefile.yaml",
	} {
		if !contains(got, want) {
			t.Errorf("kitex tree missing %q\n got: %v", want, got)
		}
	}
	for _, unwanted := range []string{
		"template/data.json",
		"template/layout.yaml",
		"template/package.yaml",
		"idl/app/demo.proto",
	} {
		if contains(got, unwanted) {
			t.Errorf("kitex tree must not include hertz file %q", unwanted)
		}
	}
}

func TestGenerateKitexTemplatesIncludeSafeOptionalWiringAnchors(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	serverBody, err := os.ReadFile(filepath.Join(res.Dir, "template", "kitex-template", "server.yaml"))
	if err != nil {
		t.Fatalf("read kitex server template: %v", err)
	}
	clientBody, err := os.ReadFile(filepath.Join(res.Dir, "template", "kitex-template", "client.yaml"))
	if err != nil {
		t.Fatalf("read kitex client template: %v", err)
	}
	serverTemplate := string(serverBody)
	for _, want := range []string{
		"Optional structured logging wiring",
		"import \"{{.Module}}/internal/base/logging\"",
		"logging.KitexRequestID()",
		"logging.KitexAccessLog()",
		"logging.KitexRecovery()",
		"Optional release canary wiring",
		"import \"{{.Module}}/internal/base/release\"",
		"release.KitexTraffic()",
	} {
		if !strings.Contains(serverTemplate, want) {
			t.Errorf("kitex server template missing optional wiring anchor %q", want)
		}
	}
	clientTemplate := string(clientBody)
	for _, want := range []string{
		"Optional client-side structured RPC logs",
		"options = append(options, kitexclient.WithMiddleware(endpoint.Chain(",
		"logging.KitexRequestID()",
		"logging.KitexAccessLog()",
		"options = append(options, kitexclient.WithMiddleware(release.KitexTraffic()))",
		"release.Selector",
	} {
		if !strings.Contains(clientTemplate, want) {
			t.Errorf("kitex client template missing optional wiring anchor %q", want)
		}
	}
}

func TestGenerateKitexWritesManifest(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(res.Dir, ".ncgo", "manifest.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		"kind: kitex",
		"idl: idl/demo.proto",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("kitex manifest missing %q\n---\n%s", want, s)
		}
	}
}

func TestGenerateKitexInvokesKitexViaRunner(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	opts.NoGenerate = false
	r := &fakeRunner{}
	opts.Runner = r
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(strings.Join(res.NextSteps, "\n"), "go mod init") {
		t.Errorf("post-generate next steps must not include go mod init: %v", res.NextSteps)
	}
	if len(r.calls) != 1 || r.calls[0].Name != "kitex" {
		t.Fatalf("expected one kitex call, got %+v", r.calls)
	}
	args := strings.Join(r.calls[0].Args, " ")
	for _, want := range []string{
		"-module github.com/x/demo",
		"-template-dir template/kitex-template",
		"-type protobuf",
		"idl/demo.proto",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("kitex args missing %q in %q", want, args)
		}
	}
}

func TestGenerateKitexNormalizesHyphenatedServiceName(t *testing.T) {
	opts := baseOpts(t)
	opts.Name = "user-api"
	opts.Module = "github.com/x/user-api"
	opts.Dir = filepath.Join(t.TempDir(), "user-api")
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "idl", "userapi.proto"))
	if err != nil {
		t.Fatalf("read normalized kitex IDL: %v", err)
	}
	for _, want := range []string{"package userapi;", "kitex_gen/userapi;userapi", "service UserApi"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("kitex IDL missing %q\n---\n%s", want, string(body))
		}
	}
}

func TestValidateRejectsBadInputs(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Options)
	}{
		{"bad-name", func(o *Options) { o.Name = "Bad_Name" }},
		{"empty-module", func(o *Options) { o.Module = "" }},
		{"flat-module", func(o *Options) { o.Module = "demo" }},
		{"empty-version", func(o *Options) { o.NCGOVersion = "" }},
		{"bad-kind", func(o *Options) { o.Kind = "grpc" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := baseOpts(t)
			tc.mut(&o)
			if _, err := Generate(context.Background(), o); err == nil {
				t.Errorf("expected error")
			}
		})
	}
}

func walk(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(out)
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

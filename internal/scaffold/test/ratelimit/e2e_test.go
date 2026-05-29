package ratelimit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Config parsing tests
// ---------------------------------------------------------------------------

func TestParseRateLimitConfig(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantSource  string
		wantBackend string
	}{
		{
			name: "config + memory",
			yamlContent: `
rate_limit:
  enabled: true
  source:
    type: config
  backend: memory
`,
			wantSource:  "config",
			wantBackend: "memory",
		},
		{
			name: "database + redis",
			yamlContent: `
rate_limit:
  enabled: true
  source:
    type: database
  backend: redis
`,
			wantSource:  "database",
			wantBackend: "redis",
		},
		{
			name: "rule_center + redis",
			yamlContent: `
rate_limit:
  enabled: true
  source:
    type: rule_center
  backend: redis
`,
			wantSource:  "rule_center",
			wantBackend: "redis",
		},
		{
			name: "defaults when empty",
			yamlContent: `
rate_limit:
  enabled: false
`,
			wantSource:  "config",
			wantBackend: "memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			confDir := filepath.Join(dir, "conf", "dev")
			if err := os.MkdirAll(confDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(confDir, "conf.yaml"), []byte(tt.yamlContent), 0o644); err != nil {
				t.Fatal(err)
			}

			src, bk, err := parseRateLimitConfig(dir)
			if err != nil {
				t.Fatalf("parseRateLimitConfig: %v", err)
			}
			if src != tt.wantSource {
				t.Errorf("source = %q, want %q", src, tt.wantSource)
			}
			if bk != tt.wantBackend {
				t.Errorf("backend = %q, want %q", bk, tt.wantBackend)
			}
		})
	}
}

func TestParseRateLimitConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, _, err := parseRateLimitConfig(dir)
	if err == nil {
		t.Error("expected error for missing conf.yaml, got nil")
	}
}

// ---------------------------------------------------------------------------
// Dependency computation tests
// ---------------------------------------------------------------------------

func TestComputeDependencies(t *testing.T) {
	tests := []struct {
		source, backend string
		wantPostgres    bool
		wantRedis       bool
	}{
		{"config", "memory", false, false},
		{"config", "redis", false, true},
		{"database", "memory", true, false},
		{"database", "redis", true, true},
		{"rule_center", "redis", true, true},
		{"grpc", "memory", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.source+"+"+tt.backend, func(t *testing.T) {
			deps := computeDependencies(tt.source, tt.backend)
			if deps.needPostgres != tt.wantPostgres {
				t.Errorf("needPostgres = %v, want %v", deps.needPostgres, tt.wantPostgres)
			}
			if deps.needRedis != tt.wantRedis {
				t.Errorf("needRedis = %v, want %v", deps.needRedis, tt.wantRedis)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Result classification tests
// ---------------------------------------------------------------------------

func TestClassifyResult(t *testing.T) {
	tests := []struct {
		name       string
		ar         attackResult
		wantStatus string
		wantPass   bool
	}{
		{
			name:       "pass: some 429s",
			ar:         attackResult{TotalReqs: 1000, Status200: 600, Status429: 400, StatusOther: 0},
			wantStatus: "PASS",
			wantPass:   true,
		},
		{
			name:       "fail: no 429s",
			ar:         attackResult{TotalReqs: 1000, Status200: 1000, Status429: 0, StatusOther: 0},
			wantStatus: "FAIL",
			wantPass:   false,
		},
		{
			name:       "warn: all 429s",
			ar:         attackResult{TotalReqs: 1000, Status200: 0, Status429: 1000, StatusOther: 0},
			wantStatus: "WARN",
			wantPass:   false,
		},
		{
			name:       "fail: no requests",
			ar:         attackResult{TotalReqs: 0},
			wantStatus: "FAIL",
			wantPass:   false,
		},
		{
			name:       "pass: 429s with other errors",
			ar:         attackResult{TotalReqs: 1000, Status200: 500, Status429: 400, StatusOther: 100},
			wantStatus: "PASS",
			wantPass:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, pass := classifyResult(&tt.ar)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if pass != tt.wantPass {
				t.Errorf("pass = %v, want %v", pass, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Vegeta JSON parsing tests
// ---------------------------------------------------------------------------

func TestParseVegetaJSON(t *testing.T) {
	input := `{
		"latencies": {"total": 12000000000, "mean": 12000000, "99th": 45000000},
		"requests": 2000,
		"success": 1200,
		"status_codes": {"200": 1200, "429": 750, "503": 50}
	}`

	ar, err := parseVegetaJSON([]byte(input))
	if err != nil {
		t.Fatalf("parseVegetaJSON: %v", err)
	}
	if ar.TotalReqs != 2000 {
		t.Errorf("TotalReqs = %d, want 2000", ar.TotalReqs)
	}
	if ar.Status200 != 1200 {
		t.Errorf("Status200 = %d, want 1200", ar.Status200)
	}
	if ar.Status429 != 750 {
		t.Errorf("Status429 = %d, want 750", ar.Status429)
	}
	if ar.StatusOther != 50 {
		t.Errorf("StatusOther = %d, want 50", ar.StatusOther)
	}
	if ar.AvgLatency != 12*time.Millisecond {
		t.Errorf("AvgLatency = %s, want 12ms", ar.AvgLatency)
	}
	if ar.P99Latency != 45*time.Millisecond {
		t.Errorf("P99Latency = %s, want 45ms", ar.P99Latency)
	}
}

func TestParseVegetaJSONInvalid(t *testing.T) {
	_, err := parseVegetaJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// Report generation tests
// ---------------------------------------------------------------------------

func TestWriteMarkdownReport(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.md")

	result := &E2EResult{
		Mode:        "mono",
		Kind:        "hertz",
		Source:      "database",
		Backend:     "redis",
		ServiceName: "demo-api",
		Status:      "PASS",
		TotalReqs:   2000,
		Status200:   1200,
		Status429:   800,
		StatusOther: 0,
		AvgLatency:  12 * time.Millisecond,
		P99Latency:  45 * time.Millisecond,
		StartedAt:   time.Now(),
	}
	opts := E2EOptions{
		Host:     "localhost",
		Port:     8080,
		Rate:     200,
		Duration: "10s",
		Paths:    []string{"/healthz", "/"},
		Report:   reportPath,
	}

	if err := writeReport(result, opts); err != nil {
		t.Fatalf("writeReport: %v", err)
	}

	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	content := string(body)
	for _, want := range []string{
		"# Rate Limit E2E Test Report",
		"Project mode**: mono",
		"Rule source**: database",
		"Counter backend**: redis",
		"Service**: demo-api (hertz)",
		"200 OK",
		"429 Too Many Requests",
		"Avg latency",
		"P99 latency",
		"PostgreSQL: running",
		"Redis: running",
	} {
		if !containsStr(content, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestWriteJSONReport(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.json")

	result := &E2EResult{
		Mode:        "micro",
		Kind:        "hertz",
		Source:      "rule_center",
		Backend:     "redis",
		ServiceName: "user-api",
		Status:      "PASS",
		TotalReqs:   2000,
		Status200:   1200,
		Status429:   800,
		StatusOther: 0,
		AvgLatency:  12 * time.Millisecond,
		P99Latency:  45 * time.Millisecond,
		StartedAt:   time.Now(),
	}
	opts := E2EOptions{
		Host:     "localhost",
		Port:     8080,
		Rate:     200,
		Duration: "10s",
		Paths:    []string{"/healthz"},
		Report:   reportPath,
	}

	if err := writeReport(result, opts); err != nil {
		t.Fatalf("writeReport: %v", err)
	}

	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	var report jsonReport
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("parse JSON report: %v", err)
	}

	if report.Mode != "micro" {
		t.Errorf("mode = %q, want micro", report.Mode)
	}
	if report.Source != "rule_center" {
		t.Errorf("source = %q, want rule_center", report.Source)
	}
	if report.Backend != "redis" {
		t.Errorf("backend = %q, want redis", report.Backend)
	}
	if report.Results.Status != "PASS" {
		t.Errorf("status = %q, want PASS", report.Results.Status)
	}
	if report.Results.Status429 != 800 {
		t.Errorf("status429 = %d, want 800", report.Results.Status429)
	}
}

func TestWriteReportUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	opts := E2EOptions{Report: filepath.Join(dir, "report.txt")}
	result := &E2EResult{}

	err := writeReport(result, opts)
	if err == nil {
		t.Error("expected error for unsupported format, got nil")
	}
}

// ---------------------------------------------------------------------------
// Compose service detection tests
// ---------------------------------------------------------------------------

func TestHasComposeService(t *testing.T) {
	dir := t.TempDir()

	// Create a compose.yaml with postgres service
	composeContent := `name: demo
services:
  demo:
    image: demo:latest
    ports:
      - "8080:8080"
  postgres:
    image: postgres:alpine
  redis:
    image: redis:alpine
`
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(composeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if !hasComposeService(dir, "postgres") {
		t.Error("expected postgres service to be found")
	}
	if !hasComposeService(dir, "redis") {
		t.Error("expected redis service to be found")
	}
	if hasComposeService(dir, "nonexistent") {
		t.Error("expected nonexistent service to not be found")
	}
}

func TestHasComposeServiceNoFile(t *testing.T) {
	dir := t.TempDir()
	if hasComposeService(dir, "postgres") {
		t.Error("expected false when no compose file exists")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStrAt(s, substr))
}

func containsStrAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// End-to-end integration tests (detect + config + dry-run + report)
// ---------------------------------------------------------------------------

// setupFakeProject creates a minimal project structure for E2E testing.
func setupFakeProject(t *testing.T, manifestBody, confBody string) string {
	t.Helper()
	dir := t.TempDir()

	// Manifest
	ncgoDir := filepath.Join(dir, ".ncgo")
	if err := os.MkdirAll(ncgoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ncgoDir, "manifest.yaml"), []byte(manifestBody), 0o644); err != nil {
		t.Fatal(err)
	}

	// Conf
	confDir := filepath.Join(dir, "conf", "dev")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "conf.yaml"), []byte(confBody), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestE2E_DryRun_MonoDatabaseRedis(t *testing.T) {
	dir := setupFakeProject(t, `
ncgo:
    version: 0.0.0-test
    assets_version: test-assets
mode: mono
module: github.com/x/demo
service:
    name: demo-api
    kind: hertz
    with_database: true
generated_at: 2026-04-29T00:00:00Z
`, `
rate_limit:
  enabled: true
  source:
    type: database
  backend: redis
`)

	ctx := t.Context()
	result, err := E2E(ctx, E2EOptions{
		Root:   dir,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("E2E dry-run: %v", err)
	}

	if result.Mode != "mono" {
		t.Errorf("mode = %q, want mono", result.Mode)
	}
	if result.Kind != "hertz" {
		t.Errorf("kind = %q, want hertz", result.Kind)
	}
	if result.ServiceName != "demo-api" {
		t.Errorf("service = %q, want demo-api", result.ServiceName)
	}
	if result.Source != "database" {
		t.Errorf("source = %q, want database", result.Source)
	}
	if result.Backend != "redis" {
		t.Errorf("backend = %q, want redis", result.Backend)
	}
}

func TestE2E_DryRun_MicroRuleCenter(t *testing.T) {
	// Create a micro workspace with rule-center
	dir := t.TempDir()

	// Workspace file
	wsBody := `
ncgo:
    version: 0.0.0-test
mode: micro
name: my-workspace
module: github.com/x/workspace
services:
    - name: rule-center
      kind: kitex
      dir: services/rule-center
    - name: user-api
      kind: hertz
      dir: services/user-api
`
	if err := os.WriteFile(filepath.Join(dir, "ncgo.workspace"), []byte(wsBody), 0o644); err != nil {
		t.Fatal(err)
	}

	// Conf for the first Hertz service
	confBody := `
rate_limit:
  enabled: true
  source:
    type: rule_center
  backend: redis
`
	confDir := filepath.Join(dir, "conf", "dev")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "conf.yaml"), []byte(confBody), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	result, err := E2E(ctx, E2EOptions{
		Root:   dir,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("E2E micro dry-run: %v", err)
	}

	if result.Mode != "micro" {
		t.Errorf("mode = %q, want micro", result.Mode)
	}
	if result.Source != "rule_center" {
		t.Errorf("source = %q, want rule_center", result.Source)
	}
	if result.Backend != "redis" {
		t.Errorf("backend = %q, want redis", result.Backend)
	}
	if result.ServiceName != "user-api" {
		t.Errorf("service = %q, want user-api", result.ServiceName)
	}
}

func TestE2E_DryRun_MicroWithoutRuleCenter(t *testing.T) {
	dir := t.TempDir()

	wsBody := `
ncgo:
    version: 0.0.0-test
mode: micro
name: my-workspace
module: github.com/x/workspace
services:
    - name: user-api
      kind: hertz
      dir: services/user-api
`
	if err := os.WriteFile(filepath.Join(dir, "ncgo.workspace"), []byte(wsBody), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	_, err := E2E(ctx, E2EOptions{
		Root:   dir,
		DryRun: true,
	})
	if err == nil {
		t.Fatal("expected error for micro without rule-center, got nil")
	}
	if !strings.Contains(err.Error(), "no rule-center") {
		t.Errorf("error = %q, want 'no rule-center'", err.Error())
	}
}

func TestE2E_DryRun_UnknownProject(t *testing.T) {
	dir := t.TempDir()

	ctx := t.Context()
	_, err := E2E(ctx, E2EOptions{
		Root:   dir,
		DryRun: true,
	})
	if err == nil {
		t.Fatal("expected error for unknown project, got nil")
	}
}

func TestE2E_DryRun_ConfigMemory(t *testing.T) {
	// Simplest case: config source + memory backend
	dir := setupFakeProject(t, `
ncgo:
    version: 0.0.0-test
    assets_version: test-assets
mode: mono
module: github.com/x/demo
service:
    name: my-api
    kind: kitex
generated_at: 2026-04-29T00:00:00Z
`, `
rate_limit:
  enabled: true
  source:
    type: config
  backend: memory
`)

	ctx := t.Context()
	result, err := E2E(ctx, E2EOptions{
		Root:   dir,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("E2E config dry-run: %v", err)
	}

	if result.Mode != "mono" {
		t.Errorf("mode = %q, want mono", result.Mode)
	}
	if result.Kind != "kitex" {
		t.Errorf("kind = %q, want kitex", result.Kind)
	}
	if result.Source != "config" {
		t.Errorf("source = %q, want config", result.Source)
	}
	if result.Backend != "memory" {
		t.Errorf("backend = %q, want memory", result.Backend)
	}
}

func TestE2E_FullFlow_ReportGeneration(t *testing.T) {
	// Test the full E2E flow up to report generation using DryRun to skip
	// actual docker/vegeta, then manually set result fields and verify report.
	dir := setupFakeProject(t, `
ncgo:
    version: 0.0.0-test
    assets_version: test-assets
mode: mono
module: github.com/x/demo
service:
    name: demo-api
    kind: hertz
    with_database: true
generated_at: 2026-04-29T00:00:00Z
`, `
rate_limit:
  enabled: true
  source:
    type: database
  backend: redis
`)

	reportPath := filepath.Join(dir, "e2e-report.md")

	ctx := t.Context()
	result, err := E2E(ctx, E2EOptions{
		Root:   dir,
		DryRun: true,
		Report: reportPath,
	})
	if err != nil {
		t.Fatalf("E2E dry-run: %v", err)
	}

	// Simulate a real attack result (dry-run skips attack)
	result.Status = "PASS"
	result.Pass = true
	result.TotalReqs = 2000
	result.Status200 = 1200
	result.Status429 = 800
	result.AvgLatency = 12 * time.Millisecond
	result.P99Latency = 45 * time.Millisecond

	// Write report manually to verify the generation path
	if err := writeReport(result, E2EOptions{Report: reportPath}); err != nil {
		t.Fatalf("writeReport: %v", err)
	}

	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	content := string(body)
	// Verify report has the right project context from E2E detection
	for _, want := range []string{
		"Project mode**: mono",
		"Rule source**: database",
		"Counter backend**: redis",
		"demo-api",
		"hertz",
		"PostgreSQL: running",
		"Redis: running",
	} {
		if !containsStr(content, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestE2E_DefaultsApplied(t *testing.T) {
	// When E2E is called with zero values, defaults should be applied
	dir := setupFakeProject(t, `
ncgo:
    version: 0.0.0-test
    assets_version: test-assets
mode: mono
module: github.com/x/demo
service:
    name: test-svc
    kind: hertz
generated_at: 2026-04-29T00:00:00Z
`, `
rate_limit:
  enabled: true
`)

	ctx := t.Context()
	result, err := E2E(ctx, E2EOptions{
		Root:   dir,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("E2E defaults: %v", err)
	}

	// Defaults
	if result.Source != "config" {
		t.Errorf("source = %q, want config (default)", result.Source)
	}
	if result.Backend != "memory" {
		t.Errorf("backend = %q, want memory (default)", result.Backend)
	}
}

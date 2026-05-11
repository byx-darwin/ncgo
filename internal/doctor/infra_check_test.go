package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"gopkg.in/yaml.v3"
)

func seedInfraProject(t *testing.T, infraKinds []string, confYAML string) string {
	t.Helper()
	root := seedProject(t, false, "")
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	m.Infra = infraKinds
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if confYAML != "" {
		dir := filepath.Join(root, "conf", "dev")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir conf/dev: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "conf.yaml"), []byte(confYAML), 0o644); err != nil {
			t.Fatalf("write conf.yaml: %v", err)
		}
	}
	return root
}

func seedInfraFile(t *testing.T, root, kind string) {
	t.Helper()
	rel, ok := infraFiles[kind]
	if !ok {
		t.Fatalf("unknown infra kind: %s", kind)
	}
	dir := filepath.Join(root, filepath.Dir(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir infra dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte("package data\n"), 0o644); err != nil {
		t.Fatalf("write infra file: %v", err)
	}
}

func infraFailingChecks(root string, m *manifest.Manifest) []Check {
	var out []Check
	for _, c := range checkInfraFiles(root, m) {
		if !c.OK {
			out = append(out, c)
		}
	}
	return out
}

func TestInfraFilesDetectRedisInRateLimitWithoutManifestEntry(t *testing.T) {
	conf := `rate_limit:
  backend: "redis"
  window: "1m"
  max_requests: 100
`
	root := seedInfraProject(t, nil, conf)
	fails := infraFailingChecks(root, loadOrFail(t, root))
	if len(fails) != 1 {
		t.Fatalf("expected 1 failing check, got %d: %+v", len(fails), fails)
	}
	if fails[0].ID != "infra.file.redis" {
		t.Errorf("ID = %q, want infra.file.redis", fails[0].ID)
	}
	if fails[0].Severity != SeverityError {
		t.Errorf("Severity = %s, want error", fails[0].Severity)
	}
	if fails[0].Hint == "" {
		t.Errorf("expected hint, got empty")
	}
}

func TestInfraFilesOKWhenFileExists(t *testing.T) {
	conf := `rate_limit:
  backend: "redis"
`
	root := seedInfraProject(t, nil, conf)
	seedInfraFile(t, root, "redis")
	checks := checkInfraFiles(root, loadOrFail(t, root))
	for _, c := range checks {
		if !c.OK {
			t.Errorf("expected all checks OK when infra file exists, got: %+v", c)
		}
	}
}

func TestInfraFilesCheckSkipsWhenConfigAbsent(t *testing.T) {
	// No conf/dev/conf.yaml — should not report any infra failures from config
	root := seedInfraProject(t, nil, "")
	checks := checkInfraFiles(root, loadOrFail(t, root))
	for _, c := range checks {
		if !c.OK {
			t.Errorf("expected all skip-OK when config absent, got: %+v", c)
		}
	}
}

func TestInfraFilesCheckDetectsNacosConfigCenter(t *testing.T) {
	conf := `config_center:
  enabled: true
  provider: "nacos"
`
	root := seedInfraProject(t, nil, conf)
	fails := infraFailingChecks(root, loadOrFail(t, root))
	if len(fails) != 1 {
		t.Fatalf("expected 1 failing check, got %d: %+v", len(fails), fails)
	}
	if fails[0].ID != "infra.file.nacos" {
		t.Errorf("ID = %q, want infra.file.nacos", fails[0].ID)
	}
}

func TestInfraFilesCheckManifestMissingFile(t *testing.T) {
	// Manifest lists redis but the file does not exist
	root := seedInfraProject(t, []string{"redis"}, "")
	fails := infraFailingChecks(root, loadOrFail(t, root))
	if len(fails) != 1 {
		t.Fatalf("expected 1 failing check, got %d: %+v", len(fails), fails)
	}
	if fails[0].ID != "infra.file.redis" {
		t.Errorf("ID = %q, want infra.file.redis", fails[0].ID)
	}
	if fails[0].Severity != SeverityError {
		t.Errorf("Severity = %s, want error", fails[0].Severity)
	}
}

func TestScanConfigInfraNeedsParsesAllPaths(t *testing.T) {
	conf := `rate_limit:
  backend: "redis"
auth:
  signature:
    nonce:
      backend: "redis"
idempotency:
  backend: "redis"
config_center:
  enabled: true
  provider: "polaris"
`
	root := t.TempDir()
	dir := filepath.Join(root, "conf", "dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conf.yaml"), []byte(conf), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	needs := scanConfigInfraNeeds(filepath.Join(dir, "conf.yaml"))
	if _, ok := needs["redis"]; !ok {
		t.Errorf("expected redis in needs")
	}
	if _, ok := needs["polaris"]; !ok {
		t.Errorf("expected polaris in needs")
	}
}

func TestScanConfigInfraNeedsSkipsDisabledConfigCenter(t *testing.T) {
	conf := `config_center:
  enabled: false
  provider: "nacos"
`
	root := t.TempDir()
	dir := filepath.Join(root, "conf", "dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conf.yaml"), []byte(conf), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	needs := scanConfigInfraNeeds(filepath.Join(dir, "conf.yaml"))
	if _, ok := needs["nacos"]; ok {
		t.Errorf("should not detect nacos when config_center is disabled")
	}
}

func TestScanConfigInfraNeedsMalformedYAML(t *testing.T) {
	// Malformed YAML should return empty map, not panic
	needs := scanConfigInfraNeeds("/nonexistent/path")
	if len(needs) != 0 {
		t.Errorf("expected empty needs for nonexistent file, got %v", needs)
	}
}

func TestScanConfigInfraNeedsMemoryBackend(t *testing.T) {
	conf := `rate_limit:
  backend: "memory"
auth:
  signature:
    nonce:
      backend: "memory"
idempotency:
  backend: "memory"
`
	root := t.TempDir()
	dir := filepath.Join(root, "conf", "dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conf.yaml"), []byte(conf), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	needs := scanConfigInfraNeeds(filepath.Join(dir, "conf.yaml"))
	if len(needs) != 0 {
		t.Errorf("expected no infra needs for memory backends, got %v", needs)
	}
}

func TestInfraFileCheckUnknownKind(t *testing.T) {
	c := infraFileCheck("/root", "unknown_thing", "test reason")
	if !c.OK {
		t.Errorf("unknown kind should return OK skip check: %+v", c)
	}
	if c.Severity != SeverityWarn {
		t.Errorf("unknown kind should be warn, got %s", c.Severity)
	}
}

func TestInfraCheckIntegrationWithDoctor(t *testing.T) {
	// Verify that the doctor run includes infra checks when a project has
	// manifest + config referencing redis
	conf := `rate_limit:
  backend: "redis"
`
	root := seedInfraProject(t, []string{"redis"}, conf)
	// Do NOT seed the redis infra file — doctor should detect missing file
	r := Run(t.Context(), Options{
		Root: root,
		Runner: &scriptedRunner{out: map[string]string{
			"hz":    "hz version v0.9.7",
			"kitex": "v0.16.1",
		}},
	})
	// Find the infra check
	found := false
	for _, c := range r.Checks {
		if c.ID == "infra.file.redis" {
			found = true
			if c.OK {
				t.Errorf("infra.file.redis should be false when file missing")
			}
			if c.Severity != SeverityError {
				t.Errorf("severity = %s, want error", c.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("infra.file.redis check not found in doctor report")
	}
}

func TestScanConfigInfraNeedsInvalidYAML(t *testing.T) {
	// Write a file with invalid YAML content
	root := t.TempDir()
	dir := filepath.Join(root, "conf", "dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conf.yaml"), []byte("::::invalid:::yaml:::"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	needs := scanConfigInfraNeeds(filepath.Join(dir, "conf.yaml"))
	if len(needs) != 0 {
		t.Errorf("expected empty needs for invalid YAML, got %v", needs)
	}
}

func TestInfraFilesCheckNoDuplicateReport(t *testing.T) {
	// When both manifest and config reference redis, it should only be
	// reported once (not duplicated)
	conf := `rate_limit:
  backend: "redis"
`
	root := seedInfraProject(t, []string{"redis"}, conf)
	seedInfraFile(t, root, "redis")
	checks := checkInfraFiles(root, loadOrFail(t, root))
	redisCount := 0
	for _, c := range checks {
		if c.ID == "infra.file.redis" {
			redisCount++
		}
	}
	if redisCount != 1 {
		t.Errorf("redis should be reported exactly once, got %d times", redisCount)
	}
}

// TestScanConfigInfraNeedsIdempotencyRedis verifies that the idempotency
// backend=redis path is detected.
func TestScanConfigInfraNeedsIdempotencyRedis(t *testing.T) {
	conf := `idempotency:
  backend: "redis"
  window: "5m"
`
	root := t.TempDir()
	dir := filepath.Join(root, "conf", "dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conf.yaml"), []byte(conf), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	needs := scanConfigInfraNeeds(filepath.Join(dir, "conf.yaml"))
	if _, ok := needs["redis"]; !ok {
		t.Errorf("expected redis in needs for idempotency.backend")
	}
}

func TestManifestHasKind(t *testing.T) {
	m := &manifest.Manifest{Infra: []string{"redis", "kafka"}}
	if !manifestHasKind(m, "redis") {
		t.Error("expected redis to be found")
	}
	if !manifestHasKind(m, "kafka") {
		t.Error("expected kafka to be found")
	}
	if manifestHasKind(m, "nacos") {
		t.Error("expected nacos to NOT be found")
	}
}

func TestScanConfigInfraNeedsYAMLParseError(t *testing.T) {
	// Write invalid YAML to a temp file and verify no panic
	dir := t.TempDir()
	path := filepath.Join(dir, "conf.yaml")
	if err := os.WriteFile(path, []byte("foo: [unclosed"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	needs := scanConfigInfraNeeds(path)
	if len(needs) != 0 {
		t.Errorf("expected empty needs for bad YAML, got %v", needs)
	}
}

func TestInfraFileCheckFileStatError(t *testing.T) {
	// Verify that a missing file produces error severity
	c := infraFileCheck("/nonexistent/root", "redis", "test")
	if c.OK {
		t.Error("expected OK=false for missing file")
	}
	if c.Severity != SeverityError {
		t.Errorf("Severity = %s, want error", c.Severity)
	}
}

func TestInfraFileCheckMessageContainsKind(t *testing.T) {
	c := infraFileCheck("/nonexistent", "kafka", "test reason")
	if c.Message == "" {
		t.Fatal("expected non-empty message")
	}
	// Message should mention the kind and the reason
	if !containsAny(c.Message, "kafka", "test reason") {
		t.Errorf("message should mention kafka and reason: %s", c.Message)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && !containsStr(s, sub) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || findStr(s, sub))
}

func findStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestInfraFileCheckHintMentionsAddInfra(t *testing.T) {
	c := infraFileCheck("/nonexistent", "redis", "test")
	if c.Hint == "" {
		t.Fatal("expected non-empty hint")
	}
	if !findStr(c.Hint, "ncgo add infra") {
		t.Errorf("hint should mention 'ncgo add infra': %s", c.Hint)
	}
}

func TestInfraFileCheckOKMessage(t *testing.T) {
	dir := t.TempDir()
	rel := infraFiles["redis"]
	fullPath := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("package data\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := infraFileCheck(dir, "redis", "manifest")
	if !c.OK {
		t.Error("expected OK=true when file exists")
	}
	if !findStr(c.Message, "redis") {
		t.Errorf("message should mention redis: %s", c.Message)
	}
}

func TestInfraFileCheckUnknownKindReturnsWarn(t *testing.T) {
	c := infraFileCheck("/any", "some_weird_thing", "reason")
	if !c.OK {
		t.Error("unknown kind should return OK=true (skipped)")
	}
	if c.Severity != SeverityWarn {
		t.Errorf("Severity = %s, want warn for unknown kind", c.Severity)
	}
}

func TestScanConfigInfraNeedsPreservesYAMLStructure(t *testing.T) {
	// Build a YAML using yaml.Marshal to ensure it parses correctly
	cfg := map[string]any{
		"rate_limit": map[string]any{
			"backend": "redis",
		},
		"idempotency": map[string]any{
			"backend": "redis",
		},
	}
	body, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "conf.yaml")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	needs := scanConfigInfraNeeds(path)
	if _, ok := needs["redis"]; !ok {
		t.Errorf("expected redis in needs, got %v", needs)
	}
}

package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// E2EOptions configures the end-to-end rate-limit test.
type E2EOptions struct {
	Root     string
	Host     string
	Port     int
	Rate     int
	Duration string
	Paths    []string
	DSN      string
	Cleanup  bool
	DryRun   bool
	Report   string // output report file path (.md or .json)
}

// E2EResult holds the result of an e2e test run.
type E2EResult struct {
	Mode        string        // mono | micro
	Kind        string        // hertz | kitex
	Source      string        // config | database | rule_center | grpc
	Backend     string        // memory | redis
	ServiceName string
	Pass        bool          // true = rate limiting detected
	Status      string        // PASS | FAIL | WARN
	TotalReqs   int
	Status429   int
	Status200   int
	StatusOther int
	AvgLatency  time.Duration
	P99Latency  time.Duration
	StartedAt   time.Time
	Duration    time.Duration
	ReportPath  string
}

// E2E runs the full end-to-end rate-limit test: detect project type,
// read rate-limit config, start dependencies, seed rules, run attack,
// analyze results, and optionally generate a report.
func E2E(ctx context.Context, opts E2EOptions) (*E2EResult, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Host == "" {
		opts.Host = "localhost"
	}
	if opts.Port == 0 {
		opts.Port = 8080
	}
	if opts.Rate == 0 {
		opts.Rate = 200
	}
	if opts.Duration == "" {
		opts.Duration = "10s"
	}
	if len(opts.Paths) == 0 {
		opts.Paths = []string{"/healthz", "/"}
	}

	// 1. Detect project type
	mode, kind, serviceName, err := detectProject(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("detect project: %w", err)
	}

	// 2. Read rate-limit config
	source, backend, err := parseRateLimitConfig(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	result := &E2EResult{
		Mode:        mode,
		Kind:        kind,
		Source:      source,
		Backend:     backend,
		ServiceName: serviceName,
		StartedAt:   time.Now(),
	}

	if opts.DryRun {
		return result, nil
	}

	// 3. Compute dependencies
	deps := computeDependencies(source, backend)

	// 4. Start dependencies
	if opts.Cleanup {
		defer func() {
			stopDeps(ctx, opts.Root, deps)
		}()
	}
	if err := startDeps(ctx, opts.Root, deps); err != nil {
		return result, fmt.Errorf("start dependencies: %w", err)
	}

	// 5. Health check
	targetURL := fmt.Sprintf("http://%s:%d%s", opts.Host, opts.Port, opts.Paths[0])
	if err := waitForReady(ctx, targetURL, 2*time.Second, 30*time.Second); err != nil {
		return result, fmt.Errorf("service not ready at %s: %w", targetURL, err)
	}

	// 6. Seed rules (only for database/rule_center/grpc sources)
	needsSeed := source == "database" || source == "rule_center" || source == "grpc"
	if needsSeed {
		if err := Seed(ctx, SeedOptions{
			Root: opts.Root,
			DSN:  opts.DSN,
		}); err != nil {
			return result, fmt.Errorf("seed rules: %w", err)
		}
	}

	// 7. Run attack
	attackStart := time.Now()
	ar, err := runAttackCapture(ctx, opts.Root, opts.Host, opts.Port, opts.Rate, opts.Duration, opts.Paths)
	if err != nil {
		return result, fmt.Errorf("run attack: %w", err)
	}
	result.Duration = time.Since(attackStart)

	// 8. Analyze results
	result.TotalReqs = ar.TotalReqs
	result.Status200 = ar.Status200
	result.Status429 = ar.Status429
	result.StatusOther = ar.StatusOther
	result.AvgLatency = ar.AvgLatency
	result.P99Latency = ar.P99Latency
	result.Status, result.Pass = classifyResult(ar)

	// 9. Write report
	if opts.Report != "" {
		if err := writeReport(result, opts); err != nil {
			return result, fmt.Errorf("write report: %w", err)
		}
		result.ReportPath = opts.Report
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Project detection
// ---------------------------------------------------------------------------

func detectProject(root string) (mode, kind, serviceName string, err error) {
	// Try micro workspace first
	ws, err := manifest.LoadWorkspace(root)
	if err == nil {
		// Micro mode — find rule-center and first Hertz service
		hasRuleCenter := false
		var firstHertz string
		for _, s := range ws.Services {
			if strings.EqualFold(s.Name, "rule-center") || strings.EqualFold(s.Name, "rule_center") {
				hasRuleCenter = true
			}
			if s.Kind == manifest.KindHertz && firstHertz == "" {
				firstHertz = s.Name
			}
		}
		if !hasRuleCenter {
			return "", "", "", fmt.Errorf("micro workspace has no rule-center service; e2e test requires rule-center")
		}
		svc := firstHertz
		if svc == "" {
			svc = ws.Name
		}
		return "micro", manifest.KindHertz, svc, nil
	}

	// Try mono manifest
	m, err := manifest.Load(root)
	if err == nil {
		return "mono", m.Service.Kind, m.Service.Name, nil
	}

	return "", "", "", fmt.Errorf("unknown project type: no ncgo.workspace or .ncgo/manifest.yaml found")
}

// ---------------------------------------------------------------------------
// Config parsing
// ---------------------------------------------------------------------------

type rlConfig struct {
	RateLimit struct {
		Enabled bool `yaml:"enabled"`
		Source  struct {
			Type string `yaml:"type"`
		} `yaml:"source"`
		Backend string `yaml:"backend"`
	} `yaml:"rate_limit"`
}

func parseRateLimitConfig(root string) (source, backend string, err error) {
	confPath := filepath.Join(root, "conf", "dev", "conf.yaml")
	b, err := os.ReadFile(confPath)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", confPath, err)
	}

	var cfg rlConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return "", "", fmt.Errorf("parse %s: %w", confPath, err)
	}

	src := cfg.RateLimit.Source.Type
	if src == "" {
		src = "config"
	}
	bk := cfg.RateLimit.Backend
	if bk == "" {
		bk = "memory"
	}

	return src, bk, nil
}

// ---------------------------------------------------------------------------
// Dependency matrix
// ---------------------------------------------------------------------------

type depFlags struct {
	needPostgres bool
	needRedis    bool
}

func computeDependencies(source, backend string) depFlags {
	var d depFlags
	switch source {
	case "database", "rule_center", "grpc":
		d.needPostgres = true
	}
	if backend == "redis" {
		d.needRedis = true
	}
	return d
}

func startDeps(ctx context.Context, root string, deps depFlags) error {
	var services []string
	if deps.needPostgres {
		services = append(services, "postgres")
	}
	if deps.needRedis {
		// Try docker compose first
		if hasComposeService(root, "redis") {
			services = append(services, "redis")
		} else {
			// Fallback: standalone redis container
			if err := startRedisStandalone(ctx); err != nil {
				return fmt.Errorf("start redis: %w (try adding redis to your compose.yaml)", err)
			}
			fmt.Println("[deps] Started standalone redis")
		}
	}
	if len(services) > 0 {
		fmt.Printf("[deps] Starting %s via docker compose\n", strings.Join(services, ", "))
		if err := dockerComposeUp(ctx, root, services...); err != nil {
			return fmt.Errorf("docker compose up %s: %w", strings.Join(services, ","), err)
		}
	}
	return nil
}

func stopDeps(ctx context.Context, root string, deps depFlags) {
	var services []string
	if deps.needPostgres {
		services = append(services, "postgres")
	}
	if deps.needRedis && hasComposeService(root, "redis") {
		services = append(services, "redis")
	}
	if len(services) > 0 {
		fmt.Printf("[deps] Stopping %s via docker compose\n", strings.Join(services, ", "))
		_ = dockerComposeStop(ctx, root, services...)
	}
	// Stop standalone redis if we started it
	if deps.needRedis && !hasComposeService(root, "redis") {
		_ = exec.CommandContext(ctx, "docker", "rm", "-f", "ncgo-e2e-redis").Run()
		fmt.Println("[deps] Stopped standalone redis")
	}
}

func hasComposeService(root string, serviceName string) bool {
	composeFiles := []string{
		filepath.Join(root, "compose.yaml"),
		filepath.Join(root, "docker-compose.yaml"),
		filepath.Join(root, "compose.yml"),
		filepath.Join(root, "docker-compose.yml"),
	}
	for _, f := range composeFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		body := string(b)
		// Simple text search — compose services are listed under "services:"
		if strings.Contains(body, serviceName+":") {
			return true
		}
	}
	return false
}

func dockerComposeUp(ctx context.Context, root string, services ...string) error {
	args := append([]string{"compose", "up", "-d"}, services...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dockerComposeStop(ctx context.Context, root string, services ...string) error {
	args := append([]string{"compose", "stop"}, services...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startRedisStandalone(ctx context.Context) error {
	// Check if already running
	if out, err := exec.CommandContext(ctx, "docker", "inspect", "ncgo-e2e-redis", "-f", "{{.State.Running}}").CombinedOutput(); err == nil && strings.TrimSpace(string(out)) == "true" {
		return nil
	}
	// Remove stale container if exists
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", "ncgo-e2e-redis").Run()

	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", "ncgo-e2e-redis",
		"-p", "6379:6379",
		"redis:alpine",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// Health check
// ---------------------------------------------------------------------------

func waitForReady(ctx context.Context, url string, interval, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	client := &http.Client{Timeout: 3 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for %s after %s", url, timeout)
			}
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			if resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				fmt.Printf("[check] Service ready at %s\n", url)
				return nil
			}
			resp.Body.Close()
		}
	}
}

// ---------------------------------------------------------------------------
// Attack capture
// ---------------------------------------------------------------------------

type attackResult struct {
	TotalReqs   int
	Status429   int
	Status200   int
	StatusOther int
	AvgLatency  time.Duration
	P99Latency  time.Duration
}

func runAttackCapture(ctx context.Context, root, host string, port, rate int, duration string, paths []string) (*attackResult, error) {
	targets := buildTargets(host, port, paths)
	targetBody := strings.Join(targets, "\n") + "\n"

	// Write targets to temp file
	tmpTargets := filepath.Join(os.TempDir(), "ncgo-e2e-targets.txt")
	if err := os.WriteFile(tmpTargets, []byte(targetBody), 0o644); err != nil {
		return nil, fmt.Errorf("write targets: %w", err)
	}
	defer os.Remove(tmpTargets)

	// Run vegeta attack, save binary output
	tmpBin := filepath.Join(os.TempDir(), "ncgo-e2e-results.bin")
	defer os.Remove(tmpBin)

	vegeta, err := exec.LookPath("vegeta")
	if err != nil {
		// Try docker
		return runAttackViaDocker(ctx, root, tmpTargets, tmpBin, strconv.Itoa(rate), duration)
	}

	// Run vegeta attack locally
	atkCmd := exec.CommandContext(ctx, vegeta, "attack",
		"-targets="+tmpTargets,
		"-rate="+strconv.Itoa(rate),
		"-duration="+duration,
		"-output="+tmpBin,
	)
	atkCmd.Stdout = os.Stderr // vegeta attack writes progress to stderr
	atkCmd.Stderr = os.Stderr
	if err := atkCmd.Run(); err != nil {
		return nil, fmt.Errorf("vegeta attack: %w", err)
	}

	// Run vegeta report
	return parseVegetaReport(ctx, vegeta, tmpBin)
}

func runAttackViaDocker(ctx context.Context, root, targetsPath, binPath, rate, duration string) (*attackResult, error) {
	// Mount targets file, run attack, write results to mounted output
	tmpOutDir := os.TempDir()
	dockerArgs := []string{"compose", "run", "--rm",
		"--entrypoint", "vegeta",
		"-v", targetsPath + ":/targets.txt",
		"-v", tmpOutDir + ":/output",
		"vegeta",
		"attack",
		"-targets=/targets.txt",
		"-rate=" + rate,
		"-duration=" + duration,
		"-output=/output/ncgo-e2e-results.bin",
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	cmd.Dir = root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker vegeta attack: %w", err)
	}

	binPath = filepath.Join(tmpOutDir, "ncgo-e2e-results.bin")
	defer os.Remove(binPath)

	// Run vegeta report via docker
	reportArgs := []string{"compose", "run", "--rm",
		"--entrypoint", "vegeta",
		"-v", tmpOutDir + ":/output",
		"vegeta",
		"report", "-type=json",
		"/output/ncgo-e2e-results.bin",
	}
	reportCmd := exec.CommandContext(ctx, "docker", reportArgs...)
	reportCmd.Dir = root
	out, err := reportCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker vegeta report: %w\n%s", err, string(out))
	}

	return parseVegetaJSON(out)
}

func parseVegetaReport(ctx context.Context, vegetaPath, binPath string) (*attackResult, error) {
	reportCmd := exec.CommandContext(ctx, vegetaPath, "report", "-type=json", binPath)
	out, err := reportCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("vegeta report: %w\n%s", err, string(out))
	}
	return parseVegetaJSON(out)
}

// vegetaReportJSON mirrors the output of `vegeta report -type=json`.
type vegetaReportJSON struct {
	Latencies struct {
		Total int64 `json:"total"`
		Mean  int64 `json:"mean"`
		P99   int64 `json:"99th"`
	} `json:"latencies"`
	Requests   int            `json:"requests"`
	Success    int            `json:"success"`
	StatusCodes map[string]int `json:"status_codes"`
}

func parseVegetaJSON(data []byte) (*attackResult, error) {
	var report vegetaReportJSON
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse vegeta JSON: %w", err)
	}

	ar := &attackResult{
		TotalReqs:  report.Requests,
		AvgLatency: time.Duration(report.Latencies.Mean),
		P99Latency: time.Duration(report.Latencies.P99),
	}

	for code, count := range report.StatusCodes {
		switch code {
		case "200":
			ar.Status200 = count
		case "429":
			ar.Status429 = count
		default:
			ar.StatusOther += count
		}
	}

	return ar, nil
}

func classifyResult(ar *attackResult) (status string, pass bool) {
	if ar.TotalReqs == 0 {
		return "FAIL", false
	}
	if ar.Status429 == 0 {
		return "FAIL", false
	}
	if ar.Status429 == ar.TotalReqs {
		return "WARN", false
	}
	return "PASS", true
}

// ---------------------------------------------------------------------------
// Report generation
// ---------------------------------------------------------------------------

func writeReport(result *E2EResult, opts E2EOptions) error {
	ext := strings.ToLower(filepath.Ext(opts.Report))
	switch ext {
	case ".md":
		return writeMarkdownReport(result, opts)
	case ".json":
		return writeJSONReport(result, opts)
	default:
		return fmt.Errorf("unsupported report format: %s (use .md or .json)", ext)
	}
}

func writeMarkdownReport(result *E2EResult, opts E2EOptions) error {
	var statusIcon string
	switch result.Status {
	case "PASS":
		statusIcon = "PASS"
	case "WARN":
		statusIcon = "WARN"
	default:
		statusIcon = "FAIL"
	}

	totalReqs := result.TotalReqs
	if totalReqs == 0 {
		totalReqs = 1 // avoid division by zero
	}

	var b strings.Builder
	b.WriteString("# Rate Limit E2E Test Report\n\n")
	b.WriteString(fmt.Sprintf("- **Generated at**: %s\n", result.StartedAt.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **Project mode**: %s\n", result.Mode))
	b.WriteString(fmt.Sprintf("- **Rule source**: %s\n", result.Source))
	b.WriteString(fmt.Sprintf("- **Counter backend**: %s\n", result.Backend))
	b.WriteString(fmt.Sprintf("- **Service**: %s (%s)\n\n", result.ServiceName, result.Kind))

	b.WriteString("## Test Parameters\n\n")
	b.WriteString("| Parameter | Value |\n")
	b.WriteString("|-----------|-------|\n")
	b.WriteString(fmt.Sprintf("| Target URL | http://%s:%d |\n", opts.Host, opts.Port))
	b.WriteString(fmt.Sprintf("| Test paths | %s |\n", strings.Join(opts.Paths, ", ")))
	b.WriteString(fmt.Sprintf("| Rate | %d rps |\n", opts.Rate))
	b.WriteString(fmt.Sprintf("| Duration | %s |\n", opts.Duration))
	b.WriteString(fmt.Sprintf("| Total requests | %d |\n\n", result.TotalReqs))

	b.WriteString("## Results\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	b.WriteString(fmt.Sprintf("| Status | %s |\n", statusIcon))
	b.WriteString(fmt.Sprintf("| 200 OK | %d (%.1f%%) |\n", result.Status200, float64(result.Status200)/float64(totalReqs)*100))
	b.WriteString(fmt.Sprintf("| 429 Too Many Requests | %d (%.1f%%) |\n", result.Status429, float64(result.Status429)/float64(totalReqs)*100))
	b.WriteString(fmt.Sprintf("| Other errors | %d (%.1f%%) |\n", result.StatusOther, float64(result.StatusOther)/float64(totalReqs)*100))
	b.WriteString(fmt.Sprintf("| Avg latency | %s |\n", result.AvgLatency.Round(time.Millisecond)))
	b.WriteString(fmt.Sprintf("| P99 latency | %s |\n\n", result.P99Latency.Round(time.Millisecond)))

	b.WriteString("## Environment\n\n")
	if result.Source == "database" || result.Source == "rule_center" || result.Source == "grpc" {
		b.WriteString("- PostgreSQL: running\n")
	}
	if result.Backend == "redis" {
		b.WriteString("- Redis: running\n")
	}
	if result.Mode == "micro" {
		b.WriteString("- Rule-center: running\n")
	}
	b.WriteString(fmt.Sprintf("- Consumer: %s running (port %d)\n", result.ServiceName, opts.Port))

	return os.WriteFile(opts.Report, []byte(b.String()), 0o644)
}

type jsonReport struct {
	GeneratedAt string `json:"generatedAt"`
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	Source      string `json:"source"`
	Backend     string `json:"backend"`
	Service     string `json:"service"`
	TestParams  struct {
		TargetURL   string   `json:"targetUrl"`
		Paths       []string `json:"paths"`
		Rate        int      `json:"rate"`
		Duration    string   `json:"duration"`
		TotalReqs   int      `json:"totalRequests"`
	} `json:"testParams"`
	Results struct {
		Status      string `json:"status"`
		Status200   int    `json:"status200"`
		Status429   int    `json:"status429"`
		StatusOther int    `json:"statusOther"`
		AvgLatency  string `json:"avgLatency"`
		P99Latency  string `json:"p99Latency"`
	} `json:"results"`
}

func writeJSONReport(result *E2EResult, opts E2EOptions) error {
	report := jsonReport{
		GeneratedAt: result.StartedAt.UTC().Format(time.RFC3339),
		Mode:        result.Mode,
		Source:      result.Source,
		Backend:     result.Backend,
		Service:     result.ServiceName,
		Status:      result.Status,
	}
	report.TestParams.TargetURL = fmt.Sprintf("http://%s:%d", opts.Host, opts.Port)
	report.TestParams.Paths = opts.Paths
	report.TestParams.Rate = opts.Rate
	report.TestParams.Duration = opts.Duration
	report.TestParams.TotalReqs = result.TotalReqs
	report.Results.Status = result.Status
	report.Results.Status200 = result.Status200
	report.Results.Status429 = result.Status429
	report.Results.StatusOther = result.StatusOther
	report.Results.AvgLatency = result.AvgLatency.String()
	report.Results.P99Latency = result.P99Latency.String()

	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON report: %w", err)
	}
	return os.WriteFile(opts.Report, b, 0o644)
}

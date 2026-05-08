// Package infra implements `ncgo add infra <kind>`.
//
// The optional add-on files in internal/assets/_data/optional/ or
// internal/assets/_data/<framework>/optional/ are copied into the project.
// Most are written verbatim; a small set of Hertz helper assets also render
// `{{.GoModule}}` placeholders before write. Most add-ons land in
// internal/base/data/<kind>.go; specialized add-ons may target packages such as
// internal/base/registry, internal/base/observability, or
// internal/base/logging. Hertz data add-ons may also emit example config
// snippets under conf/dev/*.yaml. Add updates the manifest's infra list and
// prints the setup commands the user must run.
package infra

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
	planpkg "github.com/byx-darwin/ncgo/internal/scaffold/plan"
)

// Kind values supported by `ncgo add infra`. The corresponding template files
// live in internal/assets/_data/<framework>/optional/ and ship with the binary.
const (
	KindRedis             = "redis"
	KindKafka             = "kafka"
	KindES                = "es"
	KindClickHouse        = "clickhouse"
	KindRegistryEtcd      = "registry_etcd"
	KindObservabilityOtel = "observability_otel"
	KindOtelAlias         = "otel"
	KindObservabilityLog  = "observability_logging"
	KindLoggingAlias      = "logging"
	KindReleaseCanary     = "release_canary"
	KindCanaryAlias       = "canary"
)

// SupportedKinds returns all add-on names in canonical order. Some kinds are
// service-kind specific; Add validates that after loading the manifest.
func SupportedKinds() []string {
	return []string{KindRedis, KindKafka, KindES, KindClickHouse, KindObservabilityOtel, KindOtelAlias, KindObservabilityLog, KindLoggingAlias, KindReleaseCanary, KindCanaryAlias, KindRegistryEtcd}
}

func commonKinds() []string {
	return []string{KindRedis, KindKafka, KindES, KindClickHouse, KindObservabilityOtel, KindObservabilityLog, KindReleaseCanary}
}

func kitexOnlyKinds() []string {
	return []string{KindRegistryEtcd}
}

// goGetDeps is the source of truth for `go get` dependency next-steps. Keeping
// it here rather than parsing the file header avoids relying on free-form
// comments.
var goGetDeps = map[string][]string{
	KindRedis:        {"github.com/redis/go-redis/v9", "github.com/samber/oops"},
	KindKafka:        {"github.com/segmentio/kafka-go", "github.com/samber/oops"},
	KindES:           {"github.com/elastic/go-elasticsearch/v8", "github.com/samber/oops"},
	KindClickHouse:   {"github.com/ClickHouse/clickhouse-go/v2", "github.com/samber/oops"},
	KindRegistryEtcd: {"github.com/kitex-contrib/registry-etcd", "github.com/samber/oops"},
	KindObservabilityLog: {
		"github.com/samber/oops",
		"gopkg.in/natefinch/lumberjack.v2",
		"go.opentelemetry.io/otel/trace",
	},
}

var setupSteps = map[string][]string{
	KindObservabilityOtel: {
		"curl -fsSL https://cdn.jsdelivr.net/gh/alibaba/loongsuite-go-agent@main/install.sh | sudo bash",
		"otel version",
		"otel go build ./...",
		"OTEL_SERVICE_NAME=<service> OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 ./<your-binary>",
	},
}

var commonAssetKinds = map[string]bool{
	KindObservabilityOtel: true,
	KindObservabilityLog:  true,
	KindReleaseCanary:     true,
}

var outputRelPaths = map[string]string{
	KindRedis:             filepath.Join("internal", "base", "data", "redis.go"),
	KindKafka:             filepath.Join("internal", "base", "data", "kafka.go"),
	KindES:                filepath.Join("internal", "base", "data", "es.go"),
	KindClickHouse:        filepath.Join("internal", "base", "data", "clickhouse.go"),
	KindRegistryEtcd:      filepath.Join("internal", "base", "registry", "etcd.go"),
	KindObservabilityOtel: filepath.Join("internal", "base", "observability", "otel.go"),
	KindObservabilityLog:  filepath.Join("internal", "base", "logging", "logging.go"),
	KindReleaseCanary:     filepath.Join("internal", "base", "release", "canary.go"),
}

const hertzRedisSharedHelperRelPath = "internal/base/data/redis_shared.go"

const hertzConfigRelPath = "conf/dev/conf.yaml"

var hertzConfigSnippetKeys = map[string]string{
	KindRedis:      "redis",
	KindKafka:      "kafka",
	KindES:         "es",
	KindClickHouse: "clickhouse",
}

// Options configures Add.
type Options struct {
	Root   string // project root containing .ncgo/manifest.yaml
	Kind   string // one of the SupportedKinds values
	Force  bool   // overwrite existing generated add-on file
	Wire   bool   // update generated server/client wiring when supported
	DryRun bool   // report intended writes/wiring without modifying files
}

// Result describes what Add produced.
type Result struct {
	WrittenPath  string   // absolute path of the first created/overwritten file
	WrittenPaths []string // absolute paths of all created/overwritten files
	WiredPaths   []string // absolute paths of source files updated by --wire
	NextSteps    []string // shell commands the user/agent should run next
	Plan         []PlanItem
	Updated      bool // true when manifest.Infra changed
	DryRun       bool // true when no files were modified
}

// PlanItem is a machine-readable summary of Add's intended or completed work.
type PlanItem = planpkg.Item

// Add validates opts, copies the embedded add-on into the project, and
// updates the manifest. It does NOT call `go get`; the caller is expected to
// follow the printed next-steps.
func Add(opts Options) (*Result, error) {
	kind, err := normalizeKind(opts.Kind)
	if err != nil {
		return nil, err
	}
	if opts.Root == "" {
		return nil, errors.New("infra: Root is required")
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("infra: resolve root: %w", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	files, err := assetFiles(m.Service.Kind, kind)
	if err != nil {
		return nil, err
	}
	files, err = appendHertzRedisHelperIfMissing(files, root, m.Service.Kind, kind)
	if err != nil {
		return nil, err
	}
	if opts.Wire && !wireSupportedKind(kind) {
		return nil, unsupportedWireError()
	}
	writes := make([]plannedWrite, 0, len(files)+1)
	paths := make([]string, 0, len(files)+1)
	filePlans := make([]PlanItem, 0, len(files)+1)
	for _, file := range files {
		body, err := fs.ReadFile(assets.FS(), file.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("infra: read embedded %s: %w", file.SourcePath, err)
		}
		body = renderAssetBody(body, m.Module)
		dst := filepath.Join(root, file.OutputRelPath)
		action, err := plannedFileAction(dst, opts.Force)
		if err != nil {
			return nil, err
		}
		writes = append(writes, plannedWrite{Path: dst, Body: body, Action: action})
		paths = append(paths, dst)
		filePlans = append(filePlans, PlanItem{Kind: "file", Action: action, Path: dst})
	}
	confWrite, err := planHertzConfigWrite(root, m.Service.Kind, kind, opts.Force)
	if err != nil {
		return nil, err
	}
	if confWrite != nil {
		writes = append(writes, *confWrite)
		paths = append(paths, confWrite.Path)
		filePlans = append(filePlans, PlanItem{Kind: "file", Action: confWrite.Action, Path: confWrite.Path})
	}
	wiredPaths := []string(nil)
	wirePlans := []PlanItem(nil)
	if opts.Wire {
		wiredPaths, wirePlans, err = PreviewWirePlan(root, m.Module, m.Service.Kind, kind)
		if err != nil {
			return nil, err
		}
	}
	if !opts.DryRun {
		for _, w := range writes {
			if err := writeFile(w.Path, w.Body); err != nil {
				return nil, err
			}
		}
	}
	updated := !manifestHasInfra(m, kind)
	if updated {
		mergeInfra(m, kind)
	}
	if updated && !opts.DryRun {
		if err := manifest.Save(root, m); err != nil {
			return nil, err
		}
	}
	if opts.Wire && !opts.DryRun {
		wiredPaths, err = Wire(root, m.Module, m.Service.Kind, kind)
		if err != nil {
			return nil, err
		}
	}
	next := nextSteps(kind, m.Service.Kind, m.Service.Name)
	return &Result{
		WrittenPath:  paths[0],
		WrittenPaths: paths,
		WiredPaths:   wiredPaths,
		NextSteps:    next,
		Plan:         buildPlan(filePlans, updated, opts.Wire, wiredPaths, wirePlans, next),
		Updated:      updated,
		DryRun:       opts.DryRun,
	}, nil
}

type addOnFile struct {
	SourcePath    string
	OutputRelPath string
}

type plannedWrite struct {
	Path   string
	Body   []byte
	Action string
}

func assetFiles(serviceKind, infraKind string) ([]addOnFile, error) {
	if infraKind == KindObservabilityLog || infraKind == KindReleaseCanary {
		files := []addOnFile{{
			SourcePath:    "optional/" + infraKind + ".go",
			OutputRelPath: outputRelPaths[infraKind],
		}}
		adapterName := frameworkAdapterName(infraKind, serviceKind)
		switch serviceKind {
		case manifest.KindHertz:
			files = append(files, addOnFile{SourcePath: serviceKind + "/optional/" + infraKind + ".go", OutputRelPath: adapterName})
		case manifest.KindKitex:
			files = append(files, addOnFile{SourcePath: serviceKind + "/optional/" + infraKind + ".go", OutputRelPath: adapterName})
		default:
			return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
		}
		return files, nil
	}
	srcPath, err := assetPath(serviceKind, infraKind)
	if err != nil {
		return nil, err
	}
	rel, ok := outputRelPaths[infraKind]
	if !ok {
		return nil, fmt.Errorf("infra: kind %q has no output path", infraKind)
	}
	return []addOnFile{{SourcePath: srcPath, OutputRelPath: rel}}, nil
}

func appendHertzRedisHelperIfMissing(files []addOnFile, root, serviceKind, infraKind string) ([]addOnFile, error) {
	if serviceKind != manifest.KindHertz || infraKind != KindRedis {
		return files, nil
	}
	helperPath := filepath.Join(root, filepath.FromSlash(hertzRedisSharedHelperRelPath))
	if _, err := os.Stat(helperPath); err == nil {
		return files, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("infra: stat %s: %w", helperPath, err)
	}
	return append(files, addOnFile{SourcePath: "hertz/optional/redis_shared.go", OutputRelPath: filepath.FromSlash(hertzRedisSharedHelperRelPath)}), nil
}

func renderAssetBody(body []byte, module string) []byte {
	if module == "" {
		return body
	}
	rendered := strings.ReplaceAll(string(body), "{{.GoModule}}", module)
	return []byte(rendered)
}

func planHertzConfigWrite(root, serviceKind, infraKind string, force bool) (*plannedWrite, error) {
	if serviceKind != manifest.KindHertz {
		return nil, nil
	}
	if _, ok := hertzConfigSnippetKeys[infraKind]; !ok {
		return nil, nil
	}
	snippet, err := fs.ReadFile(assets.FS(), filepath.ToSlash(filepath.Join("hertz", "optional-config", infraKind+".yaml")))
	if err != nil {
		return nil, fmt.Errorf("infra: read embedded hertz/optional-config/%s.yaml: %w", infraKind, err)
	}
	path := filepath.Join(root, filepath.FromSlash(hertzConfigRelPath))
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &plannedWrite{
				Path:   path,
				Body:   []byte(wrapHertzConfigSnippet(string(snippet), infraKind) + "\n"),
				Action: "create",
			}, nil
		}
		return nil, fmt.Errorf("infra: read %s: %w", path, err)
	}
	merged, changed, err := mergeHertzConfig(current, string(snippet), infraKind, force)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, nil
	}
	return &plannedWrite{Path: path, Body: []byte(merged), Action: "update"}, nil
}

func mergeHertzConfig(current []byte, snippet, infraKind string, force bool) (string, bool, error) {
	src := string(current)
	startMarker, endMarker := hertzConfigMarkers(infraKind)
	if strings.Contains(src, startMarker) || strings.Contains(src, endMarker) {
		if !strings.Contains(src, startMarker) || !strings.Contains(src, endMarker) {
			return "", false, fmt.Errorf("infra: malformed config markers for %q in %s", infraKind, filepath.FromSlash(hertzConfigRelPath))
		}
		if !force {
			return src, false, nil
		}
		return replaceMarkedHertzConfigBlock(src, wrapHertzConfigSnippet(snippet, infraKind), startMarker, endMarker)
	}
	if hasTopLevelConfigKey(src, hertzConfigSnippetKeys[infraKind]) {
		return src, false, nil
	}
	block := wrapHertzConfigSnippet(snippet, infraKind)
	trimmed := strings.TrimRight(src, "\n")
	if trimmed == "" {
		return block + "\n", true, nil
	}
	return trimmed + "\n\n" + block + "\n", true, nil
}

func wrapHertzConfigSnippet(snippet, infraKind string) string {
	startMarker, endMarker := hertzConfigMarkers(infraKind)
	return startMarker + "\n" + strings.TrimRight(snippet, "\n") + "\n" + endMarker
}

func hertzConfigMarkers(infraKind string) (string, string) {
	return "# ncgo:add-infra:start " + infraKind, "# ncgo:add-infra:end " + infraKind
}

func replaceMarkedHertzConfigBlock(src, block, startMarker, endMarker string) (string, bool, error) {
	start := strings.Index(src, startMarker)
	if start < 0 {
		return src, false, nil
	}
	end := strings.Index(src[start:], endMarker)
	if end < 0 {
		return "", false, fmt.Errorf("infra: malformed config markers: missing %q", endMarker)
	}
	end += start
	lineEnd := end + len(endMarker)
	if lineEnd < len(src) && src[lineEnd] == '\r' {
		lineEnd++
	}
	if lineEnd < len(src) && src[lineEnd] == '\n' {
		lineEnd++
	}
	out := src[:start] + block
	if lineEnd < len(src) {
		out += src[lineEnd:]
	} else {
		out += "\n"
	}
	return out, true, nil
}

func hasTopLevelConfigKey(src, key string) bool {
	needle := key + ":"
	for _, line := range strings.Split(src, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if strings.HasPrefix(line, needle) {
			return true
		}
	}
	return false
}

func frameworkAdapterName(infraKind, serviceKind string) string {
	switch infraKind {
	case KindReleaseCanary:
		return filepath.Join("internal", "base", "release", serviceKind+".go")
	default:
		return filepath.Join("internal", "base", "logging", serviceKind+".go")
	}
}

func normalizeKind(kind string) (string, error) {
	if kind == KindOtelAlias {
		return KindObservabilityOtel, nil
	}
	if kind == KindLoggingAlias {
		return KindObservabilityLog, nil
	}
	if kind == KindCanaryAlias {
		return KindReleaseCanary, nil
	}
	for _, k := range SupportedKinds() {
		if kind == k {
			return kind, nil
		}
	}
	return "", fmt.Errorf("infra: kind %q is invalid; want one of %v", kind, SupportedKinds())
}

func isKitexOnly(kind string) bool {
	for _, k := range kitexOnlyKinds() {
		if kind == k {
			return true
		}
	}
	return false
}

// assetPath maps (service.kind, infra kind) to the embedded asset path. Common
// add-ons may live under optional/ or under both hertz/optional and
// kitex/optional. Kitex-only add-ons are rejected for Hertz before attempting to
// read a non-existent asset file.
func assetPath(serviceKind, infraKind string) (string, error) {
	if commonAssetKinds[infraKind] {
		return "optional/" + infraKind + ".go", nil
	}
	switch serviceKind {
	case manifest.KindHertz:
		if isKitexOnly(infraKind) {
			return "", fmt.Errorf("infra: kind %q is only supported for kitex services", infraKind)
		}
		return serviceKind + "/optional/" + infraKind + ".go", nil
	case manifest.KindKitex:
		return serviceKind + "/optional/" + infraKind + ".go", nil
	default:
		return "", fmt.Errorf("infra: unsupported service kind %q", serviceKind)
	}
}

func outputPath(root, kind string) (string, error) {
	rel, ok := outputRelPaths[kind]
	if !ok {
		return "", fmt.Errorf("infra: kind %q has no output path", kind)
	}
	return filepath.Join(root, rel), nil
}

func plannedFileAction(path string, force bool) (string, error) {
	if _, err := os.Stat(path); err == nil && !force {
		return "", fmt.Errorf("infra: %s already exists; rerun with --force to overwrite", path)
	} else if err == nil {
		return "overwrite", nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("infra: stat %s: %w", path, err)
	}
	return "create", nil
}

func writeFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("infra: mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("infra: write %s: %w", path, err)
	}
	return nil
}

// mergeInfra appends kind to m.Infra if missing and keeps the slice sorted
// for deterministic output. Returns true when the slice changed.
func mergeInfra(m *manifest.Manifest, kind string) bool {
	if manifestHasInfra(m, kind) {
		return false
	}
	m.Infra = append(m.Infra, kind)
	sort.Strings(m.Infra)
	return true
}

func manifestHasInfra(m *manifest.Manifest, kind string) bool {
	for _, k := range m.Infra {
		if k == kind {
			return true
		}
	}
	return false
}

func buildPlan(filePlans []PlanItem, manifestUpdated bool, wire bool, wiredPaths []string, wirePlans []PlanItem, next []string) []PlanItem {
	plan := append([]PlanItem(nil), filePlans...)
	manifestAction := "already_present"
	if manifestUpdated {
		manifestAction = "add"
	}
	plan = append(plan, PlanItem{Kind: "manifest", Action: manifestAction, Path: filepath.Join(".ncgo", "manifest.yaml")})
	if wire {
		if len(wiredPaths) == 0 {
			plan = append(plan, PlanItem{Kind: "wire", Action: "already_wired"})
		}
		for _, path := range wiredPaths {
			plan = append(plan, PlanItem{Kind: "wire", Action: "update", Path: path})
		}
		plan = append(plan, wirePlans...)
	}
	for _, step := range next {
		plan = append(plan, PlanItem{Kind: "next_step", Action: "run", Detail: step})
	}
	return plan
}

func nextSteps(kind, serviceKind, serviceName string) []string {
	if steps, ok := setupSteps[kind]; ok {
		out := append([]string(nil), steps...)
		if serviceName != "" {
			for i, step := range out {
				if step == "OTEL_SERVICE_NAME=<service> OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 ./<your-binary>" {
					out[i] = "OTEL_SERVICE_NAME=" + serviceName + " OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 ./<your-binary>"
				}
			}
		}
		return out
	}
	steps := make([]string, 0, len(goGetDeps[kind])+1)
	for _, dep := range goGetDeps[kind] {
		steps = append(steps, "go get "+dep)
	}
	if serviceKind == manifest.KindHertz {
		if key, ok := hertzConfigSnippetKeys[kind]; ok {
			steps = append(steps, "review "+filepath.FromSlash(hertzConfigRelPath)+" and complete the `"+key+"` section for local config or your config-center payload")
		}
	}
	steps = append(steps, "go mod tidy")
	return steps
}

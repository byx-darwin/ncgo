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
	"github.com/byx-darwin/ncgo/internal/scaffold/shared"
	"gopkg.in/yaml.v3"
)

// Kind values supported by `ncgo add infra`. The corresponding template files
// live in internal/assets/_data/<framework>/optional/ and ship with the binary.
const (
	KindRedis            = "redis"
	KindKafka            = "kafka"
	KindES               = "es"
	KindClickHouse       = "clickhouse"
	KindRegistryPolaris  = "registry_polaris"
	KindObservabilityLog = "observability_logging"
	KindLoggingAlias     = "logging"
	KindReleaseCanary    = "release_canary"
	KindCanaryAlias      = "canary"
	KindRateLimit        = "rate_limit"
	KindRateLimitAlias   = "rate-limit"
)

// SupportedKinds returns all add-on names in canonical order. Some kinds are
// service-kind specific; Add validates that after loading the manifest.
func SupportedKinds() []string {
	return []string{KindRedis, KindKafka, KindES, KindClickHouse, KindObservabilityLog, KindLoggingAlias, KindReleaseCanary, KindCanaryAlias, KindRegistryPolaris, KindRateLimit, KindRateLimitAlias}
}

func commonKinds() []string {
	return []string{KindRedis, KindKafka, KindES, KindClickHouse, KindObservabilityLog, KindReleaseCanary}
}

func kitexOnlyKinds() []string {
	return []string{KindRegistryPolaris, KindRateLimit}
}

// goGetDeps is the source of truth for `go get` dependency next-steps. Keeping
// it here rather than parsing the file header avoids relying on free-form
// comments.
var goGetDeps = map[string][]string{
	KindRedis:           {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"},
	KindKafka:           {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"},
	KindES:              {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"},
	KindClickHouse:      {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"},
	KindRegistryPolaris: {"github.com/kitex-contrib/polaris", "github.com/byx-darwin/go-tools/go-common"},
	KindObservabilityLog: {
		"github.com/byx-darwin/go-tools/go-common",
	},
}

var setupSteps = map[string][]string{
	KindRateLimit: {
		"review conf/dev/conf.yaml rate_limit block (source.type: config|database|rule_center)",
		"observe shadow logs (grep 'ratelimit shadow denied'), then set mode: enforce",
		"optional: set static.max_qps / static.max_connections for a global safety net",
		"go mod tidy",
	},
}

var commonAssetKinds = map[string]bool{
	KindObservabilityLog: true,
	KindReleaseCanary:    true,
}

var outputRelPaths = map[string]string{
	KindRedis:            filepath.Join("internal", "base", "data", "redis.go"),
	KindKafka:            filepath.Join("internal", "base", "data", "kafka.go"),
	KindES:               filepath.Join("internal", "base", "data", "es.go"),
	KindClickHouse:       filepath.Join("internal", "base", "data", "clickhouse.go"),
	KindRegistryPolaris:  filepath.Join("internal", "base", "registry", "polaris.go"),
	KindObservabilityLog: filepath.Join("internal", "base", "logging", "logging.go"),
	KindReleaseCanary:    filepath.Join("internal", "base", "release", "canary.go"),
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
		var body []byte
		if file.PreRenderedBody != nil {
			body = renderAssetBody(file.PreRenderedBody, m.Module)
		} else {
			var err error
			body, err = fs.ReadFile(assets.FS(), file.SourcePath)
			if err != nil {
				return nil, fmt.Errorf("infra: read embedded %s: %w", file.SourcePath, err)
			}
			body = renderAssetBody(body, m.Module)
		}
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
	rateLimitConfWrite, err := planKitexRateLimitConfigWrite(root, m.Service.Kind, kind, opts.Force)
	if err != nil {
		return nil, err
	}
	if rateLimitConfWrite != nil {
		writes = append(writes, *rateLimitConfWrite)
		paths = append(paths, rateLimitConfWrite.Path)
		filePlans = append(filePlans, PlanItem{Kind: "file", Action: rateLimitConfWrite.Action, Path: rateLimitConfWrite.Path})
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
	if !opts.DryRun {
		if err := shared.WriteServiceDockerConfig(root, m); err != nil {
			return nil, err
		}
		if err := shared.WriteMonoCompose(root, m); err != nil {
			return nil, err
		}
		if err := shared.RefreshWorkspaceComposeForServiceRoot(root); err != nil {
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
	// PreRenderedBody, when non-nil, is written verbatim (after the caller has
	// performed its own placeholder rendering). Used for shared fragments whose
	// source yaml wraps the body under a `body:` field.
	PreRenderedBody []byte
}

type plannedWrite struct {
	Path   string
	Body   []byte
	Action string
}

func assetFiles(serviceKind, infraKind string) ([]addOnFile, error) {
	if infraKind == KindRateLimit {
		if serviceKind != manifest.KindKitex {
			return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", infraKind)
		}
		return rateLimitAssetFiles()
	}
	if infraKind == KindRegistryPolaris {
		if serviceKind != manifest.KindKitex {
			return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", infraKind)
		}
		return []addOnFile{
			{SourcePath: "kitex/optional/registry_polaris.go", OutputRelPath: outputRelPaths[KindRegistryPolaris]},
			{SourcePath: "kitex/optional/registry_polaris.yaml", OutputRelPath: "polaris.yaml"},
		}, nil
	}
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
	rendered = strings.ReplaceAll(rendered, "{{.Module}}", module)
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
	if kind == KindLoggingAlias {
		return KindObservabilityLog, nil
	}
	if kind == KindCanaryAlias {
		return KindReleaseCanary, nil
	}
	if kind == KindRateLimitAlias {
		return KindRateLimit, nil
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

// rateLimitAssetFiles returns the add-on files for the rate_limit kind (kitex
// only). Shared fragments under ratelimit/*.yaml are pre-rendered via
// shared.ReadSharedFragmentBody; the kitex middleware template is parsed for
// its `body:` and written to the `path:` declared inside the yaml.
func rateLimitAssetFiles() ([]addOnFile, error) {
	srcFS := assets.FS()
	sharedFragments := []struct {
		name   string // asset name under ratelimit/ (no .yaml suffix)
		target string // relative output path
	}{
		{"ratelimit/resolver", filepath.Join("internal", "pkg", "ratelimit", "resolver.go")},
		{"ratelimit/resolver_test", filepath.Join("internal", "pkg", "ratelimit", "resolver_test.go")},
		{"ratelimit/store", filepath.Join("internal", "pkg", "ratelimit", "store.go")},
		{"ratelimit/store_test", filepath.Join("internal", "pkg", "ratelimit", "store_test.go")},
	}
	files := make([]addOnFile, 0, len(sharedFragments)+1)
	// Module placeholder is resolved below with an empty string; the real
	// module is substituted when Add() renders bodies. Because shared
	// fragments require the module at read time, we delay rendering: pass
	// a zero-module ReadSharedFragmentBody here, and let Add()'s
	// renderAssetBody handle the final substitution via PreRenderedBody's
	// re-rendering path. Instead, pre-render with a sentinel that we
	// replace below.
	//
	// Simplification: we read the yaml body here with module=""; Add()'s
	// PreRenderedBody path writes the bytes verbatim, so we must render
	// {{.Module}} at this point. Because we don't know the module here,
	// we read the raw body and defer rendering by returning the fragment
	// source path; the caller will re-render via renderAssetBody. The
	// cleanest approach is to read the yaml, extract its body, and store
	// it as PreRenderedBody with {{.Module}} placeholder intact; then
	// Add() substitutes the module in renderAssetBody on the
	// PreRenderedBody too. We achieve that by skipping PreRenderedBody
	// and using SourcePath + a custom body extractor at read time — but
	// that requires more plumbing. Instead, read the body here with
	// module="{{.Module}}" literal placeholder preserved (i.e. read raw
	// and do not substitute).
	for _, frag := range sharedFragments {
		body, err := readSharedFragmentBodyRaw(srcFS, frag.name)
		if err != nil {
			return nil, err
		}
		files = append(files, addOnFile{
			OutputRelPath:   frag.target,
			PreRenderedBody: body,
		})
	}
	// Middleware template: parse yaml for body, keep {{.Module}} intact for
	// Add() to render.
	mwBody, mwPath, err := readKitexRateLimitMiddlewareTemplate(srcFS)
	if err != nil {
		return nil, err
	}
	files = append(files, addOnFile{
		OutputRelPath:   filepath.FromSlash(mwPath),
		PreRenderedBody: mwBody,
	})
	return files, nil
}

// readKitexRateLimitMiddlewareTemplate parses the kitex ratelimit middleware
// yaml template and returns the body (with {{.Module}} placeholder preserved)
// and the declared target path.
func readKitexRateLimitMiddlewareTemplate(srcFS fs.FS) ([]byte, string, error) {
	b, err := fs.ReadFile(srcFS, "kitex/kitex-template/ratelimit_middleware.yaml")
	if err != nil {
		return nil, "", fmt.Errorf("infra: read ratelimit middleware template: %w", err)
	}
	var doc struct {
		Path string `yaml:"path"`
		Body string `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, "", fmt.Errorf("infra: parse ratelimit middleware template: %w", err)
	}
	if doc.Path == "" {
		return nil, "", fmt.Errorf("infra: ratelimit middleware template missing path")
	}
	return []byte(doc.Body), doc.Path, nil
}

// planKitexRateLimitConfigRead returns the path of conf/dev/conf.yaml for the
// given root.
func kitexRateLimitConfPath(root string) string {
	return filepath.Join(root, "conf", "dev", "conf.yaml")
}

// planKitexRateLimitConfigWrite returns a planned write for the rate_limit
// config block when serviceKind is kitex and infraKind is rate_limit.
func planKitexRateLimitConfigWrite(root, serviceKind, infraKind string, force bool) (*plannedWrite, error) {
	if serviceKind != manifest.KindKitex || infraKind != KindRateLimit {
		return nil, nil
	}
	return planKitexRateLimitConfig(root, force)
}

// readSharedFragmentBodyRaw reads a shared fragment yaml (canonical kitex
// format: path/update_behavior/body) and returns just the body field with
// placeholders like {{.Module}} preserved for later rendering by
// renderAssetBody.
func readSharedFragmentBodyRaw(srcFS fs.FS, name string) ([]byte, error) {
	b, err := fs.ReadFile(srcFS, name+".yaml")
	if err != nil {
		return nil, fmt.Errorf("infra: read shared fragment %s: %w", name, err)
	}
	var frag struct {
		Body string `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &frag); err != nil {
		return nil, fmt.Errorf("infra: parse shared fragment %s: %w", name, err)
	}
	return []byte(frag.Body), nil
}

// planKitexRateLimitConfig merges the rate_limit block into conf/dev/conf.yaml
// for kitex services. If no rate_limit: top-level key exists, a default block
// is appended. If one exists, enabled is set to true and mode is set to shadow
// within the block's scope only.
func planKitexRateLimitConfig(root string, force bool) (*plannedWrite, error) {
	path := kitexRateLimitConfPath(root)
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			body := defaultRateLimitConfBlock()
			return &plannedWrite{Path: path, Body: []byte(body), Action: "create"}, nil
		}
		return nil, fmt.Errorf("infra: read %s: %w", path, err)
	}
	merged, changed := mergeKitexRateLimitConfig(string(current))
	if !changed {
		return nil, nil
	}
	return &plannedWrite{Path: path, Body: []byte(merged), Action: "update"}, nil
}

func defaultRateLimitConfBlock() string {
	return `rate_limit:
  enabled: true
  mode: shadow
  backend: memory
  fail_open: true
  source:
    type: config
    cache_ttl_seconds: 60s
    fallback_on_error: true
  static:
    max_qps: 0
    max_connections: 0
`
}

// mergeKitexRateLimitConfig updates an existing conf.yaml. If rate_limit: is
// missing, appends the default block. If present, sets enabled: true and
// mode: shadow within the rate_limit scope only. Returns (merged, changed).
//
// Only TOP-LEVEL keys within the rate_limit block are flipped — nested keys
// (e.g. pre_auth.enabled) are left untouched. Top-level keys are identified
// by having the same indent as the first direct child of rate_limit:.
func mergeKitexRateLimitConfig(src string) (string, bool) {
	if !hasTopLevelConfigKey(src, "rate_limit") {
		trimmed := strings.TrimRight(src, "\n")
		if trimmed == "" {
			return defaultRateLimitConfBlock(), true
		}
		return trimmed + "\n\n" + defaultRateLimitConfBlock(), true
	}
	// Scoped replacement within rate_limit block. Find the rate_limit:
	// line and the next top-level key (or EOF). Track the indent of the
	// first direct child so we only flip top-level keys.
	lines := strings.Split(src, "\n")
	startIdx := -1
	endIdx := len(lines)
	childIndent := -1
	for i, line := range lines {
		if startIdx == -1 {
			if strings.HasPrefix(line, "rate_limit:") {
				startIdx = i
			}
			continue
		}
		// End when we hit another top-level key (non-empty, non-comment,
		// not indented).
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			endIdx = i
			break
		}
		if childIndent == -1 {
			childIndent = len(line) - len(strings.TrimLeft(line, " \t"))
		}
	}
	if startIdx == -1 {
		return src, false
	}
	changed := false
	for i := startIdx + 1; i < endIdx; i++ {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		// Only flip direct children of rate_limit: (at childIndent).
		// Deeper lines belong to nested sub-blocks and must not be touched.
		if childIndent >= 0 && indent != childIndent {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "enabled:") {
			if !strings.Contains(trimmed, "true") {
				indentStr := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indentStr + "enabled: true"
				changed = true
			}
		} else if strings.HasPrefix(trimmed, "mode:") {
			if !strings.Contains(trimmed, "shadow") {
				indentStr := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indentStr + "mode: shadow"
				changed = true
			}
		}
	}
	if !changed {
		return src, false
	}
	return strings.Join(lines, "\n"), true
}

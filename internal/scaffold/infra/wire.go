package infra

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

// Wire connects generated optional infra helpers into generated service startup
// files. It is intentionally opt-in and conservative: only known ncgo template
// snippets are patched, and unsupported add-ons return an error.
func Wire(root, module, serviceKind, kind string) ([]string, error) {
	res, err := wire(root, module, serviceKind, kind, false)
	if err != nil {
		return nil, err
	}
	return res.paths, nil
}

// PreviewWire reports which generated startup files would change for --wire,
// without writing the formatted result back to disk.
func PreviewWire(root, module, serviceKind, kind string) ([]string, error) {
	res, err := wire(root, module, serviceKind, kind, true)
	if err != nil {
		return nil, err
	}
	return res.paths, nil
}

// PreviewWirePlan reports path-level and operation-level wiring changes without
// writing the formatted result back to disk.
func PreviewWirePlan(root, module, serviceKind, kind string) ([]string, []PlanItem, error) {
	res, err := wire(root, module, serviceKind, kind, true)
	if err != nil {
		return nil, nil, err
	}
	return res.paths, res.plan, nil
}

type wireResult struct {
	paths []string
	plan  []PlanItem
}

func wire(root, module, serviceKind, kind string, dryRun bool) (*wireResult, error) {
	if !wireSupportedKind(kind) {
		return nil, unsupportedWireError()
	}
	switch serviceKind {
	case manifestKindHertz:
		return wireHertz(root, module, kind, dryRun)
	case manifestKindKitex:
		return wireKitex(root, module, kind, dryRun)
	default:
		return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
	}
}

func wireSupportedKind(kind string) bool {
	p, ok := pluginByKind(kind)
	if !ok {
		return false
	}
	if _, ok := p.(hertzServerWirer); ok {
		return true
	}
	if _, ok := p.(kitexServerWirer); ok {
		return true
	}
	if _, ok := p.(kitexClientWirer); ok {
		return true
	}
	return false
}

func unsupportedWireError() error {
	return fmt.Errorf("infra: --wire is only supported for %s/%s/%s/%s", KindObservabilityLog, KindReleaseCanary, KindRegistryPolaris, KindRateLimit)
}

const (
	manifestKindHertz = "hertz"
	manifestKindKitex = "kitex"

	anchorSourceMarker = "marker"
	anchorSourceLegacy = "legacy"

	markerLoggingInit               = "// ncgo:wire:logging:init"
	markerLoggingServerMiddleware   = "// ncgo:wire:logging:server-middleware"
	markerCanaryServerTraffic       = "// ncgo:wire:canary:server-traffic"
	markerKitexClientMiddleware     = "// ncgo:wire:kitex-client:middleware"
	markerRegistryServer            = "// ncgo:wire:registry:server"
	markerRegistryClient            = "// ncgo:wire:registry:client"
	markerRateLimitServerMiddleware = "// ncgo:wire:ratelimit:server-middleware"
	markerRateLimitStaticLimit      = "// ncgo:wire:ratelimit:static-limit"
)

func wireHertz(root, module, kind string, dryRun bool) (*wireResult, error) {
	path := filepath.Join(root, "internal", "base", "server", "server.go")
	body, err := readSource(path)
	if err != nil {
		return nil, err
	}
	s := string(body)
	plan := []PlanItem(nil)
	if p, ok := pluginByKind(kind); ok {
		if w, ok := p.(hertzServerWirer); ok {
			s, err = w.WireHertzServer(s, module, &plan)
			if err != nil {
				return nil, err
			}
			for i := range plan {
				plan[i].Path = path
			}
		}
	}
	written, err := writeFormatted(path, []byte(s), dryRun)
	if err != nil {
		return nil, err
	}
	return wireResultFor(written, plan), nil
}

func wireKitex(root, module, kind string, dryRun bool) (*wireResult, error) {
	paths := []string{}
	plan := []PlanItem(nil)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	body, err := readSource(serverPath)
	if err != nil {
		return nil, err
	}
	s := string(body)
	serverPlan := []PlanItem(nil)
	if p, ok := pluginByKind(kind); ok {
		if w, ok := p.(kitexServerWirer); ok {
			s, err = w.WireKitexServer(s, module, &serverPlan)
			if err != nil {
				return nil, err
			}
			for i := range serverPlan {
				serverPlan[i].Path = serverPath
			}
		}
	}
	written, err := writeFormatted(serverPath, []byte(s), dryRun)
	if err != nil {
		return nil, err
	}
	paths = append(paths, written...)
	if len(written) > 0 {
		plan = append(plan, serverPlan...)
	}
	clientPaths, err := filepath.Glob(filepath.Join(root, "pkg", "client", "*", "client.go"))
	if err != nil {
		return nil, err
	}
	for _, p := range clientPaths {
		res, err := wireKitexClient(p, module, kind, dryRun)
		if err != nil {
			return nil, err
		}
		paths = append(paths, res.paths...)
		plan = append(plan, res.plan...)
	}
	return &wireResult{paths: paths, plan: plan}, nil
}

func wireKitexClient(path, module, kind string, dryRun bool) (*wireResult, error) {
	body, err := readSource(path)
	if err != nil {
		return nil, err
	}
	s := string(body)
	plan := []PlanItem(nil)
	if p, ok := pluginByKind(kind); ok {
		if w, ok := p.(kitexClientWirer); ok {
			s, err = w.WireKitexClient(s, module, &plan)
			if err != nil {
				return nil, err
			}
			for i := range plan {
				plan[i].Path = path
			}
		}
	}
	written, err := writeFormatted(path, []byte(s), dryRun)
	if err != nil {
		return nil, err
	}
	return wireResultFor(written, plan), nil
}

func hertzLoggingInit() string {
	return "\tlogCfg := logging.Config{\n" +
		"\t\tEnabled:   cfg.Logging.Enabled,\n" +
		"\t\tMode:      cfg.Logging.Mode,\n" +
		"\t\tFormat:    cfg.Logging.Format,\n" +
		"\t\tLevel:     cfg.Logging.Level,\n" +
		"\t\tAddSource: cfg.Logging.AddSource,\n" +
		"\t\tConsole: logging.ConsoleConfig{\n" +
		"\t\t\tEnabled: cfg.Logging.Console.Enabled,\n" +
		"\t\t},\n" +
		"\t\tFile: logging.FileConfig{\n" +
		"\t\t\tEnabled:    cfg.Logging.File.Enabled,\n" +
		"\t\t\tDir:        cfg.Logging.File.Dir,\n" +
		"\t\t\tFilename:   cfg.Logging.File.Filename,\n" +
		"\t\t\tMaxSizeMB:  cfg.Logging.File.MaxSizeMB,\n" +
		"\t\t\tMaxBackups: cfg.Logging.File.MaxBackups,\n" +
		"\t\t\tMaxAgeDays: cfg.Logging.File.MaxAgeDays,\n" +
		"\t\t\tCompress:   cfg.Logging.File.Compress,\n" +
		"\t\t},\n" +
		"\t\tCategories: map[string]logging.CategoryConfig{},\n" +
		"\t}\n" +
		"\tfor category, cc := range cfg.Logging.Categories {\n" +
		"\t\tlogCfg.Categories[category] = logging.CategoryConfig{\n" +
		"\t\t\tEnabled: cc.Enabled,\n" +
		"\t\t\tFile:    cc.File,\n" +
		"\t\t\tLevel:   cc.Level,\n" +
		"\t\t}\n" +
		"\t}\n" +
		"\tif _, err := logging.Init(logCfg, logging.ReleaseInfo{\n" +
		"\t\tServiceName: cfg.Server.Name,\n" +
		"\t\tServiceKind: \"hertz\",\n" +
		"\t\tVersion:     cfg.Server.Version,\n" +
		"\t\tTrack:       cfg.Release.Info.Track,\n" +
		"\t\tGitSHA:      cfg.Release.Info.GitSHA,\n" +
		"\t\tBuildTime:   cfg.Release.Info.BuildTime,\n" +
		"\t}); err != nil {\n" +
		"\t\tlog.Fatalf(\"init logging: %v\", err)\n" +
		"\t}\n"
}

func hertzCanaryTraffic() string {
	return "\tif cfg.Release.Enabled {\n" +
		"\t\th.Use(release.HertzTraffic())\n" +
		"\t}\n"
}

func kitexLoggingInit() string {
	return "\tif _, err := logging.Init(logging.DefaultConfig(), logging.ReleaseInfo{\n" +
		"\t\tServiceName: cfg.Server.Name,\n" +
		"\t\tServiceKind: \"kitex\",\n" +
		"\t}); err != nil {\n" +
		"\t\tlog.Fatalf(\"init logging: %v\", err)\n" +
		"\t}\n"
}

func kitexRegistryServer() string {
	return "\tif reg, regErr := registry.NewRegistry(registry.PolarisConfig{ServiceName: cfg.Server.Registry.Name, ConfigFile: \"polaris.yaml\"}); regErr != nil {\n" +
		"\t\tlog.Fatalf(\"polaris registry: %v\", regErr)\n" +
		"\t} else {\n" +
		"\t\topts = append(opts, kitexserver.WithRegistry(reg))\n" +
		"\t}\n"
}

func kitexRegistryClient() string {
	return "\tif res, resErr := registry.NewResolver(registry.PolarisConfig{ServiceName: cfg.ServiceName, ConfigFile: \"polaris.yaml\"}); resErr == nil {\n" +
		"\t\toptions = append(options, kitexclient.WithResolver(res))\n" +
		"\t}\n"
}

func readSource(path string) ([]byte, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("infra: --wire read %s: %w", path, err)
	}
	return body, nil
}

func writeFormatted(path string, body []byte, dryRun bool) ([]string, error) {
	formatted, err := format.Source(body)
	if err != nil {
		return nil, fmt.Errorf("infra: --wire format %s: %w", path, err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("infra: --wire read %s: %w", path, err)
	}
	if string(current) == string(formatted) {
		return nil, nil
	}
	if dryRun {
		return []string{path}, nil
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return nil, fmt.Errorf("infra: --wire write %s: %w", path, err)
	}
	return []string{path}, nil
}

func wireResultFor(paths []string, plan []PlanItem) *wireResult {
	if len(paths) == 0 {
		return &wireResult{}
	}
	return &wireResult{paths: paths, plan: plan}
}

func wirePlan(path, action, detail string) PlanItem {
	return PlanItem{Kind: "wire", Action: action, Path: path, Detail: detail}
}

func wirePlanWithAnchor(path, action, detail, anchorSource, anchor string) PlanItem {
	item := wirePlan(path, action, detail)
	item.AnchorSource = anchorSource
	item.Anchor = anchor
	return item
}

func addGoImportWithPlan(src, importPath, path string, plan *[]PlanItem) (string, error) {
	out, changed, err := addGoImportTracked(src, importPath)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlan(path, "add_import", importPath))
	}
	return out, nil
}

func insertOnceMarkerOrAnchorWithPlan(src, exists, marker, anchor, addition, path string, plan *[]PlanItem, action, detail string) (string, error) {
	out, changed, anchorSource, anchorValue, err := insertOnceMarkerOrAnchorTracked(src, exists, marker, anchor, addition)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlanWithAnchor(path, action, detail, anchorSource, anchorValue))
	}
	return out, nil
}

func replaceOnceStrictWithPlan(src, exists, old, new, path string, plan *[]PlanItem, detail string) (string, error) {
	out, changed, err := replaceOnceStrictTracked(src, exists, old, new)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlanWithAnchor(path, "replace_middleware", detail, anchorSourceLegacy, old))
	}
	return out, nil
}

func insertAfterMarkerOrAnyWithPlan(src, exists, marker string, anchors []string, addition, path string, plan *[]PlanItem, action, detail string) (string, error) {
	out, changed, anchorSource, anchorValue, err := insertAfterMarkerOrAnyTracked(src, exists, marker, anchors, addition)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlanWithAnchor(path, action, detail, anchorSource, anchorValue))
	}
	return out, nil
}

func addGoImportTracked(src, importPath string) (string, bool, error) {
	quoted := "\"" + importPath + "\""
	if importPresentInCode(src, quoted) {
		return src, false, nil
	}
	idx := strings.Index(src, "import (\n")
	if idx < 0 {
		return "", false, fmt.Errorf("infra: --wire could not find import block for %s", importPath)
	}
	insertAt := idx + len("import (\n")
	return src[:insertAt] + "\t" + quoted + "\n" + src[insertAt:], true, nil
}

// importPresentInCode reports whether quoted (e.g. "\"github.com/x/foo\"")
// appears on a non-comment line in src. A line whose trimmed form starts with
// "//" is a comment and does NOT count — this prevents template hint comments
// like `// import "github.com/x/foo"` from short-circuiting real import
// insertion. Comment lines never legitimately contain an import declaration,
// so this prefilter is safe for all callers.
func importPresentInCode(src, quoted string) bool {
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, quoted) {
			return true
		}
	}
	return false
}

func insertOnceStrictTracked(src, exists, anchor, addition string) (string, bool, error) {
	if strings.Contains(src, exists) {
		return src, false, nil
	}
	if !strings.Contains(src, anchor) {
		return "", false, fmt.Errorf("infra: --wire could not find insertion anchor for %s", strings.TrimSpace(addition))
	}
	return strings.Replace(src, anchor, anchor+addition, 1), true, nil
}

func insertOnceMarkerOrAnchorTracked(src, exists, marker, anchor, addition string) (string, bool, string, string, error) {
	if strings.Contains(src, exists) {
		return src, false, "", "", nil
	}
	if markerLine, ok := lineContaining(src, marker); ok {
		return strings.Replace(src, markerLine, markerLine+addition, 1), true, anchorSourceMarker, marker, nil
	}
	out, changed, err := insertOnceStrictTracked(src, exists, anchor, addition)
	if err != nil {
		return "", false, "", "", err
	}
	return out, changed, anchorSourceLegacy, anchor, nil
}

func replaceOnceStrictTracked(src, exists, old, new string) (string, bool, error) {
	if strings.Contains(src, exists) {
		return src, false, nil
	}
	if !strings.Contains(src, old) {
		return "", false, fmt.Errorf("infra: --wire could not find replacement anchor %s", strings.TrimSpace(old))
	}
	return strings.Replace(src, old, new, 1), true, nil
}

func insertAfterMarkerOrAnyTracked(src, exists, marker string, anchors []string, addition string) (string, bool, string, string, error) {
	if strings.Contains(src, exists) {
		return src, false, "", "", nil
	}
	if markerLine, ok := lineContaining(src, marker); ok {
		return strings.Replace(src, markerLine, markerLine+addition, 1), true, anchorSourceMarker, marker, nil
	}
	for _, anchor := range anchors {
		if strings.Contains(src, anchor) {
			return strings.Replace(src, anchor, anchor+addition, 1), true, anchorSourceLegacy, anchor, nil
		}
	}
	return "", false, "", "", fmt.Errorf("infra: --wire could not find middleware anchor for %s", strings.TrimSpace(addition))
}

func lineContaining(src, marker string) (string, bool) {
	idx := strings.Index(src, marker)
	if idx < 0 {
		return "", false
	}
	start := strings.LastIndex(src[:idx], "\n") + 1
	endRel := strings.Index(src[idx:], "\n")
	if endRel < 0 {
		return src[start:], true
	}
	return src[start : idx+endRel+1], true
}

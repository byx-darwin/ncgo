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
	return kind == KindObservabilityLog || kind == KindReleaseCanary
}

func unsupportedWireError() error {
	return fmt.Errorf("infra: --wire is only supported for %s/%s", KindObservabilityLog, KindReleaseCanary)
}

const (
	manifestKindHertz = "hertz"
	manifestKindKitex = "kitex"

	markerLoggingInit             = "// ncgo:wire:logging:init"
	markerLoggingServerMiddleware = "// ncgo:wire:logging:server-middleware"
	markerCanaryServerTraffic     = "// ncgo:wire:canary:server-traffic"
	markerKitexClientMiddleware   = "// ncgo:wire:kitex-client:middleware"
)

func wireHertz(root, module, kind string, dryRun bool) (*wireResult, error) {
	path := filepath.Join(root, "internal", "base", "server", "server.go")
	body, err := readSource(path)
	if err != nil {
		return nil, err
	}
	s := string(body)
	plan := []PlanItem(nil)
	switch kind {
	case KindObservabilityLog:
		s, err = addGoImportWithPlan(s, module+"/internal/base/logging", path, &plan)
		if err != nil {
			return nil, err
		}
		s, err = insertOnceMarkerOrAnchorWithPlan(s, "logging.Init(", markerLoggingInit, "\tdo.ProvideValue(injector, cfg)\n", hertzLoggingInit(), path, &plan, "insert_logging_init", "logging.Init")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrictWithPlan(s, "h.Use(logging.HertzRecovery())", "h.Use(middleware.Recovery())", "h.Use(logging.HertzRecovery())", path, &plan, "hertz recovery")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrictWithPlan(s, "h.Use(logging.HertzRequestID())", "h.Use(middleware.RequestID())", "h.Use(logging.HertzRequestID())", path, &plan, "hertz request id")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrictWithPlan(s, "h.Use(logging.HertzAccessLog())", "h.Use(middleware.AccessLog())", "h.Use(logging.HertzAccessLog())", path, &plan, "hertz access log")
		if err != nil {
			return nil, err
		}
	case KindReleaseCanary:
		s, err = addGoImportWithPlan(s, module+"/internal/base/release", path, &plan)
		if err != nil {
			return nil, err
		}
		s, err = insertAfterMarkerOrAnyWithPlan(s, "release.HertzTraffic()", markerCanaryServerTraffic, []string{
			"\th.Use(logging.HertzRequestID())\n",
			"\th.Use(middleware.RequestID())\n",
		}, "\th.Use(release.HertzTraffic())\n", path, &plan, "insert_traffic_middleware", "release.HertzTraffic")
		if err != nil {
			return nil, err
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
	switch kind {
	case KindObservabilityLog:
		s, err = addGoImportWithPlan(s, module+"/internal/base/logging", serverPath, &serverPlan)
		if err != nil {
			return nil, err
		}
		s, err = insertOnceMarkerOrAnchorWithPlan(s, "logging.Init(", markerLoggingInit, "\tif cfg == nil {\n\t\tcfg = conf.Default()\n\t}\n", kitexLoggingInit(), serverPath, &serverPlan, "insert_logging_init", "logging.Init")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrictWithPlan(s, "logging.KitexRequestID(),", "interceptor.RequestID(),", "logging.KitexRequestID(),", serverPath, &serverPlan, "kitex request id")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrictWithPlan(s, "logging.KitexAccessLog(),", "interceptor.AccessLog(),", "logging.KitexAccessLog(),", serverPath, &serverPlan, "kitex access log")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrictWithPlan(s, "logging.KitexRecovery(),", "interceptor.Recovery(),", "logging.KitexRecovery(),", serverPath, &serverPlan, "kitex recovery")
		if err != nil {
			return nil, err
		}
	case KindReleaseCanary:
		s, err = addGoImportWithPlan(s, module+"/internal/base/release", serverPath, &serverPlan)
		if err != nil {
			return nil, err
		}
		s, err = insertAfterMarkerOrAnyWithPlan(s, "release.KitexTraffic()", markerCanaryServerTraffic, []string{
			"\t\t\tlogging.KitexRequestID(),\n",
			"\t\t\tinterceptor.RequestID(),\n",
		}, "\t\t\trelease.KitexTraffic(),\n", serverPath, &serverPlan, "insert_traffic_middleware", "release.KitexTraffic")
		if err != nil {
			return nil, err
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
	anchor := "\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"
	switch kind {
	case KindObservabilityLog:
		s, err = addGoImportWithPlan(s, module+"/internal/base/logging", path, &plan)
		if err != nil {
			return nil, err
		}
		loggingBlock := "\toptions = append(options, kitexclient.WithMiddleware(endpoint.Chain(\n\t\tlogging.KitexRequestID(),\n\t\tlogging.KitexAccessLog(),\n\t)))\n"
		s, err = insertOnceMarkerOrAnchorWithPlan(s, "logging.KitexAccessLog()", markerKitexClientMiddleware, anchor, loggingBlock, path, &plan, "insert_client_middleware", "logging.KitexAccessLog")
		if err != nil {
			return nil, err
		}
	case KindReleaseCanary:
		s, err = addGoImportWithPlan(s, module+"/internal/base/release", path, &plan)
		if err != nil {
			return nil, err
		}
		s, err = insertOnceMarkerOrAnchorWithPlan(s, "release.KitexTraffic()", markerKitexClientMiddleware, anchor, "\toptions = append(options, kitexclient.WithMiddleware(release.KitexTraffic()))\n", path, &plan, "insert_client_middleware", "release.KitexTraffic")
		if err != nil {
			return nil, err
		}
	}
	written, err := writeFormatted(path, []byte(s), dryRun)
	if err != nil {
		return nil, err
	}
	return wireResultFor(written, plan), nil
}

func hertzLoggingInit() string {
	return "\tif _, err := logging.Init(logging.DefaultConfig(), logging.ReleaseInfo{\n" +
		"\t\tServiceName: cfg.Server.Name,\n" +
		"\t\tServiceKind: \"hertz\",\n" +
		"\t\tVersion:     cfg.Server.Version,\n" +
		"\t}); err != nil {\n" +
		"\t\tpanic(err)\n" +
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

func insertOnceStrictWithPlan(src, exists, anchor, addition, path string, plan *[]PlanItem, action, detail string) (string, error) {
	out, changed, err := insertOnceStrictTracked(src, exists, anchor, addition)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlan(path, action, detail))
	}
	return out, nil
}

func insertOnceMarkerOrAnchorWithPlan(src, exists, marker, anchor, addition, path string, plan *[]PlanItem, action, detail string) (string, error) {
	out, changed, err := insertOnceMarkerOrAnchorTracked(src, exists, marker, anchor, addition)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlan(path, action, detail))
	}
	return out, nil
}

func replaceOnceStrictWithPlan(src, exists, old, new, path string, plan *[]PlanItem, detail string) (string, error) {
	out, changed, err := replaceOnceStrictTracked(src, exists, old, new)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlan(path, "replace_middleware", detail))
	}
	return out, nil
}

func insertAfterAnyOnceWithPlan(src, exists string, anchors []string, addition, path string, plan *[]PlanItem, action, detail string) (string, error) {
	out, changed, err := insertAfterAnyOnceTracked(src, exists, anchors, addition)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlan(path, action, detail))
	}
	return out, nil
}

func insertAfterMarkerOrAnyWithPlan(src, exists, marker string, anchors []string, addition, path string, plan *[]PlanItem, action, detail string) (string, error) {
	out, changed, err := insertAfterMarkerOrAnyTracked(src, exists, marker, anchors, addition)
	if err != nil {
		return "", err
	}
	if changed {
		*plan = append(*plan, wirePlan(path, action, detail))
	}
	return out, nil
}

func addGoImport(src, importPath string) (string, error) {
	out, _, err := addGoImportTracked(src, importPath)
	return out, err
}

func addGoImportTracked(src, importPath string) (string, bool, error) {
	quoted := "\"" + importPath + "\""
	if strings.Contains(src, quoted) {
		return src, false, nil
	}
	idx := strings.Index(src, "import (\n")
	if idx < 0 {
		return "", false, fmt.Errorf("infra: --wire could not find import block for %s", importPath)
	}
	insertAt := idx + len("import (\n")
	return src[:insertAt] + "\t" + quoted + "\n" + src[insertAt:], true, nil
}

func insertOnceStrict(src, exists, anchor, addition string) (string, error) {
	out, _, err := insertOnceStrictTracked(src, exists, anchor, addition)
	return out, err
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

func insertOnceMarkerOrAnchorTracked(src, exists, marker, anchor, addition string) (string, bool, error) {
	if strings.Contains(src, exists) {
		return src, false, nil
	}
	if markerLine, ok := lineContaining(src, marker); ok {
		return strings.Replace(src, markerLine, markerLine+addition, 1), true, nil
	}
	return insertOnceStrictTracked(src, exists, anchor, addition)
}

func replaceOnceStrict(src, exists, old, new string) (string, error) {
	out, _, err := replaceOnceStrictTracked(src, exists, old, new)
	return out, err
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

func insertAfterAnyOnce(src, exists string, anchors []string, addition string) (string, error) {
	out, _, err := insertAfterAnyOnceTracked(src, exists, anchors, addition)
	return out, err
}

func insertAfterAnyOnceTracked(src, exists string, anchors []string, addition string) (string, bool, error) {
	if strings.Contains(src, exists) {
		return src, false, nil
	}
	for _, anchor := range anchors {
		if strings.Contains(src, anchor) {
			return strings.Replace(src, anchor, anchor+addition, 1), true, nil
		}
	}
	return "", false, fmt.Errorf("infra: --wire could not find middleware anchor for %s", strings.TrimSpace(addition))
}

func insertAfterMarkerOrAnyTracked(src, exists, marker string, anchors []string, addition string) (string, bool, error) {
	if strings.Contains(src, exists) {
		return src, false, nil
	}
	if markerLine, ok := lineContaining(src, marker); ok {
		return strings.Replace(src, markerLine, markerLine+addition, 1), true, nil
	}
	return insertAfterAnyOnceTracked(src, exists, anchors, addition)
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

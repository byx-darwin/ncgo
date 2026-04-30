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
	return wire(root, module, serviceKind, kind, false)
}

// PreviewWire reports which generated startup files would change for --wire,
// without writing the formatted result back to disk.
func PreviewWire(root, module, serviceKind, kind string) ([]string, error) {
	return wire(root, module, serviceKind, kind, true)
}

func wire(root, module, serviceKind, kind string, dryRun bool) ([]string, error) {
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
)

func wireHertz(root, module, kind string, dryRun bool) ([]string, error) {
	path := filepath.Join(root, "internal", "base", "server", "server.go")
	body, err := readSource(path)
	if err != nil {
		return nil, err
	}
	s := string(body)
	switch kind {
	case KindObservabilityLog:
		s, err = addGoImport(s, module+"/internal/base/logging")
		if err != nil {
			return nil, err
		}
		s, err = insertOnceStrict(s, "logging.Init(", "\tdo.ProvideValue(injector, cfg)\n", hertzLoggingInit())
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrict(s, "h.Use(logging.HertzRecovery())", "h.Use(middleware.Recovery())", "h.Use(logging.HertzRecovery())")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrict(s, "h.Use(logging.HertzRequestID())", "h.Use(middleware.RequestID())", "h.Use(logging.HertzRequestID())")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrict(s, "h.Use(logging.HertzAccessLog())", "h.Use(middleware.AccessLog())", "h.Use(logging.HertzAccessLog())")
		if err != nil {
			return nil, err
		}
	case KindReleaseCanary:
		s, err = addGoImport(s, module+"/internal/base/release")
		if err != nil {
			return nil, err
		}
		s, err = insertAfterAnyOnce(s, "release.HertzTraffic()", []string{
			"\th.Use(logging.HertzRequestID())\n",
			"\th.Use(middleware.RequestID())\n",
		}, "\th.Use(release.HertzTraffic())\n")
		if err != nil {
			return nil, err
		}
	}
	return writeFormatted(path, []byte(s), dryRun)
}

func wireKitex(root, module, kind string, dryRun bool) ([]string, error) {
	paths := []string{}
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	body, err := readSource(serverPath)
	if err != nil {
		return nil, err
	}
	s := string(body)
	switch kind {
	case KindObservabilityLog:
		s, err = addGoImport(s, module+"/internal/base/logging")
		if err != nil {
			return nil, err
		}
		s, err = insertOnceStrict(s, "logging.Init(", "\tif cfg == nil {\n\t\tcfg = conf.Default()\n\t}\n", kitexLoggingInit())
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrict(s, "logging.KitexRequestID(),", "interceptor.RequestID(),", "logging.KitexRequestID(),")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrict(s, "logging.KitexAccessLog(),", "interceptor.AccessLog(),", "logging.KitexAccessLog(),")
		if err != nil {
			return nil, err
		}
		s, err = replaceOnceStrict(s, "logging.KitexRecovery(),", "interceptor.Recovery(),", "logging.KitexRecovery(),")
		if err != nil {
			return nil, err
		}
	case KindReleaseCanary:
		s, err = addGoImport(s, module+"/internal/base/release")
		if err != nil {
			return nil, err
		}
		s, err = insertAfterAnyOnce(s, "release.KitexTraffic()", []string{
			"\t\t\tlogging.KitexRequestID(),\n",
			"\t\t\tinterceptor.RequestID(),\n",
		}, "\t\t\trelease.KitexTraffic(),\n")
		if err != nil {
			return nil, err
		}
	}
	written, err := writeFormatted(serverPath, []byte(s), dryRun)
	if err != nil {
		return nil, err
	}
	paths = append(paths, written...)
	clientPaths, err := filepath.Glob(filepath.Join(root, "pkg", "client", "*", "client.go"))
	if err != nil {
		return nil, err
	}
	for _, p := range clientPaths {
		written, err := wireKitexClient(p, module, kind, dryRun)
		if err != nil {
			return nil, err
		}
		paths = append(paths, written...)
	}
	return paths, nil
}

func wireKitexClient(path, module, kind string, dryRun bool) ([]string, error) {
	body, err := readSource(path)
	if err != nil {
		return nil, err
	}
	s := string(body)
	anchor := "\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"
	switch kind {
	case KindObservabilityLog:
		s, err = addGoImport(s, module+"/internal/base/logging")
		if err != nil {
			return nil, err
		}
		loggingBlock := "\toptions = append(options, kitexclient.WithMiddleware(endpoint.Chain(\n\t\tlogging.KitexRequestID(),\n\t\tlogging.KitexAccessLog(),\n\t)))\n"
		s, err = insertOnceStrict(s, "logging.KitexAccessLog()", anchor, loggingBlock)
		if err != nil {
			return nil, err
		}
	case KindReleaseCanary:
		s, err = addGoImport(s, module+"/internal/base/release")
		if err != nil {
			return nil, err
		}
		s, err = insertOnceStrict(s, "release.KitexTraffic()", anchor, "\toptions = append(options, kitexclient.WithMiddleware(release.KitexTraffic()))\n")
		if err != nil {
			return nil, err
		}
	}
	return writeFormatted(path, []byte(s), dryRun)
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

func addGoImport(src, importPath string) (string, error) {
	quoted := "\"" + importPath + "\""
	if strings.Contains(src, quoted) {
		return src, nil
	}
	idx := strings.Index(src, "import (\n")
	if idx < 0 {
		return "", fmt.Errorf("infra: --wire could not find import block for %s", importPath)
	}
	insertAt := idx + len("import (\n")
	return src[:insertAt] + "\t" + quoted + "\n" + src[insertAt:], nil
}

func insertOnceStrict(src, exists, anchor, addition string) (string, error) {
	if strings.Contains(src, exists) {
		return src, nil
	}
	if !strings.Contains(src, anchor) {
		return "", fmt.Errorf("infra: --wire could not find insertion anchor for %s", strings.TrimSpace(addition))
	}
	return strings.Replace(src, anchor, anchor+addition, 1), nil
}

func replaceOnceStrict(src, exists, old, new string) (string, error) {
	if strings.Contains(src, exists) {
		return src, nil
	}
	if !strings.Contains(src, old) {
		return "", fmt.Errorf("infra: --wire could not find replacement anchor %s", strings.TrimSpace(old))
	}
	return strings.Replace(src, old, new, 1), nil
}

func insertAfterAnyOnce(src, exists string, anchors []string, addition string) (string, error) {
	if strings.Contains(src, exists) {
		return src, nil
	}
	for _, anchor := range anchors {
		if strings.Contains(src, anchor) {
			return strings.Replace(src, anchor, anchor+addition, 1), nil
		}
	}
	return "", fmt.Errorf("infra: --wire could not find middleware anchor for %s", strings.TrimSpace(addition))
}

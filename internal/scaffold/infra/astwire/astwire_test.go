package astwire

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

// ---- Helpers ----

func mustParse(t *testing.T, src string) (*ast.File, *token.FileSet) {
	t.Helper()
	f, fset, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return f, fset
}

func mustFormat(t *testing.T, fset *token.FileSet, f *ast.File) string {
	t.Helper()
	out, err := Format(fset, f)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	return string(out)
}

func assertContains(t *testing.T, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Errorf("output missing %q\n--- output ---\n%s", want, output)
	}
}

func assertNotContains(t *testing.T, output, want string) {
	t.Helper()
	if strings.Contains(output, want) {
		t.Errorf("output should not contain %q\n--- output ---\n%s", want, output)
	}
}

func assertContainsAll(t *testing.T, output string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, output)
		}
	}
}

// ---- AddImport Tests ----

func TestAddImport_NewImport(t *testing.T) {
	src := `package server

import (
	"fmt"
)
`
	f, fset := mustParse(t, src)
	ok := AddImport(f, "github.com/x/demo/internal/base/logging")
	if !ok {
		t.Fatal("AddImport should have returned true")
	}
	output := mustFormat(t, fset, f)
	assertContains(t, output, `"github.com/x/demo/internal/base/logging"`)
}

func TestAddImport_ExistingImport(t *testing.T) {
	src := `package server

import (
	"fmt"

	"github.com/x/demo/internal/base/logging"
)
`
	f, fset := mustParse(t, src)
	ok := AddImport(f, "github.com/x/demo/internal/base/logging")
	if ok {
		t.Fatal("AddImport should have returned false for existing import")
	}
	output := mustFormat(t, fset, f)
	// Should only contain the import once
	if strings.Count(output, "github.com/x/demo/internal/base/logging") != 1 {
		t.Error("import should not be duplicated")
	}
}

func TestAddImport_NoImportBlock(t *testing.T) {
	src := `package server

func Run() {}
`
	f, fset := mustParse(t, src)
	ok := AddImport(f, "fmt")
	if !ok {
		t.Fatal("AddImport should have returned true")
	}
	output := mustFormat(t, fset, f)
	assertContains(t, output, `"fmt"`)
	assertContains(t, output, "import (")
}

// ---- InsertStmtsAfterMarker Tests ----

func TestInsertStmtsAfterMarker_Found(t *testing.T) {
	src := `package server

import (
	"fmt"
)

func Run() {
	injector := do.New()
	// ncgo:wire:logging:init
	h := server.Default()
	h.Use(middleware.Recovery())
}
`
	f, fset := mustParse(t, src)
	stmts, err := parseStmts("\tlogCfg := logging.Config{}\n\tlogging.Init(logCfg)\n")
	if err != nil {
		t.Fatalf("parseStmts: %v", err)
	}
	ok := insertStmtsAfterMarker(f, fset, "// ncgo:wire:logging:init", stmts)
	if !ok {
		t.Fatal("insertStmtsAfterMarker should have returned true")
	}
	output := mustFormat(t, fset, f)
	assertContains(t, output, "logCfg := logging.Config{}")
	assertContains(t, output, "logging.Init(logCfg)")
	// Marker should still be present
	assertContains(t, output, "// ncgo:wire:logging:init")
	// Statements should be after marker but before h := server.Default()
	markerIdx := strings.Index(output, "// ncgo:wire:logging:init")
	logCfgIdx := strings.Index(output, "logCfg := logging.Config{}")
	hIdx := strings.Index(output, "h := server.Default()")
	if markerIdx < 0 || logCfgIdx < 0 || hIdx < 0 {
		t.Fatal("could not find expected tokens in output")
	}
	if !(markerIdx < logCfgIdx && logCfgIdx < hIdx) {
		t.Error("inserted statements should be between marker and h := server.Default()")
	}
}

func TestInsertStmtsAfterMarker_NotFound(t *testing.T) {
	src := `package server

func Run() {
	h := server.Default()
	h.Use(middleware.Recovery())
}
`
	f, fset := mustParse(t, src)
	stmts, _ := parseStmts("\tlogCfg := logging.Config{}\n")
	ok := insertStmtsAfterMarker(f, fset, "// ncgo:wire:logging:init", stmts)
	if ok {
		t.Fatal("insertStmtsAfterMarker should have returned false when marker not found")
	}
}

// ---- InsertStmtsAfterAnchors Tests ----

func TestInsertStmtsAfterAnchors_Found(t *testing.T) {
	src := `package server

import (
	"fmt"
)

func Run() {
	injector := do.New()
	cfg := conf.Get()
	do.ProvideValue(injector, cfg)
	h := server.Default()
	h.Use(middleware.AccessLog())
}
`
	f, fset := mustParse(t, src)
	stmts, _ := parseStmts("\tlogCfg := logging.Config{}\n\tlogging.Init(logCfg)\n")
	ok := insertStmtsAfterAnchors(f, fset, []byte(src), []string{"do.ProvideValue(injector, cfg)"}, stmts)
	if !ok {
		t.Fatal("insertStmtsAfterAnchors should have returned true")
	}
	output := mustFormat(t, fset, f)
	assertContains(t, output, "logCfg := logging.Config{}")
}

func TestInsertStmtsAfterAnchors_NotFound(t *testing.T) {
	src := `package server

func Run() {
	h := server.Default()
}
`
	f, fset := mustParse(t, src)
	stmts, _ := parseStmts("\tlogCfg := logging.Config{}\n")
	ok := insertStmtsAfterAnchors(f, fset, []byte(src), []string{"not-exist"}, stmts)
	if ok {
		t.Fatal("insertStmtsAfterAnchors should have returned false when anchor not found")
	}
}

// ---- ReplaceCallExpr Tests ----

func TestReplaceCallExpr_InterceptorRequestID(t *testing.T) {
	src := `package server

func Run() {
	opts := []Option{
		WithMiddleware(Chain(
			interceptor.RequestID(),
			interceptor.AccessLog(),
		)),
	}
}
`
	f, fset := mustParse(t, src)
	replaceCallExpr(f, "interceptor", "RequestID", "logging", "KitexRequestID")
	output := mustFormat(t, fset, f)
	assertContains(t, output, "logging.KitexRequestID()")
	assertNotContains(t, output, "interceptor.RequestID()")
}

func TestReplaceCallExpr_HertzMiddleware(t *testing.T) {
	src := `package server

func Run() {
	h := server.Default()
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	h.Use(middleware.AccessLog())
}
`
	f, fset := mustParse(t, src)
	replaceCallExpr(f, "middleware", "Recovery", "logging", "HertzRecovery")
	output := mustFormat(t, fset, f)
	assertContains(t, output, "logging.HertzRecovery()")
	assertNotContains(t, output, "middleware.Recovery()")
	// Other middleware calls should be unchanged
	assertContains(t, output, "middleware.RequestID()")
	assertContains(t, output, "middleware.AccessLog()")
}

func TestReplaceCallExpr_NoMatch(t *testing.T) {
	src := `package server

func Run() {
	h := server.Default()
	h.Use(custom.Recovery())
}
`
	f, fset := mustParse(t, src)
	replaceCallExpr(f, "middleware", "Recovery", "logging", "HertzRecovery")
	output := mustFormat(t, fset, f)
	assertContains(t, output, "custom.Recovery()")
	assertNotContains(t, output, "logging.HertzRecovery()")
}

// ---- InsertExprAfterMarker Tests ----

func TestInsertExprAfterMarker_Found(t *testing.T) {
	src := `package server

func Run() {
	opts := []Option{
		WithMiddleware(Chain(
			interceptor.RequestID(),
			// ncgo:wire:canary:server-traffic
			interceptor.AccessLog(),
		)),
	}
}
`
	f, fset := mustParse(t, src)
	expr, err := parseExpr("release.KitexTraffic(),")
	if err != nil {
		t.Fatalf("parseExpr: %v", err)
	}
	// Remove trailing comma for insertion into arg list
	if ce, ok := expr.(*ast.CallExpr); ok {
		ok = insertExprAfterMarker(f, fset, "// ncgo:wire:canary:server-traffic", ce)
		if !ok {
			t.Fatal("insertExprAfterMarker should have returned true")
		}
	} else {
		t.Fatalf("expected CallExpr, got %T", expr)
	}
	output := mustFormat(t, fset, f)
	assertContains(t, output, "release.KitexTraffic()")
	// Should be inserted between interceptor.RequestID() and interceptor.AccessLog()
	assertContains(t, output, "interceptor.RequestID()")
	assertContains(t, output, "interceptor.AccessLog()")
	reqIDIdx := strings.Index(output, "interceptor.RequestID()")
	trafficIdx := strings.Index(output, "release.KitexTraffic()")
	accessIdx := strings.Index(output, "interceptor.AccessLog()")
	if reqIDIdx < 0 || trafficIdx < 0 || accessIdx < 0 {
		t.Fatal("could not find expected tokens in output")
	}
	if !(reqIDIdx < trafficIdx && trafficIdx < accessIdx) {
		t.Errorf("expected: RequestID < KitexTraffic < AccessLog\ngot idx: %d < %d < %d", reqIDIdx, trafficIdx, accessIdx)
	}
}

// ---- WireFile Tests (end-to-end) ----

func TestWireFile_HertzLoggingFull(t *testing.T) {
	src := `package server

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/samber/do/v2"

	"github.com/x/demo/internal/base/conf"
	"github.com/x/demo/internal/pkg/middleware"
)

func Run() {
	injector := do.New()
	cfg := conf.Get()
	if cfg == nil {
		cfg = conf.Default()
	}
	do.ProvideValue(injector, cfg)
	h := server.Default()
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	h.Use(middleware.AccessLog())
	h.Use(middleware.RequestTimeout(time.Second))
}
`
	loggingInit := "\tlogCfg := logging.Config{\n" +
		"\t\tEnabled: cfg.Logging.Enabled,\n" +
		"\t}\n" +
		"\tif _, err := logging.Init(logCfg); err != nil {\n" +
		"\t\tpanic(err)\n" +
		"\t}\n"
	result, err := WireFile([]byte(src),
		WireOp{AddImport: "github.com/x/demo/internal/base/logging"},
		WireOp{
			Marker:         "// ncgo:wire:logging:init",
			Anchors:        []string{"\tdo.ProvideValue(injector, cfg)\n"},
			InsertSrc:      loggingInit,
			ExistsSentinel: "logging.Init(",
		},
		WireOp{
			ReplacePkg:     "middleware",
			ReplaceName:    "Recovery",
			NewPkg:         "logging",
			NewName:        "HertzRecovery",
			ExistsSentinel: "logging.HertzRecovery()",
		},
		WireOp{
			ReplacePkg:     "middleware",
			ReplaceName:    "RequestID",
			NewPkg:         "logging",
			NewName:        "HertzRequestID",
			ExistsSentinel: "logging.HertzRequestID()",
		},
		WireOp{
			ReplacePkg:     "middleware",
			ReplaceName:    "AccessLog",
			NewPkg:         "logging",
			NewName:        "HertzAccessLog",
			ExistsSentinel: "logging.HertzAccessLog()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContainsAll(t, output, []string{
		`"github.com/x/demo/internal/base/logging"`,
		"logging.Init(",
		"logging.HertzRecovery()",
		"logging.HertzRequestID()",
		"logging.HertzAccessLog()",
	})
	assertNotContains(t, output, "middleware.Recovery()")
	assertNotContains(t, output, "middleware.RequestID()")
	assertNotContains(t, output, "middleware.AccessLog()")
}

func TestWireFile_HertzLoggingWithMarker(t *testing.T) {
	src := `package server

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/samber/do/v2"

	"github.com/x/demo/internal/base/conf"
	"github.com/x/demo/internal/pkg/middleware"
)

func Run() {
	injector := do.New()
	cfg := conf.Get()
	if cfg == nil {
		cfg = conf.Default()
	}
	do.ProvideValue(injector, cfg)
	// ncgo:wire:logging:init

	h := server.Default()
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	h.Use(middleware.AccessLog())
	h.Use(middleware.RequestTimeout(time.Second))
}
`
	loggingInit := "\tlogCfg := logging.Config{\n" +
		"\t\tEnabled: cfg.Logging.Enabled,\n" +
		"\t}\n" +
		"\tif _, err := logging.Init(logCfg); err != nil {\n" +
		"\t\tpanic(err)\n" +
		"\t}\n"
	result, err := WireFile([]byte(src),
		WireOp{AddImport: "github.com/x/demo/internal/base/logging"},
		WireOp{
			Marker:         "// ncgo:wire:logging:init",
			Anchors:        []string{"\tdo.ProvideValue(injector, cfg)\n"},
			InsertSrc:      loggingInit,
			ExistsSentinel: "logging.Init(",
		},
		WireOp{
			ReplacePkg:     "middleware",
			ReplaceName:    "Recovery",
			NewPkg:         "logging",
			NewName:        "HertzRecovery",
			ExistsSentinel: "logging.HertzRecovery()",
		},
		WireOp{
			ReplacePkg:     "middleware",
			ReplaceName:    "RequestID",
			NewPkg:         "logging",
			NewName:        "HertzRequestID",
			ExistsSentinel: "logging.HertzRequestID()",
		},
		WireOp{
			ReplacePkg:     "middleware",
			ReplaceName:    "AccessLog",
			NewPkg:         "logging",
			NewName:        "HertzAccessLog",
			ExistsSentinel: "logging.HertzAccessLog()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContainsAll(t, output, []string{
		`"github.com/x/demo/internal/base/logging"`,
		"logging.Init(",
		"logging.HertzRecovery()",
		"logging.HertzRequestID()",
		"logging.HertzAccessLog()",
	})
}

func TestWireFile_HertzCanaryWithMarker(t *testing.T) {
	src := `package server

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/x/demo/internal/pkg/middleware"
)

func Run() {
	h := server.Default()
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	// ncgo:wire:canary:server-traffic
	h.Use(middleware.AccessLog())
	h.Use(middleware.RequestTimeout(time.Second))
}
`
	canaryTraffic := "\tif cfg.Release.Enabled {\n" +
		"\t\th.Use(release.HertzTraffic())\n" +
		"\t}\n"
	result, err := WireFile([]byte(src),
		WireOp{AddImport: "github.com/x/demo/internal/base/release"},
		WireOp{
			Marker:         "// ncgo:wire:canary:server-traffic",
			Anchors:        []string{"\th.Use(middleware.RequestID())\n"},
			InsertSrc:      canaryTraffic,
			ExistsSentinel: "release.HertzTraffic()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContainsAll(t, output, []string{
		`"github.com/x/demo/internal/base/release"`,
		"if cfg.Release.Enabled {",
		"h.Use(release.HertzTraffic())",
	})
}

func TestWireFile_HertzCanaryWithAnchorFallback(t *testing.T) {
	// No marker in the source, uses anchor fallback
	src := `package server

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/x/demo/internal/pkg/middleware"
)

func Run() {
	h := server.Default()
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	h.Use(middleware.AccessLog())
	h.Use(middleware.RequestTimeout(time.Second))
}
`
	canaryTraffic := "\tif cfg.Release.Enabled {\n" +
		"\t\th.Use(release.HertzTraffic())\n" +
		"\t}\n"
	result, err := WireFile([]byte(src),
		WireOp{AddImport: "github.com/x/demo/internal/base/release"},
		WireOp{
			Marker:         "// ncgo:wire:canary:server-traffic",
			Anchors:        []string{"\th.Use(middleware.RequestID())\n"},
			InsertSrc:      canaryTraffic,
			ExistsSentinel: "release.HertzTraffic()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContainsAll(t, output, []string{
		`"github.com/x/demo/internal/base/release"`,
		"if cfg.Release.Enabled {",
		"h.Use(release.HertzTraffic())",
	})
}

func TestWireFile_KitexServerLoggingFull(t *testing.T) {
	src := `package server

import (
	"context"
	"log"

	"github.com/cloudwego/kitex/pkg/endpoint"
	kitexserver "github.com/cloudwego/kitex/server"

	"github.com/x/demo/internal/base/conf"
	"github.com/x/demo/internal/pkg/interceptor"
)

func Run() {
	cfg := conf.Get()
	if cfg == nil {
		cfg = conf.Default()
	}
	_ = log.Flags()
	opts := []kitexserver.Option{
		kitexserver.WithMiddleware(endpoint.Chain(
			interceptor.RequestID(),
			interceptor.AccessLog(),
			interceptor.Recovery(),
			interceptor.RequestTimeout(0),
		)),
		kitexserver.WithErrorHandler(func(ctx context.Context, err error) error { return err }),
	}
	_ = opts
}
`
	loggingInit := "\tif _, err := logging.Init(logging.DefaultConfig()); err != nil {\n" +
		"\t\tlog.Fatalf(\"init logging: %v\", err)\n" +
		"\t}\n"
	result, err := WireFile([]byte(src),
		WireOp{AddImport: "github.com/x/demo/internal/base/logging"},
		WireOp{
			Marker:         "// ncgo:wire:logging:init",
			Anchors:        []string{"\tif cfg == nil {\n\t\tcfg = conf.Default()\n\t}\n"},
			InsertSrc:      loggingInit,
			ExistsSentinel: "logging.Init(",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContains(t, output, `"github.com/x/demo/internal/base/logging"`)
	assertContains(t, output, "logging.Init(")
}

func TestWireFile_KitexServerReplaceInterceptors(t *testing.T) {
	src := `package server

func Run() {
	opts := []Option{
		WithMiddleware(Chain(
			interceptor.RequestID(),
			interceptor.AccessLog(),
			interceptor.Recovery(),
		)),
	}
	_ = opts
}
`
	result, err := WireFile([]byte(src),
		WireOp{
			ReplacePkg:     "interceptor",
			ReplaceName:    "RequestID",
			NewPkg:         "logging",
			NewName:        "KitexRequestID",
			ExistsSentinel: "logging.KitexRequestID()",
		},
		WireOp{
			ReplacePkg:     "interceptor",
			ReplaceName:    "AccessLog",
			NewPkg:         "logging",
			NewName:        "KitexAccessLog",
			ExistsSentinel: "logging.KitexAccessLog()",
		},
		WireOp{
			ReplacePkg:     "interceptor",
			ReplaceName:    "Recovery",
			NewPkg:         "logging",
			NewName:        "KitexRecovery",
			ExistsSentinel: "logging.KitexRecovery()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContainsAll(t, output, []string{
		"logging.KitexRequestID()",
		"logging.KitexAccessLog()",
		"logging.KitexRecovery()",
	})
	assertNotContains(t, output, "interceptor.RequestID()")
	assertNotContains(t, output, "interceptor.AccessLog()")
	assertNotContains(t, output, "interceptor.Recovery()")
}

func TestWireFile_KitexCanaryServerWithMarker(t *testing.T) {
	src := `package server

func Run() {
	opts := []Option{
		WithMiddleware(Chain(
			interceptor.RequestID(),
			// ncgo:wire:canary:server-traffic
			interceptor.AccessLog(),
		)),
	}
	_ = opts
}
`
	result, err := WireFile([]byte(src),
		WireOp{
			AddImport:      "github.com/x/demo/internal/base/release",
			ExistsSentinel: `"github.com/x/demo/internal/base/release"`,
		},
		WireOp{
			Marker:         "// ncgo:wire:canary:server-traffic",
			Anchors:        []string{"\t\t\tinterceptor.RequestID(),\n"},
			InsertSrc:      "release.KitexTraffic(),",
			IsExpr:         true,
			ExistsSentinel: "release.KitexTraffic()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContains(t, output, "release.KitexTraffic()")
	// Should be between RequestID and AccessLog
	reqIDIdx := strings.Index(output, "interceptor.RequestID()")
	trafficIdx := strings.Index(output, "release.KitexTraffic()")
	accessIdx := strings.Index(output, "interceptor.AccessLog()")
	if !(reqIDIdx < trafficIdx && trafficIdx < accessIdx) {
		t.Errorf("expected: RequestID < KitexTraffic < AccessLog\ngot idx: %d < %d < %d", reqIDIdx, trafficIdx, accessIdx)
	}
}

func TestWireFile_KitexCanaryServerWithAnchor(t *testing.T) {
	src := `package server

func Run() {
	opts := []Option{
		WithMiddleware(Chain(
			interceptor.RequestID(),
			interceptor.AccessLog(),
		)),
	}
	_ = opts
}
`
	result, err := WireFile([]byte(src),
		WireOp{
			AddImport:      "github.com/x/demo/internal/base/release",
			ExistsSentinel: `"github.com/x/demo/internal/base/release"`,
		},
		WireOp{
			Marker:         "// ncgo:wire:canary:server-traffic",
			Anchors:        []string{"\t\t\tinterceptor.RequestID(),\n"},
			InsertSrc:      "release.KitexTraffic(),",
			IsExpr:         true,
			ExistsSentinel: "release.KitexTraffic()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContains(t, output, "release.KitexTraffic()")
}

func TestWireFile_KitexClientLogging(t *testing.T) {
	src := `package democlient

import (
	"context"

	kitexclient "github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/transmeta"
)

type Config struct {
	EnableMetaInfo bool
}

func New(ctx context.Context, cfg Config, opts ...kitexclient.Option) {
	_ = ctx
	options := make([]kitexclient.Option, 0, len(opts)+6)
	if cfg.EnableMetaInfo {
		options = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))
	}
	options = append(options, opts...)
	_ = options
}
`
	loggingBlock := "\toptions = append(options, kitexclient.WithMiddleware(endpoint.Chain(\n" +
		"\t\tlogging.KitexRequestID(),\n" +
		"\t\tlogging.KitexAccessLog(),\n" +
		"\t)))\n"
	result, err := WireFile([]byte(src),
		WireOp{AddImport: "github.com/x/demo/internal/base/logging"},
		WireOp{
			Marker: "// ncgo:wire:kitex-client:middleware",
			Anchors: []string{"\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"},
			InsertSrc:      loggingBlock,
			ExistsSentinel: "logging.KitexAccessLog()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContainsAll(t, output, []string{
		`"github.com/x/demo/internal/base/logging"`,
		"logging.KitexRequestID()",
		"logging.KitexAccessLog()",
	})
}

func TestWireFile_KitexClientCanary(t *testing.T) {
	src := `package democlient

import (
	"context"

	kitexclient "github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/transmeta"
)

type Config struct {
	EnableMetaInfo bool
}

func New(ctx context.Context, cfg Config, opts ...kitexclient.Option) {
	_ = ctx
	options := make([]kitexclient.Option, 0, len(opts)+6)
	if cfg.EnableMetaInfo {
		options = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))
	}
	options = append(options, opts...)
	_ = options
}
`
	result, err := WireFile([]byte(src),
		WireOp{AddImport: "github.com/x/demo/internal/base/release"},
		WireOp{
			Marker: "// ncgo:wire:kitex-client:middleware",
			Anchors: []string{"\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"},
			InsertSrc:      "\toptions = append(options, kitexclient.WithMiddleware(release.KitexTraffic()))\n",
			ExistsSentinel: "release.KitexTraffic()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	assertContains(t, output, `"github.com/x/demo/internal/base/release"`)
	assertContains(t, output, "release.KitexTraffic()")
}

// ---- Idempotency Tests ----

func TestWireFile_IdempotentImport(t *testing.T) {
	src := `package server

import (
	"fmt"

	"github.com/x/demo/internal/base/logging"
)

func Run() {}
`
	result, err := WireFile([]byte(src),
		WireOp{AddImport: "github.com/x/demo/internal/base/logging"},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	count := strings.Count(output, "github.com/x/demo/internal/base/logging")
	if count != 1 {
		t.Errorf("import should not be duplicated, got %d occurrences", count)
	}
}

func TestWireFile_IdempotentInsert(t *testing.T) {
	src := `package server

func Run() {
	cfg := conf.Get()
	logging.Init(cfg)
	h := server.Default()
}
`
	// Idempotency: logging.Init is already present
	result, err := WireFile([]byte(src),
		WireOp{
			Marker:         "// ncgo:wire:logging:init",
			Anchors:        []string{"\tcfg := conf.Get()\n"},
			InsertSrc:      "\tlogging.Init(cfg)\n",
			ExistsSentinel: "logging.Init(",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	// Should still have only one logging.Init
	count := strings.Count(output, "logging.Init(")
	if count != 1 {
		t.Errorf("idempotent insert should not duplicate, got %d occurrences of logging.Init", count)
	}
}

func TestWireFile_IdempotentReplace(t *testing.T) {
	src := `package server

func Run() {
	h := server.Default()
	h.Use(logging.HertzRecovery())
}
`
	result, err := WireFile([]byte(src),
		WireOp{
			ReplacePkg:     "middleware",
			ReplaceName:    "Recovery",
			NewPkg:         "logging",
			NewName:        "HertzRecovery",
			ExistsSentinel: "logging.HertzRecovery()",
		},
	)
	if err != nil {
		t.Fatalf("WireFile: %v", err)
	}
	output := string(result)
	if !strings.Contains(output, "logging.HertzRecovery()") {
		t.Error("output should still contain logging.HertzRecovery()")
	}
}

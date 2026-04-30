// Optional structured logging add-on for Hertz and Kitex services.
//
// It provides slog-based console/file logging, category-specific rotated files,
// gzip compression through lumberjack, request/trace/release context fields, and
// samber/oops metadata extraction.
//
// Required dependencies:
//
//  go get github.com/samber/oops
//  go get gopkg.in/natefinch/lumberjack.v2
//  go get go.opentelemetry.io/otel/trace

package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/samber/oops"
	oteltrace "go.opentelemetry.io/otel/trace"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const (
	CategoryAccess   = "access"
	CategoryError    = "error"
	CategoryBiz      = "biz"
	CategoryRPC      = "rpc"
	CategoryDB       = "db"
	CategoryPanic    = "panic"
	CategoryAudit    = "audit"
	CategorySecurity = "security"
)

type Config struct {
	Enabled    bool                      `json:"enabled" yaml:"enabled"`
	Mode       string                    `json:"mode" yaml:"mode"`     // console|file|both|none
	Format     string                    `json:"format" yaml:"format"` // json|text
	Level      string                    `json:"level" yaml:"level"`
	AddSource  bool                      `json:"add_source" yaml:"add_source"`
	Console    ConsoleConfig             `json:"console" yaml:"console"`
	File       FileConfig                `json:"file" yaml:"file"`
	Categories map[string]CategoryConfig `json:"categories" yaml:"categories"`
}

type ConsoleConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type FileConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Dir        string `json:"dir" yaml:"dir"`
	Filename   string `json:"filename" yaml:"filename"`
	MaxSizeMB  int    `json:"max_size_mb" yaml:"max_size_mb"`
	MaxBackups int    `json:"max_backups" yaml:"max_backups"`
	MaxAgeDays int    `json:"max_age_days" yaml:"max_age_days"`
	Compress   bool   `json:"compress" yaml:"compress"`
}

type CategoryConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	File    string `json:"file" yaml:"file"`
	Level   string `json:"level" yaml:"level"`
}

type ReleaseInfo struct {
	ServiceName string
	ServiceKind string
	Version     string
	Track       string
	GitSHA      string
	BuildTime   string
	TrafficLane string
}

type Logger struct {
	base       *slog.Logger
	categories map[string]*slog.Logger
	release    ReleaseInfo
}

var (
	defaultLogger *Logger
	defaultMu     sync.RWMutex
)

func DefaultConfig() Config {
	return Config{
		Enabled:   true,
		Mode:      "console",
		Format:    "json",
		Level:     "info",
		AddSource: true,
		Console:   ConsoleConfig{Enabled: true},
		File: FileConfig{
			Enabled: true, Dir: "logs", Filename: "app.log",
			MaxSizeMB: 100, MaxBackups: 10, MaxAgeDays: 30, Compress: true,
		},
		Categories: map[string]CategoryConfig{
			CategoryAccess:   {Enabled: true, File: "access.log", Level: "info"},
			CategoryError:    {Enabled: true, File: "error.log", Level: "error"},
			CategoryBiz:      {Enabled: true, File: "biz.log", Level: "warn"},
			CategoryRPC:      {Enabled: true, File: "rpc.log", Level: "info"},
			CategoryDB:       {Enabled: true, File: "db.log", Level: "warn"},
			CategoryPanic:    {Enabled: true, File: "panic.log", Level: "error"},
			CategoryAudit:    {Enabled: true, File: "audit.log", Level: "info"},
			CategorySecurity: {Enabled: true, File: "security.log", Level: "warn"},
		},
	}
}

func Init(cfg Config, release ReleaseInfo) (*Logger, error) {
	if !cfg.Enabled || cfg.Mode == "none" {
		l := &Logger{base: slog.New(slog.NewTextHandler(io.Discard, nil)), release: release}
		setDefault(l)
		return l, nil
	}
	baseHandler, err := buildHandler(cfg, "", cfg.Level)
	if err != nil {
		return nil, err
	}
	l := &Logger{base: slog.New(baseHandler), categories: map[string]*slog.Logger{}, release: release}
	for category, cc := range cfg.Categories {
		if !cc.Enabled {
			continue
		}
		h, err := buildHandler(cfg, cc.File, firstNonEmpty(cc.Level, cfg.Level))
		if err != nil {
			return nil, err
		}
		l.categories[category] = slog.New(h).With("category", category)
	}
	setDefault(l)
	return l, nil
}

func L() *Logger {
	defaultMu.RLock()
	l := defaultLogger
	defaultMu.RUnlock()
	if l != nil {
		return l
	}
	l, _ = Init(DefaultConfig(), ReleaseInfo{})
	return l
}

func (l *Logger) Info(ctx context.Context, category, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelInfo, category, msg, attrs...)
}

func (l *Logger) Warn(ctx context.Context, category, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelWarn, category, msg, attrs...)
}

func (l *Logger) Error(ctx context.Context, category, msg string, err error, attrs ...slog.Attr) {
	attrs = append(attrs, ErrorAttrs(err)...)
	l.log(ctx, slog.LevelError, category, msg, attrs...)
}

func (l *Logger) log(ctx context.Context, level slog.Level, category, msg string, attrs ...slog.Attr) {
	logger := l.base
	if v, ok := l.categories[category]; ok {
		logger = v
	}
	attrs = append(ContextAttrs(ctx), attrs...)
	attrs = append(l.releaseAttrs(), attrs...)
	logger.LogAttrs(ctx, level, msg, attrs...)
}

func ErrorAttrs(err error) []slog.Attr {
	if err == nil {
		return nil
	}
	attrs := []slog.Attr{slog.String("error.message", err.Error())}
	if oe, ok := oops.AsOops(err); ok {
		attrs = append(attrs,
			slog.Any("error.code", oe.Code()),
			slog.String("error.public", oe.Public()),
			slog.String("error.scope", oe.Domain()),
			slog.Any("error.tags", oe.Tags()),
			slog.Any("error.attrs", oe.Context()),
		)
		if traceID := oe.Trace(); traceID != "" {
			attrs = append(attrs, slog.String("error.trace", traceID))
		}
		if spanID := oe.Span(); spanID != "" {
			attrs = append(attrs, slog.String("error.span", spanID))
		}
	}
	return attrs
}

func ContextAttrs(ctx context.Context) []slog.Attr {
	attrs := []slog.Attr{}
	if requestID, _ := ctx.Value(ContextKeyRequestID).(string); requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if src, _ := ctx.Value(ContextKeyRequestIDSource).(string); src != "" {
		attrs = append(attrs, slog.String("request_id_source", src))
	}
	if lane, _ := ctx.Value(ContextKeyTrafficLane).(string); lane != "" {
		attrs = append(attrs, slog.String("traffic.lane", lane))
	}
	spanCtx := oteltrace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		attrs = append(attrs,
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return attrs
}

type contextKey string

const (
	ContextKeyRequestID       contextKey = "request_id"
	ContextKeyRequestIDSource contextKey = "request_id_source"
	ContextKeyTrafficLane     contextKey = "traffic_lane"
)

func WithRequestID(ctx context.Context, requestID, source string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyRequestID, requestID)
	return context.WithValue(ctx, ContextKeyRequestIDSource, source)
}

func WithTrafficLane(ctx context.Context, lane string) context.Context {
	return context.WithValue(ctx, ContextKeyTrafficLane, lane)
}

func (l *Logger) releaseAttrs() []slog.Attr {
	r := l.release
	attrs := []slog.Attr{}
	put := func(k, v string) {
		if v != "" {
			attrs = append(attrs, slog.String(k, v))
		}
	}
	put("service.name", r.ServiceName)
	put("service.kind", r.ServiceKind)
	put("service.version", r.Version)
	put("release.track", r.Track)
	put("release.git_sha", r.GitSHA)
	put("release.build_time", r.BuildTime)
	put("traffic.lane", r.TrafficLane)
	return attrs
}

func buildHandler(cfg Config, categoryFile, level string) (slog.Handler, error) {
	opts := &slog.HandlerOptions{AddSource: cfg.AddSource, Level: parseLevel(level)}
	handlers := []slog.Handler{}
	if cfg.Mode == "console" || cfg.Mode == "both" {
		if cfg.Console.Enabled {
			handlers = append(handlers, newHandler(os.Stdout, cfg.Format, opts))
		}
	}
	if cfg.Mode == "file" || cfg.Mode == "both" {
		if cfg.File.Enabled {
			if err := os.MkdirAll(cfg.File.Dir, 0o755); err != nil {
				return nil, err
			}
			fileName := firstNonEmpty(categoryFile, cfg.File.Filename)
			w := &lumberjack.Logger{
				Filename:   filepath.Join(cfg.File.Dir, fileName),
				MaxSize:    cfg.File.MaxSizeMB,
				MaxBackups: cfg.File.MaxBackups,
				MaxAge:     cfg.File.MaxAgeDays,
				Compress:   cfg.File.Compress,
			}
			handlers = append(handlers, newHandler(w, cfg.Format, opts))
		}
	}
	if len(handlers) == 0 {
		handlers = append(handlers, newHandler(io.Discard, cfg.Format, opts))
	}
	return multiHandler{handlers: handlers}, nil
}

func newHandler(w io.Writer, format string, opts *slog.HandlerOptions) slog.Handler {
	if strings.EqualFold(format, "text") {
		return slog.NewTextHandler(w, opts)
	}
	return slog.NewJSONHandler(w, opts)
}

type multiHandler struct{ handlers []slog.Handler }

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, child := range h.handlers {
		if child.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, child := range h.handlers {
		if err := child.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, 0, len(h.handlers))
	for _, child := range h.handlers {
		out = append(out, child.WithAttrs(attrs))
	}
	return multiHandler{handlers: out}
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, 0, len(h.handlers))
	for _, child := range h.handlers {
		out = append(out, child.WithGroup(name))
	}
	return multiHandler{handlers: out}
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func setDefault(l *Logger) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultLogger = l
	slog.SetDefault(l.base)
}

func SinceMS(started time.Time) slog.Attr {
	return slog.Float64("latency_ms", float64(time.Since(started).Microseconds())/1000)
}

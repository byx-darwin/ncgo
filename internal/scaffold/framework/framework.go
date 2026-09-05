// Package framework abstracts the differences between the two scaffold
// frameworks ncgo supports today — Hertz (manifest.KindHertz) and Kitex
// (manifest.KindKitex) — behind a small set of segregated interfaces
// composed into Adapter. It exists to remove the repeated Kind-branching
// switch/if statements spread across internal/scaffold/{mono,infra,shared}.
//
// framework MUST NOT import internal/scaffold/mono, internal/scaffold/infra,
// internal/scaffold/shared, or internal/scaffold/template — all of those
// import framework, so a back-import would create a cycle. It may import
// internal/manifest, internal/assets, internal/exec, and the standard
// library.
package framework

import (
	"context"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// GeneratorOptions carries only the mono.Options fields that Generator
// methods need. It is a standalone type (not mono.Options) because
// internal/scaffold/mono imports this package, not the other way around.
type GeneratorOptions struct {
	Name   string // service name
	Module string // Go module path
}

// AssetResolver resolves embedded-asset source paths that differ by
// framework kind.
type AssetResolver interface {
	// OptionalAssetPath returns the embedded source path for infraKind's
	// `ncgo add infra` optional add-on Go file (e.g. "hertz/optional/redis.go"),
	// and whether this framework has one.
	OptionalAssetPath(infraKind string) (string, bool)
	// HertzConfigAssetPath returns the embedded hertz/optional-config/<kind>.yaml
	// path for infraKind, and whether this framework has a per-kind Hertz
	// conf.yaml snippet. The Kitex adapter always returns ("", false).
	HertzConfigAssetPath(infraKind string) (string, bool)
}

// ConfigWrite is a planned conf/dev/conf.yaml write.
type ConfigWrite struct {
	Body   []byte
	Action string // "create" | "update"
}

// ConfigMerger computes planned conf/dev/conf.yaml writes for `ncgo add
// infra`. Only Hertz owns per-plugin config-key merging; only Kitex owns
// rate_limit block merging. The adapter that doesn't own a given merge
// returns (nil, nil).
type ConfigMerger interface {
	// MergeHertzConfig merges a plugin's Hertz config snippet (already read
	// by the caller from the path returned by HertzConfigAssetPath) into the
	// existing conf/dev/conf.yaml content. current is nil when the file does
	// not exist yet. Returns (nil, nil) when no write is needed.
	MergeHertzConfig(infraKind, hertzConfigKey string, snippet, current []byte, force bool) (*ConfigWrite, error)
	// MergeRateLimitConfig merges the kitex rate_limit infra add-on's
	// conf/dev/conf.yaml block. current is nil when the file does not exist
	// yet. Returns (nil, nil) when no write is needed.
	MergeRateLimitConfig(current []byte, force bool) (*ConfigWrite, error)
}

// ComposeFeatureFlags are the compose-feature flags a framework enables on
// top of the infra-list-derived ones (postgres/redis/kafka/es/clickhouse are
// common to both kinds and computed by the caller).
type ComposeFeatureFlags struct {
	Nacos   bool // config-center + service-discovery nacos profile
	Polaris bool // config-center + service-discovery polaris profile
	Vegeta  bool // rate-limit load-testing sidecar
}

// ContainerRenderer resolves docker/compose differences that vary by kind.
type ContainerRenderer interface {
	// DockerConfigBlocks returns the conf/docker/conf.yaml YAML blocks (after
	// the shared "env: docker" header) specific to this framework and the
	// manifest's enabled infra/database flags.
	DockerConfigBlocks(m *manifest.Manifest) []string
	// ContainerPort is the fixed in-container listen port (Hertz: 8080,
	// Kitex: 8888).
	ContainerPort() int
	// ComposeFeatures reports which compose profile flags this framework
	// enables unconditionally (or when withDatabase is true).
	ComposeFeatures(withDatabase bool) ComposeFeatureFlags
}

// Generator resolves IDL path/content and code-generator invocation
// differences that vary by kind.
type Generator interface {
	// IDLPath is the default IDL path when Options.IDL is empty.
	IDLPath(opts GeneratorOptions) string
	// IDLNameToken is the per-kind service-name token substituted for
	// `{{ToLower .ServiceName}}` in a template package's relative IDL paths.
	IDLNameToken(opts GeneratorOptions) string
	// GeneratorCommand is the literal shell command a user can paste to
	// invoke this framework's generator.
	GeneratorCommand(opts GeneratorOptions, idl string) string
	// RenderIDLPlaceholder is the starter IDL file content dropped into the
	// scaffold when no IDL exists yet.
	RenderIDLPlaceholder(opts GeneratorOptions) string
	// WriteIDLSupportFiles writes any framework-specific support files that
	// must exist next to the IDL before the generator runs (Hertz: api.proto
	// + openapi/*.proto + validate/validate.proto; Kitex: no-op).
	WriteIDLSupportFiles(dir string) error
	// RunGenerator invokes this framework's code generator (hz or kitex).
	RunGenerator(ctx context.Context, r exec.Runner, dir string, opts GeneratorOptions, idl string) (exec.Result, error)
	// RequiresSQLCBeforeTidy reports whether `make sqlc` must run before the
	// first `go mod tidy` for a scaffold using this framework.
	RequiresSQLCBeforeTidy(withDatabase bool) bool
}

// Wirer resolves the one framework-specific value infra/wire.go's dispatch
// needs. The bulk of --wire logic stays in internal/scaffold/infra because it
// depends on infra-package-only Plugin capability interfaces that framework
// cannot import back (see plan Documented Scope Decisions).
type Wirer interface {
	// ServerFilePath is the project-relative (slash-separated) path of the
	// generated server bootstrap file --wire patches first.
	ServerFilePath() string
}

// Adapter is the full per-framework capability set. Concrete adapters
// (hertzAdapter, kitexAdapter) are defined in adapter_hertz.go/adapter_kitex.go
// and register themselves via init().
type Adapter interface {
	Kind() string
	AssetResolver
	ConfigMerger
	ContainerRenderer
	Generator
	Wirer
}

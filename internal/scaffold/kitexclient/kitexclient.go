// Package kitexclient implements `ncgo add kitex-client <name>` for generating
// Kitex client wrappers in an RPC service.
//
// Run this command from the RPC service directory whose module matches the
// proto's go_package. The generated kitex_gen/ types live in the calling
// module — a BFF should depend on the RPC module, not re-generate these types.
//
// Unlike the previous skeleton-only approach, Add now:
//  1. Calls the kitex CLI to generate kitex_gen/ types from the proto IDL
//  2. Generates a complete client wrapper that proxies all RPC methods
//  3. Runs go mod tidy to resolve dependencies
package kitexclient

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/protolint"
)

// Options describes a `ncgo add kitex-client` invocation.
type Options struct {
	Root    string      // project root
	Name    string      // client name (e.g. rbac, rulecenter)
	Service string      // RPC service name
	IDL     string      // path to proto file (relative to Root)
	Module  string      // Go module path; auto-detected from go.mod when empty
	Force   bool        // overwrite existing files
	DryRun  bool        // preview mode
	Runner  exec.Runner // injected exec; nil means exec.NewDefault()
}

// Result describes what Add produced.
type Result struct {
	DryRun       bool     `json:"dryRun"`
	WrittenPaths []string `json:"writtenPaths"`
	NextSteps    []string `json:"nextSteps"`
}

// Add generates the Kitex client wrapper and config for an RPC service.
//
// The command must be run from the RPC service directory: the proto's
// go_package path is expected to start with the current module path. When it
// does not, the generated kitex_gen/ layout would not match the module's
// import paths — in that case Add returns an error telling the user to run
// from the RPC service whose module owns the proto.
func Add(ctx context.Context, opts Options) (*Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Name == "" {
		return nil, fmt.Errorf("kitex-client: name is required")
	}
	if opts.Service == "" {
		return nil, fmt.Errorf("kitex-client: --service is required")
	}
	if opts.IDL == "" {
		return nil, fmt.Errorf("kitex-client: --idl is required")
	}

	// Derive module from go.mod when not explicitly provided.
	if opts.Module == "" {
		mod, err := detectModule(opts.Root)
		if err != nil {
			return nil, fmt.Errorf("kitex-client: --module is required (or provide a go.mod at root): %w", err)
		}
		opts.Module = mod
	}

	result := &Result{DryRun: opts.DryRun}

	// Parse proto to extract go_package, package name, and services.
	services, pkgDir, pkgName, err := parseProtoServices(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("kitex-client: parse proto: %w", err)
	}

	// The proto's go_package must belong to the current module. kitex nests
	// kitex_gen under the full go_package path; if that path does not start
	// with the current module, the generated tree will not match the module's
	// import layout.
	if err := checkModuleOwnership(opts); err != nil {
		return nil, err
	}

	// Find the matching service.
	svc, err := findService(services, opts.Service)
	if err != nil {
		return nil, err
	}

	// 1. Call kitex to generate kitex_gen/.
	if err := generateKitexTypes(ctx, opts, result); err != nil {
		return nil, err
	}

	// Create pkg/client/<name>/ directory
	clientDir := filepath.Join(opts.Root, "pkg", "client", opts.Name)
	if !opts.DryRun {
		if err := os.MkdirAll(clientDir, 0o755); err != nil {
			return nil, fmt.Errorf("kitex-client: mkdir %s: %w", clientDir, err)
		}
	}

	// 2. Generate complete client wrapper
	clientPath := filepath.Join(clientDir, "client.go")
	if err := generateClient(clientPath, opts, svc, pkgDir, pkgName, result); err != nil {
		return nil, err
	}

	// 3. Generate config.go
	configPath := filepath.Join(clientDir, "config.go")
	if err := generateConfig(configPath, opts, result); err != nil {
		return nil, err
	}

	// 4. Update go.mod dependencies
	if err := updateGoMod(ctx, opts); err != nil {
		return nil, fmt.Errorf("kitex-client: go mod tidy failed: %w", err)
	}

	result.NextSteps = []string{
		fmt.Sprintf("Import the client: %q", fmt.Sprintf("%s/pkg/client/%s", opts.Module, opts.Name)),
	}

	return result, nil
}

// checkModuleOwnership verifies that the proto's go_package belongs to the
// current module. kitex generates kitex_gen under the go_package path, so a
// mismatch produces code whose import paths cannot resolve.
func checkModuleOwnership(opts Options) error {
	model, err := protolint.Load(context.Background(), protolint.LoadOptions{
		Root:  opts.Root,
		Files: []string{opts.IDL},
	})
	if err != nil {
		return fmt.Errorf("kitex-client: parse proto: %w", err)
	}
	if len(model.Files) == 0 {
		return fmt.Errorf("kitex-client: no files compiled from %s", opts.IDL)
	}
	goPkg := model.Files[0].GoPackage
	// Extract the path component (strip ;name suffix).
	goPkgPath := goPkg
	if idx := strings.Index(goPkg, ";"); idx >= 0 {
		goPkgPath = goPkg[:idx]
	}
	if goPkgPath == "" {
		return nil // no go_package — kitex uses proto package name, always local
	}
	if strings.HasPrefix(goPkgPath, opts.Module+"/") || goPkgPath == opts.Module {
		return nil
	}
	return fmt.Errorf("kitex-client: proto go_package %q does not belong to module %q — run this command from the RPC service that owns the proto, not from a BFF",
		goPkgPath, opts.Module)
}

// runner returns the configured Runner or the default.
func (o Options) runner() exec.Runner {
	if o.Runner != nil {
		return o.Runner
	}
	return exec.NewDefault()
}

// generateKitexTypes calls the kitex CLI to generate kitex_gen/ types from the
// proto IDL. This must happen before generating the client wrapper because the
// wrapper imports the generated types.
func generateKitexTypes(ctx context.Context, opts Options, result *Result) error {
	if opts.DryRun {
		result.WrittenPaths = append(result.WrittenPaths, "kitex_gen/")
		return nil
	}
	r := opts.runner()
	args := []string{
		"-module", opts.Module,
		"-type", "protobuf",
		opts.IDL,
	}
	if _, err := exec.Kitex(ctx, r, opts.Root, args...); err != nil {
		return fmt.Errorf("kitex generation failed: %w", err)
	}
	result.WrittenPaths = append(result.WrittenPaths, "kitex_gen/")
	return nil
}

// protoServiceInfo is the subset of service metadata needed to render the
// client wrapper.
type protoServiceInfo struct {
	ServiceName string
	Methods     []protoMethodInfo
}

type protoMethodInfo struct {
	Name         string
	RequestType  string
	ResponseType string
}

// parseProtoServices extracts the proto go_package value, package name, and
// service definitions from the IDL file. It uses protolint for robust proto
// compilation.
func parseProtoServices(ctx context.Context, opts Options) ([]protoServiceInfo, string, string, error) {
	model, err := protolint.Load(ctx, protolint.LoadOptions{
		Root:  opts.Root,
		Files: []string{opts.IDL},
	})
	if err != nil {
		return nil, "", "", err
	}
	if len(model.Files) == 0 {
		return nil, "", "", fmt.Errorf("no files compiled from %s", opts.IDL)
	}

	goPkg := model.Files[0].GoPackage
	pkgDir, pkgName := parseGoPackage(goPkg)
	// When go_package is unset, fall back to the proto package name so the
	// generated code still resolves to the kitex_gen layout kitex produces.
	if pkgDir == "" {
		pkgDir = string(model.Files[0].Package)
	}
	if pkgName == "" {
		pkgName = pkgDir
	}

	var services []protoServiceInfo
	for _, svc := range model.Files[0].Services {
		si := protoServiceInfo{ServiceName: svc.Name}
		for _, rpc := range svc.RPCs {
			si.Methods = append(si.Methods, protoMethodInfo{
				Name:         rpc.Name,
				RequestType:  rpc.InputMessageName,
				ResponseType: rpc.OutputMessageName,
			})
		}
		services = append(services, si)
	}
	return services, pkgDir, pkgName, nil
}

// parseGoPackage splits a go_package option value into its path and name
// components. The format is "path;pkg" or just "path". When the ;pkg suffix
// is present it is used as the package name; otherwise the last path segment
// is used. The returned pkgDir is the last path segment (the kitex_gen
// subdirectory), which is where kitex actually writes generated files.
func parseGoPackage(goPkg string) (pkgDir, pkgName string) {
	if goPkg == "" {
		return "", ""
	}
	path := goPkg
	if idx := strings.Index(goPkg, ";"); idx >= 0 {
		path = goPkg[:idx]
		pkgName = goPkg[idx+1:]
	}
	// The kitex_gen subdirectory is the last segment of the path.
	if path != "" {
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			pkgDir = path[idx+1:]
		} else {
			pkgDir = path
		}
	}
	// Fall back to the path segment when no ;name suffix was given.
	if pkgName == "" {
		pkgName = pkgDir
	}
	return pkgDir, pkgName
}

// sanitizeGoPackageName strips characters that are not valid in a Go
// identifier (letters, digits, underscore). Used to derive a valid package
// name from user-supplied names like "edge-rpc".
func sanitizeGoPackageName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "client"
	}
	return b.String()
}

// findService selects the requested service by name. When only one service
// exists in the proto and opts.Service matches the proto package, it is
// returned as a convenience.
func findService(services []protoServiceInfo, name string) (protoServiceInfo, error) {
	for _, s := range services {
		if s.ServiceName == name {
			return s, nil
		}
	}
	if len(services) == 1 {
		return services[0], nil
	}
	var names []string
	for _, s := range services {
		names = append(names, s.ServiceName)
	}
	return protoServiceInfo{}, fmt.Errorf("kitex-client: service %q not found in proto (available: %s)", name, strings.Join(names, ", "))
}

type clientTemplateData struct {
	Name           string // sanitized Go package name
	Service        string // RPC service name
	Module         string // Go module path
	KitexGenDir    string // proto package (kitex_gen subdirectory)
	TypesPkgName   string // proto package name for request/response types
	ServicePkgName string // service sub-package name for Client interface
	Methods        []protoMethodInfo
}

func generateClient(path string, opts Options, svc protoServiceInfo, pkgDir, pkgName string, result *Result) error {
	if !opts.Force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("kitex-client: %s already exists (use --force to overwrite)", path)
		}
	}

	svcPkgName := strings.ToLower(svc.ServiceName)
	data := clientTemplateData{
		Name:           sanitizeGoPackageName(opts.Name),
		Service:        opts.Service,
		Module:         opts.Module,
		KitexGenDir:    pkgDir,
		TypesPkgName:   pkgName,
		ServicePkgName: svcPkgName,
		Methods:        svc.Methods,
	}

	tmpl := `package {{.Name}}

import (
	"context"
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-framework/config/kitex"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	remoteConnpool "github.com/cloudwego/kitex/pkg/remote/connpool"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/transport"
{{if eq .KitexGenDir .ServicePkgName}}
	typespkg "{{.Module}}/kitex_gen/{{.KitexGenDir}}"
	"{{.Module}}/kitex_gen/{{.KitexGenDir}}/{{.ServicePkgName}}"
{{else}}
	"{{.Module}}/kitex_gen/{{.KitexGenDir}}"
	"{{.Module}}/kitex_gen/{{.KitexGenDir}}/{{.ServicePkgName}}"
{{end}}
)

// Client wraps the Kitex generated client for {{.Service}}.
type Client struct {
	c {{.ServicePkgName}}.Client
}

// New creates a new Kitex client for {{.Service}} using the provided config.
func New(cfg *kitex.ClientConfig, opts ...client.Option) (*Client, error) {
	options := []client.Option{
		client.WithPayloadCodec(thrift.NewThriftCodec()),
		client.WithMetaHandler(transmeta.ClientTTHeaderHandler),
		client.WithTransportProtocol(transport.TTHeaderStreaming),
	}

	if cfg.RPC != nil && cfg.RPC.Intranet != "" {
		options = append(options, client.WithHostPorts(cfg.RPC.Intranet))
	}

	if cfg.ClientOption.Timeout.ConnectTimeOut > 0 {
		options = append(options, client.WithConnectTimeout(cfg.ClientOption.Timeout.ConnectTimeOut))
	} else {
		options = append(options, client.WithConnectTimeout(50*time.Millisecond))
	}

	if cfg.ClientOption.Timeout.RPCTimeout > 0 {
		options = append(options, client.WithRPCTimeout(cfg.ClientOption.Timeout.RPCTimeout))
	}

	options = append(options, client.WithConnPool(remoteConnpool.NewLongPool(
		cfg.ClientOption.Resolver.Name,
		connpool.IdleConfig{
			MinIdlePerAddress: cfg.ClientOption.ConnPool.MinIdlePerAddress,
			MaxIdlePerAddress: cfg.ClientOption.ConnPool.MaxIdlePerAddress,
			MaxIdleGlobal:     cfg.ClientOption.ConnPool.MaxIdleGlobal,
			MaxIdleTimeout:    cfg.ClientOption.ConnPool.MaxIdleTimeout,
		},
	)))

	if cfg.ClientOption.Failure.Enable {
		fp := retry.NewFailurePolicy()
		fp.WithMaxRetryTimes(cfg.ClientOption.Failure.MaxRetryTimes)
		options = append(options, client.WithFailureRetry(fp))
	}

	if cfg.ClientOption.LoadBalancer.Enable {
		options = append(options, client.WithLoadBalancer(loadbalance.NewConsistBalancer(
			loadbalance.NewConsistentHashOption(func(ctx context.Context, request any) string {
				if s, ok := request.(interface{ Key() string }); ok {
					return s.Key()
				}
				return ""
			}),
		)))
	}

	options = append(options, client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{
		ServiceName: cfg.ClientOption.Resolver.Name,
	}))

	options = append(options, opts...)

	c, err := {{.ServicePkgName}}.NewClient(cfg.ClientOption.Resolver.Name, options...)
	if err != nil {
		return nil, fmt.Errorf("kitex-client {{.Name}}: create client: %w", err)
	}
	return &Client{c: c}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	return nil
}
{{range .Methods}}
// {{.Name}} proxies the {{.Name}} RPC method.
func (c *Client) {{.Name}}(ctx context.Context, req *{{if eq $.KitexGenDir $.ServicePkgName}}typespkg{{else}}{{$.TypesPkgName}}{{end}}.{{.RequestType}}) (*{{if eq $.KitexGenDir $.ServicePkgName}}typespkg{{else}}{{$.TypesPkgName}}{{end}}.{{.ResponseType}}, error) {
	return c.c.{{.Name}}(ctx, req)
}
{{end}}`

	t, err := template.New("client").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("kitex-client: parse client template: %w", err)
	}

	if !opts.DryRun {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("kitex-client: create %s: %w", path, err)
		}
		defer func() { _ = f.Close() }()
		if err := t.Execute(f, data); err != nil {
			return fmt.Errorf("kitex-client: execute client template: %w", err)
		}
	}
	result.WrittenPaths = append(result.WrittenPaths, path)
	return nil
}

func generateConfig(path string, opts Options, result *Result) error {
	if !opts.Force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("kitex-client: %s already exists (use --force to overwrite)", path)
		}
	}

	content := fmt.Sprintf(`package %s

import "github.com/byx-darwin/go-tools/go-framework/config/kitex"

// Config aliases the Kitex client config from go-framework.
// Use kitex.ClientConfig directly; this alias keeps callers importing from
// the generated client package.
type Config = kitex.ClientConfig

// DefaultConfig returns a minimal default configuration.
func DefaultConfig() *kitex.ClientConfig {
	return &kitex.ClientConfig{
		RPC: &kitex.RPCServerOption{
			Intranet: "localhost:8888",
		},
		ClientOption: &kitex.ClientOption{
			Timeout: kitex.ClientTimeout{
				ConnectTimeOut: 50000000,  // 50ms in nanoseconds
				RPCTimeout:     200000000, // 200ms in nanoseconds
			},
		},
	}
}
`, sanitizeGoPackageName(opts.Name))

	if !opts.DryRun {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("kitex-client: write %s: %w", path, err)
		}
	}
	result.WrittenPaths = append(result.WrittenPaths, path)
	return nil
}

// updateGoMod runs `go mod tidy` in the project root to resolve the kitex
// dependency and any transitive imports.
func updateGoMod(ctx context.Context, opts Options) error {
	if opts.DryRun {
		return nil
	}
	r := opts.runner()
	_, err := exec.GoModTidy(ctx, r, opts.Root)
	return err
}

// detectModule reads the first "module" directive from go.mod at root.
func detectModule(root string) (string, error) {
	f, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}

// Package kitexclient implements `ncgo add kitex-client <name>` for generating
// Kitex client wrappers that BFF services use to call RPC services.
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
	"regexp"
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

// Add generates the Kitex client wrapper and config for calling an RPC service.
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

	// 2. Parse proto to extract go_package dir, package name, and services.
	//    This must happen before kitex generation so we can rewrite go_package
	//    to a flat package name (kitex otherwise nests kitex_gen at the full
	//    go_package path, which doesn't match the calling module).
	services, pkgDir, pkgName, err := parseProtoServices(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("kitex-client: parse proto: %w", err)
	}

	// Find the matching service.
	svc, err := findService(services, opts.Service)
	if err != nil {
		return nil, err
	}

	// 1. Call kitex to generate kitex_gen/<pkgDir>/.
	//    Rewrites go_package to just the package name so kitex generates at a
	//    flat path that matches the client wrapper's imports.
	cleanup, err := generateKitexTypes(ctx, opts, pkgName, result)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Create pkg/client/<name>/ directory
	clientDir := filepath.Join(opts.Root, "pkg", "client", opts.Name)
	if !opts.DryRun {
		if err := os.MkdirAll(clientDir, 0o755); err != nil {
			return nil, fmt.Errorf("kitex-client: mkdir %s: %w", clientDir, err)
		}
	}

	// 3. Generate complete client wrapper
	clientPath := filepath.Join(clientDir, "client.go")
	if err := generateClient(clientPath, opts, svc, pkgDir, pkgName, result); err != nil {
		return nil, err
	}

	// 4. Generate config.go
	configPath := filepath.Join(clientDir, "config.go")
	if err := generateConfig(configPath, opts, result); err != nil {
		return nil, err
	}

	// 5. Update go.mod dependencies
	if err := updateGoMod(ctx, opts); err != nil {
		return nil, fmt.Errorf("kitex-client: go mod tidy failed: %w", err)
	}

	result.NextSteps = []string{
		fmt.Sprintf("Import the client: %q", fmt.Sprintf("%s/pkg/client/%s", opts.Module, opts.Name)),
	}

	return result, nil
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
//
// kitex nests the generated tree under the full go_package path (e.g.
// kitex_gen/a/b/c/), which does not match the calling module's import paths.
// To keep the layout flat (kitex_gen/<pkg>/), the go_package option is
// rewritten to just the package name in a temporary copy of the proto. The
// returned cleanup function removes that temporary file.
func generateKitexTypes(ctx context.Context, opts Options, pkgName string, result *Result) (func(), error) {
	if opts.DryRun {
		result.WrittenPaths = append(result.WrittenPaths, "kitex_gen/")
		return nil, nil
	}
	r := opts.runner()

	idlArg := opts.IDL
	var cleanup func()

	// If the proto's go_package has a path component, rewrite it to just the
	// package name in a temp copy so kitex generates at kitex_gen/<pkg>/
	// instead of the full nested path. When go_package is already flat (or
	// absent), use the original proto as-is.
	if tmpRel, changed, err := rewriteGoPackage(opts, pkgName); err != nil {
		return nil, err
	} else if changed {
		idlArg = tmpRel
		cleanup = func() {
			_ = os.Remove(filepath.Join(opts.Root, tmpRel))
		}
	}

	args := []string{
		"-module", opts.Module,
		"-type", "protobuf",
		idlArg,
	}
	if _, err := exec.Kitex(ctx, r, opts.Root, args...); err != nil {
		return nil, fmt.Errorf("kitex generation failed: %w", err)
	}
	result.WrittenPaths = append(result.WrittenPaths, "kitex_gen/")
	return cleanup, nil
}

// rewriteGoPackage checks whether the proto at idlRel has a go_package option
// with a path component. If so, it writes a temp copy (sibling to the
// original) with go_package rewritten to the flat "<pkg>;<pkg>" and returns
// its path relative to opts.Root. The caller is responsible for removing the
// temp file. When go_package is absent or already flat, ("", false, nil) is
// returned and the original proto is used as-is.
func rewriteGoPackage(opts Options, pkgName string) (string, bool, error) {
	protoPath := filepath.Join(opts.Root, opts.IDL)
	src, err := os.ReadFile(protoPath)
	if err != nil {
		return "", false, err
	}
	re := regexp.MustCompile(`(?m)(option\s+go_package\s*=\s*")[^"]*("\s*;)`)
	match := re.FindSubmatch(src)
	if match == nil {
		return "", false, nil // no go_package — kitex uses proto package name
	}
	core := string(match[0])
	core = strings.TrimPrefix(core, `option go_package = "`)
	core = strings.TrimSuffix(core, `";`)
	if !strings.Contains(core, "/") {
		return "", false, nil // already flat
	}
	modified := re.ReplaceAll(src, []byte(`${1}`+pkgName+`;`+pkgName+`${2}`))

	// Write temp copy next to the original so relative imports still resolve.
	dir := filepath.Dir(protoPath)
	base := filepath.Base(opts.IDL)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	tmpPath := filepath.Join(dir, name+".ncgo.tmp"+ext)
	if err := os.WriteFile(tmpPath, modified, 0o644); err != nil {
		return "", false, err
	}
	rel, err := filepath.Rel(opts.Root, tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return "", false, err
	}
	return rel, true, nil
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
	ServiceName    string // sanitized service name (kitex_gen sub-package)
	Module         string // Go module path
	KitexGenDir    string // last segment of go_package path (kitex_gen subdir)
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
		ServiceName:    svcPkgName,
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

	"{{.Module}}/kitex_gen/{{.KitexGenDir}}"
	"{{.Module}}/kitex_gen/{{.KitexGenDir}}/{{.ServicePkgName}}"
	"github.com/cloudwego/kitex/client"
)

// Client wraps the Kitex generated client for {{.Service}}.
type Client struct {
	c {{.ServicePkgName}}.Client
}

// New creates a new Kitex client for {{.Service}}.
func New(addr string, opts ...client.Option) (*Client, error) {
	c, err := {{.ServicePkgName}}.NewClient(addr, opts...)
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
func (c *Client) {{.Name}}(ctx context.Context, req *{{$.TypesPkgName}}.{{.RequestType}}) (*{{$.TypesPkgName}}.{{.ResponseType}}, error) {
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
		defer f.Close()
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

// Config holds configuration for the %s Kitex client.
type Config struct {
	Address string `+"`yaml:\"address\"`"+`
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		Address: "localhost:8888",
	}
}
`, sanitizeGoPackageName(opts.Name), opts.Name)

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

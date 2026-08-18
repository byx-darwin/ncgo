package template

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ParseAllServices reads a proto file and extracts ServiceInfo for each service.
func ParseAllServices(ctx context.Context, protoPath string, module string) ([]ServiceInfo, error) {
	abs, err := filepath.Abs(protoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve proto path: %w", err)
	}
	roots, target := importRootsAndTarget(abs)

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: roots,
		}),
	}
	files, err := compiler.Compile(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("compile proto: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files compiled from %s", protoPath)
	}
	fd := files[0]

	var result []ServiceInfo
	for i := 0; i < fd.Services().Len(); i++ {
		sd := fd.Services().Get(i)
		si := ServiceInfo{
			ServiceName: string(sd.Name()),
			ImportPath:  module,
			PkgRefName:  pkgRefName(module),
		}
		for j := 0; j < sd.Methods().Len(); j++ {
			md := sd.Methods().Get(j)
			si.Methods = append(si.Methods, MethodInfo{
				Name: string(md.Name()),
				Resp: MethodResp{
					Type: protoTypeToGo(md.Output()),
				},
			})
		}
		result = append(result, si)
	}

	if len(result) == 0 {
		result = append(result, ServiceInfo{
			ServiceName: defaultServiceName(protoPath),
			ImportPath:  module,
			PkgRefName:  pkgRefName(module),
		})
	}

	return result, nil
}

// ParseServiceInfo returns the first service from ParseAllServices for backward compatibility.
func ParseServiceInfo(ctx context.Context, protoPath string, module string) (*ServiceInfo, error) {
	services, err := ParseAllServices(ctx, protoPath, module)
	if err != nil {
		return nil, err
	}
	return &services[0], nil
}

// importRootsAndTarget derives protoc-style import roots and the compile
// target (relative to the first root) for a proto file. Hertz service protos
// live at <root>/idl/app/<svc>.proto and import files rooted at idl/
// (matching `hz -I idl`), so the idl/ ancestor must be an import root. The
// proto's own directory is added for sibling imports; Kitex protos at
// <root>/idl/<svc>.proto resolve via the idl/ root too. When there is no idl/
// ancestor, compile relative to the proto's own directory.
func importRootsAndTarget(abs string) (roots []string, target string) {
	protoDir := filepath.Dir(abs)
	if idlRoot := findIDLRoot(abs); idlRoot != "" {
		if rel, err := filepath.Rel(idlRoot, abs); err == nil {
			roots = []string{idlRoot}
			if protoDir != idlRoot {
				roots = append(roots, protoDir)
			}
			return roots, filepath.ToSlash(rel)
		}
	}
	return []string{protoDir}, filepath.Base(abs)
}

// findIDLRoot returns the nearest ancestor directory named "idl", or "" if
// none exists.
func findIDLRoot(abs string) string {
	dir := filepath.Dir(abs)
	for {
		if filepath.Base(dir) == "idl" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func defaultServiceName(protoPath string) string {
	base := filepath.Base(protoPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return exportName(name)
}

func pkgRefName(module string) string {
	parts := strings.Split(module, "/")
	return parts[len(parts)-1]
}

func protoTypeToGo(md protoreflect.MessageDescriptor) string {
	return string(md.Name())
}

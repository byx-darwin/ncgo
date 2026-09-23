// Package compat defines the dependency/tool compatibility set used by ncgo
// and the projects it generates. Keep release documentation and scaffold
// contract tests aligned with these values.
package compat

const (
	GeneratedGoVersion = "1.26.5"

	GoToolsCommonVersion     = "v0.3.0"
	GoToolsFrameworkVersion  = "v0.3.0"
	GoToolsMiddlewareVersion = "v0.1.0"

	MinGoVersion    = "v1.25.0"
	MinHzVersion    = "v0.9.7"
	MinKitexVersion = "v0.16.1"
)

type ModuleVersion struct {
	Path    string
	Version string
	Field   string
}

// GoToolsModules is the complete go-tools compatibility set pinned in every
// generated service module. Field is the Hertz layout data variable.
var GoToolsModules = []ModuleVersion{
	{Path: "github.com/byx-darwin/go-tools/go-common", Version: GoToolsCommonVersion, Field: "GoToolsCommonVersion"},
	{Path: "github.com/byx-darwin/go-tools/go-framework", Version: GoToolsFrameworkVersion, Field: "GoToolsFrameworkVersion"},
	{Path: "github.com/byx-darwin/go-tools/go-middleware", Version: GoToolsMiddlewareVersion, Field: "GoToolsMiddlewareVersion"},
}

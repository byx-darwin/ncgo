// Package template extracts code templates from existing ncgo projects
// and applies them during new project generation.
package template

// TemplateFile represents a single code template as YAML.
type TemplateFile struct {
	Path           string         `yaml:"path"`
	UpdateBehavior UpdateBehavior `yaml:"update_behavior"`
	LoopService    bool           `yaml:"loop_service,omitempty"`
	Body           string         `yaml:"body"`
}

// UpdateBehavior controls how a template is applied.
type UpdateBehavior struct {
	Type string `yaml:"type"` // "skip" or "cover"
}

// ServiceInfo holds proto-level information for template variables.
type ServiceInfo struct {
	ServiceName string
	ImportPath  string
	PkgRefName  string
	Methods     []MethodInfo
}

// MethodInfo describes a single RPC method.
type MethodInfo struct {
	Name string
	Args []MethodArg
	Resp MethodResp
}

// MethodArg describes an RPC method argument.
type MethodArg struct {
	Name string
	Type string
}

// MethodResp describes an RPC method return type.
type MethodResp struct {
	Type string
}

// RenderData is the top-level template context.
type RenderData struct {
	Module       string
	ServiceName  string
	ServiceInfo  ServiceInfo
	WithDatabase bool
	Infra        []string
}

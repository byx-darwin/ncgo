// Package protolint provides the proto parsing/model foundation for future
// Proto I/O lint rules.
package protolint

// BindingKind is the normalized request-binding category used by Hertz-style
// field options.
type BindingKind string

const (
	BindingUnknown BindingKind = "unknown"
	BindingQuery   BindingKind = "query"
	BindingPath    BindingKind = "path"
	BindingHeader  BindingKind = "header"
	BindingCookie  BindingKind = "cookie"
	BindingBody    BindingKind = "body"
	BindingRawBody BindingKind = "raw_body"
	BindingForm    BindingKind = "form"
)

// Location is a 1-based source location in a .proto file.
type Location struct {
	Line   int
	Column int
}

// Binding describes one recognized field binding annotation.
type Binding struct {
	Kind       BindingKind
	Annotation string
	Value      string
}

// HTTPRule describes one recognized Hertz method option on an RPC.
type HTTPRule struct {
	Annotation string
	Method     string
	Path       string
}

// Field is the normalized view of a protobuf field.
type Field struct {
	Name                  string
	Number                int32
	TypeName              string
	ParentMessage         string
	Bindings              []Binding
	HasOpenAPIProperty    bool
	HasPGVRangeConstraint bool
	// IsString is true when the field is a scalar string type (not bytes).
	IsString bool
	// IsRepeated is true when the field is a repeated non-map field.
	IsRepeated bool
	// IsMap is true when the field is a map field.
	IsMap bool
	// IsEnum is true when the field is an enum-typed field.
	IsEnum bool
	// HasPGVLenConstraint is true when the field's validate.rules contain an
	// explicit string-length or bytes-length constraint
	// (min_len / max_len / len / min_bytes / max_bytes / len_bytes / pattern).
	HasPGVLenConstraint bool
	// HasPGVItemsConstraint is true when the field's validate.rules contain an
	// explicit item- or pair-count constraint
	// (min_items / max_items / min_pairs / max_pairs).
	HasPGVItemsConstraint bool
	// HasPGVDefinedOnly is true when the field's validate.rules contain a
	// defined_only enum constraint.
	HasPGVDefinedOnly bool
	Location          Location
}

// Message is the normalized view of a protobuf message.
type Message struct {
	Name               string
	FullName           string
	File               string
	Fields             []*Field
	TopLevelFieldNames []string
	HasOpenAPISchema   bool
	Location           Location
}

// RPC is the normalized view of a protobuf rpc method.
type RPC struct {
	Name                string
	FullName            string
	File                string
	ServiceName         string
	InputMessageName    string
	OutputMessageName   string
	InputMessage        *Message
	OutputMessage       *Message
	HTTPRules           []HTTPRule
	HTTPMethod          string
	HTTPPath            string
	PathParams          []string
	HasOpenAPIOperation bool
	IsHertz             bool
	Location            Location
}

// Service is the normalized view of a protobuf service.
type Service struct {
	Name     string
	FullName string
	File     string
	RPCs     []*RPC
	Location Location
}

// File is the normalized view of one parsed .proto file.
type File struct {
	Path      string
	Package   string
	GoPackage string // raw go_package option value (e.g. "a/b/c;pkg"), empty if unset
	Syntax    string
	Imports   []string
	IsHertz   bool
	Services  []*Service
	Messages  []*Message
}

// Model is the aggregate normalized view of the requested entry files.
type Model struct {
	Files []*File
}

// RPCs returns every RPC from every parsed file, in file/service declaration
// order.
func (m *Model) RPCs() []*RPC {
	if m == nil {
		return nil
	}
	var out []*RPC
	for _, f := range m.Files {
		for _, s := range f.Services {
			out = append(out, s.RPCs...)
		}
	}
	return out
}

// LoadOptions configures proto loading.
type LoadOptions struct {
	// Root is the import root that entry files are resolved from.
	Root string
	// Files are the entry .proto files relative to Root.
	Files []string
}

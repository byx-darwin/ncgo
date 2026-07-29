package protolint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var errEmptyRoot = errors.New("protolint: Root is required")
var errEmptyFiles = errors.New("protolint: at least one proto file is required")

var httpMethodOptions = map[protoreflect.FullName]string{
	"api.get":     "GET",
	"api.post":    "POST",
	"api.put":     "PUT",
	"api.patch":   "PATCH",
	"api.delete":  "DELETE",
	"api.options": "OPTIONS",
	"api.head":    "HEAD",
}

var fieldBindingOptions = map[protoreflect.FullName]BindingKind{
	"api.query":    BindingQuery,
	"api.path":     BindingPath,
	"api.header":   BindingHeader,
	"api.cookie":   BindingCookie,
	"api.body":     BindingBody,
	"api.raw_body": BindingRawBody,
	"api.form":     BindingForm,
}

const (
	openAPIOperationOption protoreflect.FullName = "openapi.operation"
	openAPISchemaOption    protoreflect.FullName = "openapi.schema"
	openAPIPropertyOption  protoreflect.FullName = "openapi.property"
	validateRulesOption    protoreflect.FullName = "validate.rules"
)

// importRoots returns the proto import roots: the project root plus the
// scaffold's idl directory when present. Generated protos import support
// files relative to idl/ (hz convention: compile with -I idl) — e.g.
// idl/app/demo.proto imports "api.proto" which lives at idl/api.proto.
func importRoots(root string) []string {
	roots := []string{root}
	if fi, err := os.Stat(filepath.Join(root, "idl")); err == nil && fi.IsDir() {
		roots = append(roots, filepath.Join(root, "idl"))
	}
	return roots
}

// Load compiles and normalizes the requested proto entry files.
func Load(ctx context.Context, opts LoadOptions) (*Model, error) {
	if opts.Root == "" {
		return nil, errEmptyRoot
	}
	if len(opts.Files) == 0 {
		return nil, errEmptyFiles
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("protolint: resolve root: %w", err)
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: importRoots(root),
		}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	files, err := compiler.Compile(ctx, opts.Files...)
	if err != nil {
		return nil, fmt.Errorf("protolint: compile protos: %w", err)
	}

	b := builder{messages: map[protoreflect.FullName]*Message{}}
	model := &Model{Files: make([]*File, 0, len(files))}
	for _, fd := range files {
		model.Files = append(model.Files, b.buildFile(fd))
	}
	return model, nil
}

type builder struct {
	messages map[protoreflect.FullName]*Message
}

func (b *builder) buildFile(fd protoreflect.FileDescriptor) *File {
	file := &File{
		Path:    fd.Path(),
		Package: string(fd.Package()),
		Syntax:  fd.Syntax().String(),
	}
	for i := 0; i < fd.Imports().Len(); i++ {
		file.Imports = append(file.Imports, fd.Imports().Get(i).Path())
	}
	file.IsHertz = hasImport(file.Imports, "api.proto")
	for i := 0; i < fd.Services().Len(); i++ {
		file.Services = append(file.Services, b.buildService(fd.Services().Get(i), file))
	}
	for i := 0; i < fd.Messages().Len(); i++ {
		file.Messages = append(file.Messages, b.buildMessage(fd.Messages().Get(i)))
	}
	return file
}

func (b *builder) buildService(sd protoreflect.ServiceDescriptor, file *File) *Service {
	service := &Service{
		Name:     string(sd.Name()),
		FullName: string(sd.FullName()),
		File:     sd.ParentFile().Path(),
		Location: locationFor(sd),
	}
	for i := 0; i < sd.Methods().Len(); i++ {
		service.RPCs = append(service.RPCs, b.buildRPC(sd.Methods().Get(i), file))
	}
	return service
}

func (b *builder) buildRPC(md protoreflect.MethodDescriptor, file *File) *RPC {
	rules := extractHTTPRules(md)
	rpc := &RPC{
		Name:                string(md.Name()),
		FullName:            string(md.FullName()),
		File:                md.ParentFile().Path(),
		ServiceName:         string(md.Parent().Name()),
		InputMessageName:    string(md.Input().Name()),
		OutputMessageName:   string(md.Output().Name()),
		InputMessage:        b.buildMessage(md.Input()),
		OutputMessage:       b.buildMessage(md.Output()),
		HTTPRules:           rules,
		IsHertz:             file != nil && file.IsHertz,
		HasOpenAPIOperation: hasOption(md.Options().ProtoReflect(), openAPIOperationOption),
		Location:            locationFor(md),
	}
	if len(rules) == 1 {
		rpc.HTTPMethod = rules[0].Method
		rpc.HTTPPath = rules[0].Path
		rpc.PathParams = extractPathParams(rules[0].Path)
	}
	return rpc
}

func (b *builder) buildMessage(md protoreflect.MessageDescriptor) *Message {
	if m, ok := b.messages[md.FullName()]; ok {
		return m
	}
	msg := &Message{
		Name:             string(md.Name()),
		FullName:         string(md.FullName()),
		File:             md.ParentFile().Path(),
		HasOpenAPISchema: hasOption(md.Options().ProtoReflect(), openAPISchemaOption),
		Location:         locationFor(md),
	}
	b.messages[md.FullName()] = msg
	for i := 0; i < md.Fields().Len(); i++ {
		f := b.buildField(md.Fields().Get(i), md)
		msg.Fields = append(msg.Fields, f)
		msg.TopLevelFieldNames = append(msg.TopLevelFieldNames, f.Name)
	}
	return msg
}

func (b *builder) buildField(fd protoreflect.FieldDescriptor, parent protoreflect.MessageDescriptor) *Field {
	opts := fd.Options().ProtoReflect()
	return &Field{
		Name:                  string(fd.Name()),
		Number:                int32(fd.Number()),
		TypeName:              typeNameFor(fd),
		ParentMessage:         string(parent.FullName()),
		Bindings:              extractBindings(fd),
		HasOpenAPIProperty:    hasOption(opts, openAPIPropertyOption),
		HasPGVRangeConstraint: hasPGVRangeConstraint(opts),
		IsString:              fd.Kind() == protoreflect.StringKind,
		IsRepeated:            fd.IsList(),
		IsMap:                 fd.IsMap(),
		IsEnum:                fd.Kind() == protoreflect.EnumKind,
		HasPGVLenConstraint:   hasPGVConstraintFields(opts, "min_len", "max_len", "len", "len_bytes", "min_bytes", "max_bytes", "pattern"),
		HasPGVItemsConstraint: hasPGVConstraintFields(opts, "min_items", "max_items", "min_pairs", "max_pairs"),
		HasPGVDefinedOnly:     hasPGVConstraintFields(opts, "defined_only"),
		Location:              locationFor(fd),
	}
}

func hasPGVRangeConstraint(msg protoreflect.Message) bool {
	if !msg.IsValid() {
		return false
	}
	found := false
	msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.FullName() != validateRulesOption {
			return true
		}
		if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
			found = messageHasRangeConstraint(v.Message())
		}
		return !found
	})
	return found
}

func messageHasRangeConstraint(msg protoreflect.Message) bool {
	if !msg.IsValid() {
		return false
	}
	found := false
	msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch string(fd.Name()) {
		case "gt", "gte", "lt", "lte":
			found = true
			return false
		}
		if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
			if messageHasRangeConstraint(v.Message()) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// hasPGVConstraintFields returns true when the field options message contains a
// validate.rules option whose nested messages have at least one field whose
// name matches any of the provided names (searched recursively).
func hasPGVConstraintFields(opts protoreflect.Message, names ...string) bool {
	if !opts.IsValid() {
		return false
	}
	found := false
	opts.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.FullName() != validateRulesOption {
			return true
		}
		if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
			found = messageHasNamedField(v.Message(), names)
		}
		return !found
	})
	return found
}

// messageHasNamedField recursively searches msg for any set field whose name
// is in names, descending into nested messages.
func messageHasNamedField(msg protoreflect.Message, names []string) bool {
	if !msg.IsValid() {
		return false
	}
	found := false
	msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		for _, name := range names {
			if string(fd.Name()) == name {
				found = true
				return false
			}
		}
		if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
			if messageHasNamedField(v.Message(), names) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func hasOption(msg protoreflect.Message, want protoreflect.FullName) bool {
	if !msg.IsValid() {
		return false
	}
	found := false
	msg.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		if fd.FullName() == want {
			found = true
			return false
		}
		return true
	})
	return found
}

func extractHTTPRules(md protoreflect.MethodDescriptor) []HTTPRule {
	var rules []HTTPRule
	md.Options().ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		method, ok := httpMethodOptions[fd.FullName()]
		if !ok {
			return true
		}
		rules = append(rules, HTTPRule{
			Annotation: string(fd.FullName()),
			Method:     method,
			Path:       v.String(),
		})
		return true
	})
	return rules
}

func extractBindings(fd protoreflect.FieldDescriptor) []Binding {
	var bindings []Binding
	fd.Options().ProtoReflect().Range(func(opt protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		kind, ok := fieldBindingOptions[opt.FullName()]
		if !ok {
			return true
		}
		bindings = append(bindings, Binding{
			Kind:       kind,
			Annotation: string(opt.FullName()),
			Value:      valueString(v),
		})
		return true
	})
	return bindings
}

func typeNameFor(fd protoreflect.FieldDescriptor) string {
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		if md := fd.Message(); md != nil {
			return string(md.FullName())
		}
	case protoreflect.EnumKind:
		if ed := fd.Enum(); ed != nil {
			return string(ed.FullName())
		}
	}
	return fd.Kind().String()
}

func locationFor(d protoreflect.Descriptor) Location {
	loc := d.ParentFile().SourceLocations().ByDescriptor(d)
	if loc.StartLine <= 0 && loc.StartColumn <= 0 {
		return Location{}
	}
	return Location{
		Line:   loc.StartLine + 1,
		Column: loc.StartColumn + 1,
	}
}

func extractPathParams(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	params := make([]string, 0, len(parts))
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, ":") && len(part) > 1:
			params = append(params, strings.TrimPrefix(part, ":"))
		case strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2:
			params = append(params, strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}"))
		}
	}
	return params
}

func valueString(v protoreflect.Value) string {
	if s, ok := v.Interface().(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprint(v.Interface())
}

func hasImport(imports []string, want string) bool {
	for _, imp := range imports {
		if imp == want {
			return true
		}
	}
	return false
}

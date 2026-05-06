package protolint

import (
	"fmt"
	"sort"
	"strings"
)

type rulePIO103 struct{}

func (rulePIO103) Meta() RuleMeta {
	return RuleMeta{ID: "PIO103", Level: LevelError, Phase: Phase1}
}

func (r rulePIO103) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if isDisallowedDynamicTopLevel(rpc.InputMessage) {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s top-level input must not use %s", rpc.Name, rpc.InputMessage.FullName),
				"define a method-specific request message instead of using Any/Struct/Value as the RPC input",
			)
			d.Message = rpc.InputMessage.FullName
			out = append(out, d)
		}
		if isDisallowedDynamicTopLevel(rpc.OutputMessage) {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s top-level output must not use %s", rpc.Name, rpc.OutputMessage.FullName),
				"define a method-specific response message instead of using Any/Struct/Value as the RPC output",
			)
			d.Message = rpc.OutputMessage.FullName
			out = append(out, d)
		}
	}
	return out
}

type rulePIO201 struct{}

func (rulePIO201) Meta() RuleMeta {
	return RuleMeta{ID: "PIO201", Level: LevelError, Phase: Phase1}
}

func (r rulePIO201) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if !rpc.IsHertz {
			continue
		}
		if len(rpc.HTTPRules) == 1 {
			continue
		}
		out = append(out, rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("rpc %s must declare exactly one HTTP method annotation", rpc.Name),
			"declare exactly one of api.get/api.post/api.put/api.patch/api.delete/api.options/api.head on the RPC",
		))
	}
	return out
}

type rulePIO202 struct{}

func (rulePIO202) Meta() RuleMeta {
	return RuleMeta{ID: "PIO202", Level: LevelError, Phase: Phase1}
}

func (r rulePIO202) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if !rpc.IsHertz {
			continue
		}
		pathParams := sliceToSet(rpc.PathParams)
		requestPathFields := requestBindingValues(rpc.InputMessage, BindingPath)
		if sameStringSet(pathParams, requestPathFields) {
			continue
		}
		out = append(out, rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("rpc %s path params do not match request api.path fields", rpc.Name),
			"make the HTTP path parameters and request api.path bindings match exactly",
		))
	}
	return out
}

type rulePIO203 struct{}

func (rulePIO203) Meta() RuleMeta {
	return RuleMeta{ID: "PIO203", Level: LevelError, Phase: Phase1}
}

func (r rulePIO203) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if !rpc.IsHertz || !isBodyDisallowedMethod(rpc.HTTPMethod) {
			continue
		}
		for _, field := range requestFieldsWithBinding(rpc.InputMessage, BindingBody, BindingRawBody) {
			if hasBinding(field, BindingBody) {
				d := rpcDiagnostic(
					meta,
					rpc,
					fmt.Sprintf("rpc %s uses api.body on %s request", rpc.Name, rpc.HTTPMethod),
					"remove api.body from GET/DELETE/HEAD request fields",
				)
				d.Field = field.Name
				out = append(out, d)
			}
			if hasBinding(field, BindingRawBody) {
				d := rpcDiagnostic(
					meta,
					rpc,
					fmt.Sprintf("rpc %s uses api.raw_body on %s request", rpc.Name, rpc.HTTPMethod),
					"remove api.raw_body from GET/DELETE/HEAD request fields",
				)
				d.Field = field.Name
				out = append(out, d)
			}
		}
	}
	return out
}

type rulePIO211 struct{}

func (rulePIO211) Meta() RuleMeta {
	return RuleMeta{ID: "PIO211", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO211) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if !rpc.IsHertz || rpc.InputMessage == nil {
			continue
		}
		for _, field := range rpc.InputMessage.Fields {
			if len(field.Bindings) != 0 {
				continue
			}
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("request field %s does not declare any binding annotation", field.Name),
				"declare one of api.query/api.path/api.header/api.cookie/api.body/api.raw_body/api.form on Hertz request fields that should be bound from HTTP input",
			)
			d.Message = rpc.InputMessage.Name
			d.Field = field.Name
			out = append(out, d)
		}
	}
	return out
}

type rulePIO212 struct{}

func (rulePIO212) Meta() RuleMeta {
	return RuleMeta{ID: "PIO212", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO212) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if !rpc.IsHertz {
			continue
		}
		if !rpc.HasOpenAPIOperation {
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("rpc %s is missing openapi.operation metadata", rpc.Name),
				"declare openapi.operation on Hertz RPCs so the generated API surface carries summary/description metadata",
			)
			d.Message = rpc.Name
			out = append(out, d)
		}
		out = append(out, missingOpenAPISchemaAndPropertyDiagnostics(meta, rpc, rpc.InputMessage)...)
		out = append(out, missingOpenAPISchemaAndPropertyDiagnostics(meta, rpc, rpc.OutputMessage)...)
	}
	return out
}

func missingOpenAPISchemaAndPropertyDiagnostics(meta RuleMeta, rpc *RPC, msg *Message) []Diagnostic {
	if msg == nil {
		return nil
	}
	var out []Diagnostic
	if shouldRequireOpenAPISchema(msg) && !msg.HasOpenAPISchema {
		d := rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("message %s is missing openapi.schema metadata", msg.Name),
			"declare openapi.schema on Hertz top-level response messages so the generated schema has stable title/description metadata",
		)
		d.Message = msg.Name
		out = append(out, d)
	}
	for _, field := range msg.Fields {
		if field.HasOpenAPIProperty {
			continue
		}
		d := rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("field %s is missing openapi.property metadata", field.Name),
			"declare openapi.property on Hertz top-level request/response fields so generated OpenAPI docs retain field titles/descriptions and schema hints",
		)
		d.Message = msg.Name
		d.Field = field.Name
		out = append(out, d)
	}
	return out
}

func shouldRequireOpenAPISchema(msg *Message) bool {
	if msg == nil {
		return false
	}
	return strings.HasSuffix(msg.Name, "Resp")
}

type rulePIO204 struct{}

func (rulePIO204) Meta() RuleMeta {
	return RuleMeta{ID: "PIO204", Level: LevelError, Phase: Phase1}
}

func (r rulePIO204) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if !rpc.IsHertz || rpc.InputMessage == nil {
			continue
		}
		for _, field := range rpc.InputMessage.Fields {
			if len(field.Bindings) <= 1 {
				continue
			}
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("field %s declares multiple bindings: %s", field.Name, strings.Join(bindingAnnotations(field), ", ")),
				"keep exactly one request binding annotation on the field",
			)
			d.Field = field.Name
			out = append(out, d)
		}
	}
	return out
}

type rulePIO205 struct{}

func (rulePIO205) Meta() RuleMeta {
	return RuleMeta{ID: "PIO205", Level: LevelError, Phase: Phase1}
}

func (r rulePIO205) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if !rpc.IsHertz || rpc.InputMessage == nil {
			continue
		}
		rawBodyFields := requestFieldsWithBinding(rpc.InputMessage, BindingRawBody)
		if len(rawBodyFields) <= 1 {
			continue
		}
		d := rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("message %s declares more than one raw_body field", rpc.InputMessage.Name),
			"keep at most one api.raw_body field in a Hertz request message",
		)
		d.Message = rpc.InputMessage.Name
		out = append(out, d)
	}
	return out
}

type rulePIO206 struct{}

func (rulePIO206) Meta() RuleMeta {
	return RuleMeta{ID: "PIO206", Level: LevelError, Phase: Phase1}
}

func (r rulePIO206) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if !rpc.IsHertz || rpc.OutputMessage == nil {
			continue
		}
		for _, field := range rpc.OutputMessage.Fields {
			for _, binding := range field.Bindings {
				d := rpcDiagnostic(
					meta,
					rpc,
					fmt.Sprintf("response field %s must not use request binding annotation %s", field.Name, binding.Annotation),
					"remove request-side binding annotations from response message fields",
				)
				d.Message = rpc.OutputMessage.Name
				d.Field = field.Name
				out = append(out, d)
			}
		}
	}
	return out
}

type rulePIO301 struct{}

func (rulePIO301) Meta() RuleMeta {
	return RuleMeta{ID: "PIO301", Level: LevelError, Phase: Phase1}
}

func (r rulePIO301) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if rpc.IsHertz || rpc.OutputMessage == nil {
			continue
		}
		hits := transportEnvelopeFieldHits(rpc.OutputMessage)
		if len(hits) < 2 {
			continue
		}
		d := rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("response %s looks like transport envelope with fields %s", rpc.OutputMessage.Name, strings.Join(hits, ",")),
			"move transport envelope fields out of the Kitex response message and keep only business data in the RPC output",
		)
		d.Message = rpc.OutputMessage.Name
		out = append(out, d)
	}
	return out
}

type rulePIO302 struct{}

func (rulePIO302) Meta() RuleMeta {
	return RuleMeta{ID: "PIO302", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO302) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if rpc.IsHertz || rpc.InputMessage == nil || !looksLikeListSearchQueryRPC(rpc.Name) {
			continue
		}
		if hasPaginationField(rpc.InputMessage) {
			continue
		}
		d := rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("rpc %s looks like a list/search/query method but request %s has no pagination or cursor fields", rpc.Name, rpc.InputMessage.Name),
			"consider adding page/page_size/limit/cursor-style request fields so list/search/query RPCs have an explicit pagination contract",
		)
		d.Message = rpc.InputMessage.Name
		out = append(out, d)
	}
	return out
}

type rulePIO303 struct{}

func (rulePIO303) Meta() RuleMeta {
	return RuleMeta{ID: "PIO303", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO303) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if rpc.IsHertz || rpc.InputMessage == nil {
			continue
		}
		categories, matchedFields := classifyUniversalRequest(rpc.InputMessage)
		if len(categories) < 3 || matchedFields < 4 {
			continue
		}
		d := rpcDiagnostic(
			meta,
			rpc,
			fmt.Sprintf("request %s mixes too many concerns (%s) and looks like a universal request object", rpc.InputMessage.Name, strings.Join(sortedKeys(categories), ",")),
			"consider splitting filtering, sorting, pagination, debug, or extension controls into narrower method-specific requests",
		)
		d.Message = rpc.InputMessage.Name
		out = append(out, d)
	}
	return out
}

type rulePIO401 struct{}

func (rulePIO401) Meta() RuleMeta {
	return RuleMeta{ID: "PIO401", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO401) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if rpc.InputMessage == nil {
			continue
		}
		for _, field := range rpc.InputMessage.Fields {
			if !isPGVPaginationFieldName(field.Name) || field.HasPGVRangeConstraint {
				continue
			}
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("request field %s looks pagination-related but has no obvious PGV range constraint", field.Name),
				"declare validate.rules with gte/gt/lt/lte style bounds on pagination fields such as page/page_size/limit/offset",
			)
			d.Message = rpc.InputMessage.Name
			d.Field = field.Name
			out = append(out, d)
		}
	}
	return out
}

func isDisallowedDynamicTopLevel(msg *Message) bool {
	if msg == nil {
		return false
	}
	switch msg.FullName {
	case "google.protobuf.Any", "google.protobuf.Struct", "google.protobuf.Value":
		return true
	default:
		return false
	}
}

func looksLikeListSearchQueryRPC(name string) bool {
	return strings.HasPrefix(name, "List") || strings.HasPrefix(name, "Search") || strings.HasPrefix(name, "Query")
}

func hasPaginationField(msg *Message) bool {
	if msg == nil {
		return false
	}
	for _, field := range msg.Fields {
		if isPaginationFieldName(field.Name) {
			return true
		}
	}
	return false
}

func isPaginationFieldName(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "_", ""))
	switch normalized {
	case "page", "pagesize", "limit", "cursor", "offset":
		return true
	default:
		return false
	}
}

func isPGVPaginationFieldName(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "_", ""))
	switch normalized {
	case "page", "pagesize", "limit", "offset":
		return true
	default:
		return false
	}
}

func classifyUniversalRequest(msg *Message) (map[string]struct{}, int) {
	categories := make(map[string]struct{})
	matchedFields := 0
	if msg == nil {
		return categories, matchedFields
	}
	for _, field := range msg.Fields {
		cats := fieldConcernCategories(field.Name)
		if len(cats) == 0 {
			continue
		}
		matchedFields++
		for _, cat := range cats {
			categories[cat] = struct{}{}
		}
	}
	return categories, matchedFields
}

func fieldConcernCategories(name string) []string {
	normalized := normalizeFieldName(name)
	var out []string
	if isOneOf(normalized, "filter", "filters", "query", "keyword", "keywords", "status", "statuses", "type", "types", "category", "categories", "tag", "tags") {
		out = append(out, "filter")
	}
	if isOneOf(normalized, "sort", "sortby", "orderby", "order", "direction", "asc", "desc") {
		out = append(out, "sort")
	}
	if isOneOf(normalized, "page", "pagesize", "limit", "cursor", "offset") {
		out = append(out, "pagination")
	}
	if isOneOf(normalized, "debug", "verbose", "trace", "dryrun", "explaindebug") {
		out = append(out, "debug")
	}
	if isOneOf(normalized, "extra", "extras", "ext", "extension", "extensions", "option", "options", "opt", "opts", "meta", "metadata") {
		out = append(out, "extension")
	}
	return out
}

func normalizeFieldName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

func isOneOf(value string, wants ...string) bool {
	for _, want := range wants {
		if value == want {
			return true
		}
	}
	return false
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func requestBindingValues(msg *Message, kind BindingKind) map[string]struct{} {
	values := make(map[string]struct{})
	if msg == nil {
		return values
	}
	for _, field := range msg.Fields {
		for _, binding := range field.Bindings {
			if binding.Kind == kind {
				values[binding.Value] = struct{}{}
			}
		}
	}
	return values
}

func requestFieldsWithBinding(msg *Message, kinds ...BindingKind) []*Field {
	if msg == nil {
		return nil
	}
	allowed := make(map[BindingKind]struct{}, len(kinds))
	for _, kind := range kinds {
		allowed[kind] = struct{}{}
	}
	var out []*Field
	for _, field := range msg.Fields {
		for _, binding := range field.Bindings {
			if _, ok := allowed[binding.Kind]; ok {
				out = append(out, field)
				break
			}
		}
	}
	return out
}

func hasBinding(field *Field, kind BindingKind) bool {
	if field == nil {
		return false
	}
	for _, binding := range field.Bindings {
		if binding.Kind == kind {
			return true
		}
	}
	return false
}

func isBodyDisallowedMethod(method string) bool {
	switch method {
	case "GET", "DELETE", "HEAD":
		return true
	default:
		return false
	}
}

func sliceToSet(xs []string) map[string]struct{} {
	set := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		set[x] = struct{}{}
	}
	return set
}

func sameStringSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for x := range a {
		if _, ok := b[x]; !ok {
			return false
		}
	}
	return true
}

func bindingAnnotations(field *Field) []string {
	if field == nil {
		return nil
	}
	out := make([]string, 0, len(field.Bindings))
	for _, binding := range field.Bindings {
		out = append(out, binding.Annotation)
	}
	return out
}

func transportEnvelopeFieldHits(msg *Message) []string {
	if msg == nil {
		return nil
	}
	want := map[string]struct{}{
		"code":    {},
		"msg":     {},
		"success": {},
		"error":   {},
	}
	var hits []string
	for _, name := range msg.TopLevelFieldNames {
		if _, ok := want[strings.ToLower(name)]; ok {
			hits = append(hits, name)
		}
	}
	return hits
}

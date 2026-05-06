package protolint

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strings"
)

const sarifSchemaURL = "https://json.schemastore.org/sarif-2.1.0.json"

const (
	protolintSARIFTaxonomyName      = "ncgo protolint taxonomy"
	protolintTaxonCommonContract    = "common-contract"
	protolintTaxonHertzHTTPBinding  = "hertz-http-binding"
	protolintTaxonKitexRPCContract  = "kitex-rpc-contract"
	protolintTaxonPGVConstraints    = "pgv-field-constraints"
	protolintTaxonHeuristicGuidance = "heuristic-guidance"
	protolintHelpBaseURI            = "https://github.com/byx-darwin/ncgo/blob/main"
	protolintRulesHelpURI           = protolintHelpBaseURI + "/docs/proto-io-lint-rules.zh-CN.md"
)

type protolintSARIFRuleMeta struct {
	ID           string
	Name         string
	Short        string
	Full         string
	Help         string
	HelpURI      string
	DefaultLevel string
	Tags         []string
	Taxa         []string
	Properties   map[string]any
}

// WriteSARIF writes a SARIF 2.1.0 report for the lint result.
func WriteSARIF(w io.Writer, res *Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(buildSARIF(res))
}

func buildSARIF(res *Result) map[string]any {
	return map[string]any{
		"$schema": sarifSchemaURL,
		"version": "2.1.0",
		"runs":    []any{buildSARIFRun(res)},
	}
}

func buildSARIFRun(res *Result) map[string]any {
	if res == nil {
		res = &Result{}
	}
	return map[string]any{
		"tool": map[string]any{
			"driver": map[string]any{
				"name":           "ncgo protolint",
				"informationUri": "https://github.com/byx-darwin/ncgo",
				"rules":          buildSARIFRules(res.RulesRun),
			},
		},
		"taxonomies": []any{buildSARIFTaxonomy()},
		"results":    buildSARIFResults(res.Diagnostics),
		"properties": map[string]any{
			"root":         res.Root,
			"files":        res.Files,
			"rulesRun":     res.RulesRun,
			"ignoredRules": res.IgnoredRules,
			"ignoredFiles": res.IgnoredFiles,
			"summary":      res.Summary,
			"ok":           res.OK,
		},
	}
}

func buildSARIFRules(ruleIDs []string) []any {
	out := make([]any, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		out = append(out, buildSARIFRuleDescriptor(sarifRuleMetaByID(id)))
	}
	return out
}

func buildSARIFResults(diags []Diagnostic) []any {
	out := make([]any, 0, len(diags))
	for _, d := range diags {
		meta := sarifRuleMetaByID(d.RuleID)
		result := map[string]any{
			"ruleId":  d.RuleID,
			"level":   sarifLevel(d.Level),
			"message": map[string]any{"text": d.Summary},
			"properties": map[string]any{
				"tags":    meta.Tags,
				"taxa":    meta.Taxa,
				"phase":   d.Phase,
				"service": d.Service,
				"rpc":     d.RPC,
				"message": d.Message,
				"field":   d.Field,
				"hint":    d.Hint,
			},
		}
		if len(meta.Taxa) > 0 {
			result["taxa"] = buildSARIFTaxaReferences(meta.Taxa)
		}
		if d.File != "" {
			location := map[string]any{
				"physicalLocation": map[string]any{
					"artifactLocation": map[string]any{"uri": d.File},
					"region":           map[string]any{"startLine": d.Line, "startColumn": d.Column},
				},
			}
			result["locations"] = []any{location}
		}
		out = append(out, result)
	}
	return out
}

func buildSARIFRuleDescriptor(meta protolintSARIFRuleMeta) map[string]any {
	rule := map[string]any{
		"id":            meta.ID,
		"name":          meta.Name,
		"properties":    mergeSARIFProperties(meta.Properties, map[string]any{"tags": meta.Tags, "taxa": meta.Taxa}),
		"relationships": buildSARIFTaxonomyRelationships(meta.Taxa),
	}
	if meta.Short != "" {
		rule["shortDescription"] = map[string]any{"text": meta.Short}
	}
	if meta.Full != "" {
		rule["fullDescription"] = map[string]any{"text": meta.Full}
	}
	if meta.Help != "" {
		rule["help"] = map[string]any{"text": meta.Help}
	}
	if meta.HelpURI != "" {
		rule["helpUri"] = meta.HelpURI
	}
	if meta.DefaultLevel != "" {
		rule["defaultConfiguration"] = map[string]any{"level": meta.DefaultLevel}
	}
	return rule
}

func buildSARIFTaxonomy() map[string]any {
	return map[string]any{
		"name":           protolintSARIFTaxonomyName,
		"informationUri": protolintRulesHelpURI,
		"shortDescription": map[string]any{
			"text": "Category taxonomy for ncgo Proto I/O lint rules",
		},
		"taxa": []any{
			buildSARIFTaxon(protolintTaxonCommonContract, "Common Contract", "General request/response naming and contract rules"),
			buildSARIFTaxon(protolintTaxonHertzHTTPBinding, "Hertz HTTP Binding", "Hertz-specific HTTP annotation and binding rules"),
			buildSARIFTaxon(protolintTaxonKitexRPCContract, "Kitex RPC Contract", "Kitex-specific response and request contract rules"),
			buildSARIFTaxon(protolintTaxonPGVConstraints, "PGV Field Constraints", "PGV recommendation rules for request fields"),
			buildSARIFTaxon(protolintTaxonHeuristicGuidance, "Heuristic Guidance", "Heuristic phase2 warnings intended for gradual design guidance"),
		},
	}
}

func buildSARIFTaxon(id, name, description string) map[string]any {
	return map[string]any{
		"id":   id,
		"name": name,
		"shortDescription": map[string]any{
			"text": description,
		},
	}
}

func buildSARIFTaxonomyRelationships(taxa []string) []any {
	if len(taxa) == 0 {
		return nil
	}
	out := make([]any, 0, len(taxa))
	for _, taxon := range taxa {
		out = append(out, map[string]any{
			"target": map[string]any{
				"id":            taxon,
				"toolComponent": map[string]any{"name": protolintSARIFTaxonomyName},
			},
			"kinds": []string{"superset"},
		})
	}
	return out
}

func buildSARIFTaxaReferences(taxa []string) []any {
	if len(taxa) == 0 {
		return nil
	}
	out := make([]any, 0, len(taxa))
	for _, taxon := range taxa {
		out = append(out, map[string]any{
			"id":            taxon,
			"toolComponent": map[string]any{"name": protolintSARIFTaxonomyName},
		})
	}
	return out
}

func mergeSARIFProperties(base map[string]any, extra map[string]any) map[string]any {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make(map[string]any, len(base)+len(extra))
	maps.Copy(out, base)
	for k, v := range extra {
		if v == nil {
			continue
		}
		out[k] = v
	}
	return out
}

func sarifLevel(level Level) string {
	if level == LevelWarning {
		return "warning"
	}
	return "error"
}

func ruleMetaByID(id string) (RuleMeta, bool) {
	for _, rule := range DefaultRules() {
		meta := rule.Meta()
		if meta.ID == id {
			return meta, true
		}
	}
	return RuleMeta{}, false
}

func sarifRuleMetaByID(id string) protolintSARIFRuleMeta {
	meta, ok := ruleMetaByID(id)
	if !ok {
		return protolintSARIFRuleMeta{
			ID:         id,
			Name:       id,
			Short:      id,
			Full:       "Structured Proto I/O rule emitted by ncgo protolint.",
			Help:       "Review the rule documentation and diagnostic hint for remediation details.",
			HelpURI:    protolintRulesHelpURI,
			Tags:       []string{"protolint"},
			Properties: map[string]any{"source": "protolint"},
		}
	}
	taxa := ruleTaxa(meta)
	group := ruleGroup(meta.ID)
	return protolintSARIFRuleMeta{
		ID:           id,
		Name:         ruleName(id),
		Short:        fmt.Sprintf("%s (%s / %s)", ruleName(id), meta.Level, meta.Phase),
		Full:         ruleDescription(id),
		Help:         fmt.Sprintf("See the Proto I/O rule inventory for %s, then rerun `ncgo protolint --root . --rule %s --output json` for focused diagnostics.", id, id),
		HelpURI:      protolintRulesHelpURI,
		DefaultLevel: sarifLevel(meta.Level),
		Tags:         ruleTags(meta, group),
		Taxa:         taxa,
		Properties: map[string]any{
			"source": "protolint",
			"phase":  meta.Phase,
			"level":  meta.Level,
			"group":  group,
		},
	}
}

func ruleTaxa(meta RuleMeta) []string {
	taxa := []string{ruleGroup(meta.ID)}
	if meta.Phase == Phase2 {
		taxa = append(taxa, protolintTaxonHeuristicGuidance)
	}
	return taxa
}

func ruleTags(meta RuleMeta, group string) []string {
	tags := []string{"protolint", "proto", group, string(meta.Phase), string(meta.Level)}
	if meta.Phase == Phase2 {
		tags = append(tags, "heuristic")
	}
	return tags
}

func ruleGroup(id string) string {
	switch {
	case strings.HasPrefix(id, "PIO1"):
		return protolintTaxonCommonContract
	case strings.HasPrefix(id, "PIO2"):
		return protolintTaxonHertzHTTPBinding
	case strings.HasPrefix(id, "PIO3"):
		return protolintTaxonKitexRPCContract
	case strings.HasPrefix(id, "PIO4"):
		return protolintTaxonPGVConstraints
	default:
		return "proto-io"
	}
}

func ruleName(id string) string {
	if name, ok := ruleNames[id]; ok {
		return name
	}
	return id
}

func ruleDescription(id string) string {
	if desc, ok := ruleDescriptions[id]; ok {
		return desc
	}
	return "Proto I/O lint rule implemented by ncgo protolint."
}

var ruleNames = map[string]string{
	"PIO101": "RPC uses method-specific Req/Resp messages",
	"PIO102": "Request/response names match the RPC method",
	"PIO103": "Top-level RPC I/O avoids Any/Struct/Value",
	"PIO111": "Use google.protobuf.Empty sparingly",
	"PIO112": "Avoid generic top-level message names",
	"PIO113": "Request has too many top-level fields",
	"PIO201": "Hertz RPC declares exactly one HTTP method annotation",
	"PIO202": "Hertz path params match api.path fields",
	"PIO203": "Safe HTTP methods do not use api.body/raw_body",
	"PIO204": "Request fields declare at most one binding",
	"PIO205": "Hertz request declares raw_body at most once",
	"PIO206": "Hertz responses avoid request binding annotations",
	"PIO211": "Hertz request fields declare bindings",
	"PIO212": "Hertz RPCs provide OpenAPI metadata",
	"PIO301": "Kitex responses avoid transport envelopes",
	"PIO302": "List/search Kitex RPCs include pagination",
	"PIO303": "Kitex requests avoid over-generic shapes",
	"PIO401": "Pagination fields declare PGV range constraints",
	"PIO402": "Free-text string fields declare PGV length constraints",
	"PIO403": "Repeated/map fields declare PGV size constraints",
	"PIO404": "Enum fields declare PGV defined_only constraints",
}

var ruleDescriptions = map[string]string{
	"PIO101": "Each RPC should use <Method>Req and <Method>Resp message names so method-level request/response models stay explicit and stable.",
	"PIO102": "RPC input and output message names should match the method name exactly to keep the contract easy to navigate and review.",
	"PIO103": "Top-level request and response messages should not directly use google.protobuf.Any, Struct, or Value as transport payloads.",
	"PIO111": "Directly using google.protobuf.Empty at RPC boundaries is allowed but should be treated as a design warning rather than the default pattern.",
	"PIO112": "Top-level generic names such as CommonReq, CommonResp, BaseResp, or Result make contracts harder to understand and evolve.",
	"PIO113": "Requests with too many top-level fields often indicate that the interface has become too broad or mixes multiple responsibilities.",
	"PIO201": "A Hertz RPC should declare exactly one HTTP method annotation so transport semantics stay explicit and unambiguous.",
	"PIO202": "Path parameters extracted from the HTTP route should align strictly with fields annotated by api.path in the request message.",
	"PIO203": "GET, DELETE, and HEAD requests should not declare api.body or api.raw_body because these methods should not depend on request bodies.",
	"PIO204": "A single request field should not mix multiple Hertz binding annotations such as query, path, header, cookie, body, or form.",
	"PIO205": "A Hertz request should contain at most one raw_body field so raw payload ownership stays explicit.",
	"PIO206": "Response messages should not contain request-side binding annotations because those annotations are only meaningful on incoming request fields.",
	"PIO211": "Top-level Hertz request fields should declare a recognized binding annotation so their transport source is explicit.",
	"PIO212": "Hertz RPCs, messages, and top-level fields should carry the expected OpenAPI metadata when the project relies on generated API documentation.",
	"PIO301": "Kitex response messages should not mix in transport envelope fields such as code, msg, success, or error at the top level.",
	"PIO302": "List, Search, or Query style Kitex RPCs should usually expose pagination or cursor fields rather than returning unbounded result sets.",
	"PIO303": "Kitex requests that combine too many filter, sort, pagination, debug, or extension concerns become difficult to validate and maintain.",
	"PIO401": "Pagination-style request fields such as page, page_size, limit, or offset should usually declare obvious PGV numeric range constraints.",
	"PIO402": "Free-text string request fields should usually declare PGV length or pattern constraints to bound payload size and ambiguity.",
	"PIO403": "Repeated and map request fields should usually declare PGV item or pair count constraints to avoid unbounded payload expansion.",
	"PIO404": "Enum request fields should usually declare PGV defined_only so proto3 numeric fallthrough does not admit undefined enum values.",
}

package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/byx-darwin/ncgo/internal/protolint"
)

const sarifSchemaURL = "https://json.schemastore.org/sarif-2.1.0.json"

const (
	doctorSARIFTaxonomyName     = "ncgo doctor taxonomy"
	doctorTaxonTooling          = "tooling"
	doctorTaxonProjectMetadata  = "project-metadata"
	doctorTaxonWorkspace        = "workspace-structure"
	doctorTaxonProtoContract    = "proto-contract"
	doctorTaxonArchitecture     = "architecture-layering"
	doctorHelpBaseURI           = "https://github.com/byx-darwin/ncgo/blob/main"
	doctorProtoRulesHelpURI     = doctorHelpBaseURI + "/docs/proto-io-lint-rules.zh-CN.md"
	doctorLifecycleHelpURI      = doctorHelpBaseURI + "/docs/examples.zh-CN.md"
	doctorArchitectureHelpLabel = "nc-skills-golang/SKILL.md#layer-rules"
)

type doctorSARIFRuleMeta struct {
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

// WriteSARIF writes a SARIF 2.1.0 report for doctor findings.
func WriteSARIF(w io.Writer, rep *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(buildSARIF(rep))
}

func buildSARIF(rep *Report) map[string]any {
	return map[string]any{
		"$schema": sarifSchemaURL,
		"version": "2.1.0",
		"runs":    []any{buildSARIFRun(rep)},
	}
}

func buildSARIFRun(rep *Report) map[string]any {
	if rep == nil {
		rep = &Report{}
	}
	return map[string]any{
		"tool": map[string]any{
			"driver": map[string]any{
				"name":           "ncgo doctor",
				"informationUri": "https://github.com/byx-darwin/ncgo",
				"rules":          buildSARIFRules(rep.Checks),
			},
		},
		"taxonomies": []any{buildSARIFTaxonomy()},
		"results":    buildSARIFResults(rep.Checks),
		"properties": map[string]any{
			"root":    rep.Root,
			"scope":   rep.Scope,
			"summary": rep.Summary,
			"ok":      rep.OK(),
			"checks":  rep.Checks,
		},
	}
}

func buildSARIFRules(checks []Check) []any {
	seen := map[string]Check{}
	for _, c := range checks {
		ruleID := doctorSARIFRuleID(c)
		if _, ok := seen[ruleID]; ok {
			continue
		}
		seen[ruleID] = c
	}
	out := make([]any, 0, len(seen))
	for _, c := range checks {
		ruleID := doctorSARIFRuleID(c)
		sample, ok := seen[ruleID]
		if !ok {
			continue
		}
		delete(seen, ruleID)
		out = append(out, buildSARIFRuleDescriptor(doctorSARIFRuleMetaForCheck(sample)))
	}
	return out
}

func buildSARIFResults(checks []Check) []any {
	out := make([]any, 0, len(checks))
	for _, c := range checks {
		if c.OK {
			continue
		}
		meta := doctorSARIFRuleMetaForCheck(c)
		result := map[string]any{
			"ruleId":  doctorSARIFRuleID(c),
			"level":   doctorSARIFLevel(c.Severity),
			"message": map[string]any{"text": c.Message},
			"properties": map[string]any{
				"checkId":  c.ID,
				"severity": c.Severity,
				"hint":     c.Hint,
				"tags":     meta.Tags,
				"taxa":     meta.Taxa,
			},
		}
		if len(meta.Taxa) > 0 {
			result["taxa"] = buildSARIFTaxaReferences(meta.Taxa)
		}
		if c.Rule != "" && c.Rule != doctorSARIFRuleID(c) {
			result["properties"].(map[string]any)["sourceRule"] = c.Rule
		}
		if c.File != "" {
			physical := map[string]any{
				"artifactLocation": map[string]any{"uri": c.File},
			}
			if c.Line > 0 {
				physical["region"] = map[string]any{"startLine": c.Line}
			}
			result["locations"] = []any{map[string]any{"physicalLocation": physical}}
		}
		out = append(out, result)
	}
	return out
}

func buildSARIFRuleDescriptor(meta doctorSARIFRuleMeta) map[string]any {
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
		"name":           doctorSARIFTaxonomyName,
		"informationUri": doctorHelpBaseURI,
		"shortDescription": map[string]any{
			"text": "Category taxonomy for ncgo doctor checks",
		},
		"taxa": []any{
			buildSARIFTaxon(doctorTaxonTooling, "Tooling", "Host tool availability and version checks"),
			buildSARIFTaxon(doctorTaxonProjectMetadata, "Project Metadata", "Manifest and generated metadata consistency checks"),
			buildSARIFTaxon(doctorTaxonWorkspace, "Workspace Structure", "Micro workspace discovery and aggregation checks"),
			buildSARIFTaxon(doctorTaxonProtoContract, "Proto Contract", "Proto I/O diagnostics surfaced through doctor"),
			buildSARIFTaxon(doctorTaxonArchitecture, "Architecture Layering", "Static layering and dependency direction checks"),
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
				"toolComponent": map[string]any{"name": doctorSARIFTaxonomyName},
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
			"toolComponent": map[string]any{"name": doctorSARIFTaxonomyName},
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

func doctorSARIFRuleMetaForCheck(c Check) doctorSARIFRuleMeta {
	if meta, ok := doctorProtoRuleMetadata(doctorSARIFRuleID(c), c); ok {
		return meta
	}
	if meta, ok := doctorFixedRuleMetadata(c); ok {
		return meta
	}
	return doctorSARIFRuleMeta{
		ID:           doctorSARIFRuleID(c),
		Name:         doctorSARIFRuleID(c),
		Short:        c.ID,
		Full:         "Structured check emitted by ncgo doctor.",
		Help:         doctorNonEmpty(c.Hint, "Review the check message and related file to resolve the failing doctor item."),
		DefaultLevel: doctorSARIFLevel(c.Severity),
		Tags:         []string{"doctor"},
		Properties:   map[string]any{"severity": c.Severity},
	}
}

func doctorProtoRuleMetadata(ruleID string, c Check) (doctorSARIFRuleMeta, bool) {
	meta, ok := doctorProtoRuleMeta(ruleID)
	if !ok {
		return doctorSARIFRuleMeta{}, false
	}
	return doctorSARIFRuleMeta{
		ID:           ruleID,
		Name:         ruleID,
		Short:        fmt.Sprintf("Proto I/O rule %s (%s / %s)", ruleID, meta.Level, meta.Phase),
		Full:         "Proto contract issue surfaced through ncgo doctor while linting entry proto files from a service manifest or workspace.",
		Help:         fmt.Sprintf("Run `ncgo protolint --root . --rule %s --output json` for focused diagnostics and remediation context.", ruleID),
		HelpURI:      doctorProtoRulesHelpURI,
		DefaultLevel: doctorSARIFProtoLevel(meta.Level),
		Tags:         []string{"doctor", "proto", "contract", string(meta.Phase), string(meta.Level)},
		Taxa:         []string{doctorTaxonProtoContract},
		Properties: map[string]any{
			"source":    "protolint",
			"phase":     meta.Phase,
			"lintLevel": meta.Level,
			"checkId":   c.ID,
		},
	}, true
}

func doctorFixedRuleMetadata(c Check) (doctorSARIFRuleMeta, bool) {
	byID := map[string]doctorSARIFRuleMeta{
		"tool.hz": {
			ID:           "tool.hz",
			Name:         "hz tool available",
			Short:        "Checks that hz is on PATH and new enough for ncgo.",
			Full:         "Doctor verifies that the Hertz generator binary is installed and meets the minimum supported version.",
			Help:         "Install or upgrade hz, then rerun ncgo doctor to verify the toolchain.",
			HelpURI:      doctorLifecycleHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "tooling", "hz", "blocking"},
			Taxa:         []string{doctorTaxonTooling},
		},
		"tool.kitex": {
			ID:           "tool.kitex",
			Name:         "kitex tool available",
			Short:        "Checks that kitex is on PATH and new enough for ncgo.",
			Full:         "Doctor verifies that the Kitex generator binary is installed and meets the minimum supported version.",
			Help:         "Install or upgrade kitex, then rerun ncgo doctor to verify the toolchain.",
			HelpURI:      doctorLifecycleHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "tooling", "kitex", "blocking"},
			Taxa:         []string{doctorTaxonTooling},
		},
		"tool.go": {
			ID:           "tool.go",
			Name:         "go toolchain available",
			Short:        "Checks that the Go toolchain is on PATH and new enough for ncgo.",
			Full:         "Doctor verifies that the Go toolchain is installed and meets the minimum supported version required to build ncgo and generated projects.",
			Help:         "Install or upgrade Go, then rerun ncgo doctor to verify the toolchain.",
			HelpURI:      doctorLifecycleHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "tooling", "go", "blocking"},
			Taxa:         []string{doctorTaxonTooling},
		},
		"manifest.load": {
			ID:           "manifest.load",
			Name:         "service manifest loads",
			Short:        "Checks that .ncgo/manifest.yaml exists and can be parsed.",
			Full:         "Doctor expects a valid service manifest at the inspected root before running service-level checks.",
			Help:         "Create the project with ncgo new or move to the correct service root.",
			HelpURI:      doctorLifecycleHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "manifest", "service", "blocking"},
			Taxa:         []string{doctorTaxonProjectMetadata},
		},
		"workspace.load": {
			ID:           "workspace.load",
			Name:         "workspace metadata loads",
			Short:        "Checks that ncgo.workspace exists and can be parsed.",
			Full:         "Doctor expects a valid ncgo.workspace file before running micro-workspace aggregation checks.",
			Help:         "Create the workspace with ncgo new --mode micro or move to the correct workspace root.",
			HelpURI:      doctorLifecycleHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "workspace", "metadata", "blocking"},
			Taxa:         []string{doctorTaxonWorkspace},
		},
		"manifest.data.consistent": {
			ID:           "manifest.data.consistent",
			Name:         "template data matches manifest",
			Short:        "Checks that template/data.json agrees with the manifest source of truth.",
			Full:         "Generated template input should stay consistent with manifest.module, manifest.service.name, and manifest.service.with_database.",
			Help:         "Regenerate template/data.json or hand-edit it to match the manifest.",
			HelpURI:      doctorLifecycleHelpURI,
			DefaultLevel: "warning",
			Tags:         []string{"doctor", "manifest", "generated-data", "drift"},
			Taxa:         []string{doctorTaxonProjectMetadata},
		},
		"protolint": {
			ID:           "protolint",
			Name:         "service proto lint aggregate",
			Short:        "Reports the aggregate service proto lint status.",
			Full:         "Doctor ran ncgo protolint against the service entry proto and found no blocking diagnostics.",
			Help:         "Use ncgo protolint for per-rule diagnostics when proto contract issues are suspected.",
			HelpURI:      doctorProtoRulesHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "proto", "contract"},
			Taxa:         []string{doctorTaxonProtoContract},
		},
		"protolint.load": {
			ID:           "protolint.load",
			Name:         "service proto lint loads",
			Short:        "Checks that the service entry proto can be loaded for linting.",
			Full:         "Doctor could not load or analyze the service entry proto because the file, imports, or syntax were invalid.",
			Help:         "Fix manifest.service.idl, proto imports, or syntax, then rerun doctor.",
			HelpURI:      doctorProtoRulesHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "proto", "load", "blocking"},
			Taxa:         []string{doctorTaxonProtoContract},
		},
		"workspace.protolint": {
			ID:           "workspace.protolint",
			Name:         "workspace proto lint aggregate",
			Short:        "Reports the aggregate workspace proto lint status.",
			Full:         "Doctor ran ncgo protolint across workspace services and found no blocking diagnostics.",
			Help:         "Use ncgo protolint from the workspace root for per-rule diagnostics.",
			HelpURI:      doctorProtoRulesHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "workspace", "proto", "contract"},
			Taxa:         []string{doctorTaxonWorkspace, doctorTaxonProtoContract},
		},
		"workspace.protolint.load": {
			ID:           "workspace.protolint.load",
			Name:         "workspace proto lint loads",
			Short:        "Checks that workspace proto targets can be loaded for linting.",
			Full:         "Doctor could not aggregate or lint proto targets discovered from ncgo.workspace.",
			Help:         "Check ncgo.workspace, service manifests, and service proto imports.",
			HelpURI:      doctorProtoRulesHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "workspace", "proto", "load", "blocking"},
			Taxa:         []string{doctorTaxonWorkspace, doctorTaxonProtoContract},
		},
		"layer.handler.no-repo": {
			ID:           "layer.handler.no-repo",
			Name:         "handlers do not import repositories",
			Short:        "Checks that handler packages do not depend directly on repository packages.",
			Full:         "Handlers should delegate to usecases rather than bypassing the usecase layer and reaching repositories directly.",
			Help:         "Move repository access behind a usecase boundary. Layer rule reference: " + doctorArchitectureHelpLabel,
			DefaultLevel: "warning",
			Tags:         []string{"doctor", "architecture", "layering", "handler", "repository"},
			Taxa:         []string{doctorTaxonArchitecture},
		},
		"layer.handler.no-data": {
			ID:           "layer.handler.no-data",
			Name:         "handlers do not import base/data",
			Short:        "Checks that handler packages do not depend directly on internal/base/data.",
			Full:         "Handlers should not pull infrastructure clients directly; they should call usecases instead.",
			Help:         "Move infrastructure access behind a usecase boundary. Layer rule reference: " + doctorArchitectureHelpLabel,
			DefaultLevel: "warning",
			Tags:         []string{"doctor", "architecture", "layering", "handler", "data"},
			Taxa:         []string{doctorTaxonArchitecture},
		},
		"layer.usecase.no-hertz": {
			ID:           "layer.usecase.no-hertz",
			Name:         "usecases are hertz-free",
			Short:        "Checks that internal/usecase does not import Hertz packages.",
			Full:         "Usecases should stay transport-agnostic and must not depend on Hertz APIs.",
			Help:         "Move transport-specific logic out of usecases. Layer rule reference: " + doctorArchitectureHelpLabel,
			DefaultLevel: "warning",
			Tags:         []string{"doctor", "architecture", "layering", "usecase", "hertz"},
			Taxa:         []string{doctorTaxonArchitecture},
		},
		"layer.usecase.no-kitex": {
			ID:           "layer.usecase.no-kitex",
			Name:         "usecases are kitex-free",
			Short:        "Checks that internal/usecase does not import Kitex packages.",
			Full:         "Usecases should stay transport-agnostic and must not depend on Kitex APIs.",
			Help:         "Move transport-specific logic out of usecases. Layer rule reference: " + doctorArchitectureHelpLabel,
			DefaultLevel: "warning",
			Tags:         []string{"doctor", "architecture", "layering", "usecase", "kitex"},
			Taxa:         []string{doctorTaxonArchitecture},
		},
		"layer.usecase.no-request-context": {
			ID:           "layer.usecase.no-request-context",
			Name:         "usecases do not leak request context",
			Short:        "Checks that usecases do not reference app.RequestContext.",
			Full:         "Usecases should not capture transport request context types in fields or function signatures.",
			Help:         "Pass plain inputs or domain-specific abstractions instead. Layer rule reference: " + doctorArchitectureHelpLabel,
			DefaultLevel: "warning",
			Tags:         []string{"doctor", "architecture", "layering", "usecase", "request-context"},
			Taxa:         []string{doctorTaxonArchitecture},
		},
		"layer.repo.no-sql-string": {
			ID:           "layer.repo.no-sql-string",
			Name:         "repositories avoid raw SQL strings",
			Short:        "Checks that repository code does not embed ad-hoc SQL strings.",
			Full:         "Repository code should rely on generated query packages or shared data access helpers instead of inline SQL strings.",
			Help:         "Move raw SQL to generated or centralized data access layers. Layer rule reference: " + doctorArchitectureHelpLabel,
			DefaultLevel: "warning",
			Tags:         []string{"doctor", "architecture", "layering", "repository", "sql"},
			Taxa:         []string{doctorTaxonArchitecture},
		},
		"layer.repo.no-usecase": {
			ID:           "layer.repo.no-usecase",
			Name:         "repositories do not import usecases",
			Short:        "Checks that repository implementations do not depend back on usecases.",
			Full:         "Repository implementations must not depend on the layer above them; wiring should happen from the composition root.",
			Help:         "Invert the dependency or move abstractions so repository code does not import usecases. Layer rule reference: " + doctorArchitectureHelpLabel,
			DefaultLevel: "warning",
			Tags:         []string{"doctor", "architecture", "layering", "repository", "usecase"},
			Taxa:         []string{doctorTaxonArchitecture},
		},
	}
	meta, ok := byID[doctorSARIFRuleID(c)]
	if !ok {
		return doctorSARIFRuleMeta{}, false
	}
	meta.Properties = mergeSARIFProperties(meta.Properties, map[string]any{"severity": c.Severity})
	if c.Rule != "" && c.Rule != meta.ID {
		meta.Properties["sourceRule"] = c.Rule
	}
	return meta, true
}

func doctorSARIFProtoLevel(level protolint.Level) string {
	if level == protolint.LevelWarning {
		return "warning"
	}
	return "error"
}

func doctorNonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func doctorSARIFRuleID(c Check) string {
	if strings.HasPrefix(c.Rule, "PIO") {
		return c.Rule
	}
	return c.ID
}

func doctorSARIFLevel(sev Severity) string {
	if sev == SeverityWarn {
		return "warning"
	}
	return "error"
}

func doctorProtoRuleMeta(id string) (protolint.RuleMeta, bool) {
	for _, rule := range protolint.DefaultRules() {
		meta := rule.Meta()
		if meta.ID == id {
			return meta, true
		}
	}
	return protolint.RuleMeta{}, false
}

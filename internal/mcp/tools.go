package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/doctor"
	"github.com/byx-darwin/ncgo/internal/protolint"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/method"
)

const (
	mcpI18NReportJSONPath   = "internal/pkg/i18n/.meta/report.json"
	mcpI18NReportMDPath     = "internal/pkg/i18n/.meta/report.md"
	mcpI18NReportSchemaID   = "ncgo://schemas/i18n/report-input-v1"
	mcpI18NReportSchemaPath = "schemas/i18n/report-input-v1.schema.json"
	mcpI18NCheckDev         = "dev"
	mcpI18NCheckRelease     = "release"
)

type mcpI18NReportSummary struct {
	LocaleCount              int `json:"locale_count"`
	MessageKeyCount          int `json:"message_key_count"`
	MissingSourceCount       int `json:"missing_source_count"`
	MissingTranslationsCount int `json:"missing_translations_count"`
	StaleTranslationsCount   int `json:"stale_translations_count"`
	DraftTranslationsCount   int `json:"draft_translations_count"`
	ExtraKeysCount           int `json:"extra_keys_count"`
	GlossaryHintsCount       int `json:"glossary_hints_count"`
}

type mcpI18NReportItem struct {
	Language    string `json:"language"`
	Key         string `json:"key"`
	SourceText  string `json:"source_text"`
	CurrentText string `json:"current_text"`
	Status      string `json:"status"`
}

type mcpI18NGlossaryHint struct {
	Language    string `json:"language"`
	Key         string `json:"key"`
	Term        string `json:"term"`
	Recommended string `json:"recommended"`
	CurrentText string `json:"current_text"`
}

type mcpI18NReportData struct {
	SourceLocale        string                `json:"source_locale"`
	Summary             mcpI18NReportSummary  `json:"summary"`
	MissingSource       []mcpI18NReportItem   `json:"missing_source"`
	MissingTranslations []mcpI18NReportItem   `json:"missing_translations"`
	StaleTranslations   []mcpI18NReportItem   `json:"stale_translations"`
	DraftTranslations   []mcpI18NReportItem   `json:"draft_translations"`
	ExtraKeys           []mcpI18NReportItem   `json:"extra_keys"`
	GlossaryHints       []mcpI18NGlossaryHint `json:"glossary_hints"`
}

type mcpI18NCheckResult struct {
	Root         string               `json:"root"`
	Mode         string               `json:"mode"`
	OK           bool                 `json:"ok"`
	SourceLocale string               `json:"sourceLocale,omitempty"`
	Schema       map[string]any       `json:"schema"`
	Summary      mcpI18NReportSummary `json:"summary"`
	Failures     []string             `json:"failures"`
	Warnings     []string             `json:"warnings"`
	NextSteps    []string             `json:"nextSteps"`
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) tools() []tool {
	return []tool{
		{Name: "ncgo_version", Description: "Return ncgo, build, and embedded assets versions.", InputSchema: objectSchema(nil, nil)},
		{Name: "ncgo_doctor", Description: "Run ncgo doctor and return the structured report.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root; empty skips project checks.")}, nil)},
		{Name: "ncgo_ai_sync", Description: "Render AI context files for a project.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root"), "lang": enumProp([]string{ai.LangEN, ai.LangZhCN}), "force": boolProp("Overwrite unmanaged files"), "dryRun": boolProp("Report without writing")}, []string{"root"})},
		{Name: "ncgo_i18n_report", Description: "Read the generated i18n report for a project and return structured payload for agents.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root")}, []string{"root"})},
		{Name: "ncgo_i18n_check", Description: "Evaluate the generated i18n report for dev or release workflows.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root"), "mode": enumProp([]string{mcpI18NCheckDev, mcpI18NCheckRelease})}, []string{"root"})},
		{Name: "ncgo_protolint", Description: "Lint selected .proto files with ncgo's Proto I/O rules and return structured diagnostics.", InputSchema: objectSchema(map[string]any{"root": stringProp("Import root used to resolve the proto files"), "files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Proto entry files relative to root"}, "rules": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional rule IDs to run"}}, []string{"root", "files"})},
		{Name: "ncgo_add_infra", Description: "Install an optional infrastructure add-on into an ncgo project.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root"), "kind": enumProp(infra.SupportedKinds()), "force": boolProp("Overwrite existing generated add-on file"), "wire": boolProp("Opt-in: update generated server/client wiring when supported"), "dryRun": boolProp("Preview intended add-on writes and --wire changes without modifying files")}, []string{"root", "kind"})},
		{Name: "ncgo_add_method", Description: "Insert a usecase method stub at ncgo anchors.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root"), "spec": stringProp("<domain>.<Method>"), "in": enumProp([]string{method.LayerUsecase})}, []string{"root", "spec"})},
	}
}

func (s *Server) callTool(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var p callParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	switch p.Name {
	case "ncgo_version":
		return textResult(versionText(s.NCGOVersion, s.AssetsVersion, s.BuildVersion, s.BuildTime), false), nil
	case "ncgo_doctor":
		return s.callDoctor(ctx, p.Arguments)
	case "ncgo_ai_sync":
		return callAISync(p.Arguments)
	case "ncgo_i18n_report":
		return callI18NReport(p.Arguments)
	case "ncgo_i18n_check":
		return callI18NCheck(p.Arguments)
	case "ncgo_protolint":
		return callProtolint(ctx, p.Arguments)
	case "ncgo_add_infra":
		return callAddInfra(p.Arguments)
	case "ncgo_add_method":
		return callAddMethod(p.Arguments)
	default:
		return nil, fmt.Errorf("unknown tool %q", p.Name)
	}
}

func versionText(ncgoVersion, assetsVersion, buildVersion, buildTime string) string {
	return fmt.Sprintf("ncgo %s (build: %s, built: %s, assets: %s)", nonEmpty(ncgoVersion, "unknown"), nonEmpty(buildVersion, "unknown"), nonEmpty(buildTime, "unknown"), nonEmpty(assetsVersion, "unknown"))
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (s *Server) callDoctor(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root string `json:"root"`
	}
	_ = json.Unmarshal(raw, &args)
	rep := doctor.Run(ctx, doctor.Options{Root: args.Root})
	b, _ := json.MarshalIndent(rep, "", "  ")
	return textResult(string(b), !rep.OK()), nil
}

func callAISync(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Lang   string `json:"lang"`
		Force  bool   `json:"force"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	res, err := ai.Sync(ai.Options{Root: args.Root, Lang: args.Lang, Force: args.Force, DryRun: args.DryRun})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return textResult(string(b), false), nil
}

func callI18NReport(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root string `json:"root"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(args.Root)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	jsonPath := filepath.Join(root, mcpI18NReportJSONPath)
	reportJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return textResult(fmt.Sprintf("i18n report: %s not found; run `make i18n-report` in the project first", jsonPath), true), nil
		}
		return textResult(err.Error(), true), nil
	}
	var report map[string]any
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		return textResult(fmt.Sprintf("i18n report: parse %s: %v", jsonPath, err), true), nil
	}
	out := textResult(formatI18NReportSummary(report), false)
	out["root"] = root
	out["sourceLocale"] = stringAny(report["source_locale"])
	out["localesDir"] = filepath.Join(root, "internal", "pkg", "i18n", "locales")
	out["statusPath"] = filepath.Join(root, "internal", "pkg", "i18n", ".meta", "status.json")
	out["glossaryPath"] = filepath.Join(root, "internal", "pkg", "i18n", "glossary.json")
	out["reportPathJSON"] = jsonPath
	out["reportPathMarkdown"] = filepath.Join(root, mcpI18NReportMDPath)
	out["schema"] = map[string]any{"id": mcpI18NReportSchemaID, "path": mcpI18NReportSchemaPath}
	out["report"] = report
	out["nextSteps"] = []string{
		"review internal/pkg/i18n/.meta/report.md",
		"update internal/pkg/i18n/locales/*.json or invoke an i18n agent",
		"make i18n-check",
		"make i18n",
	}
	return out, nil
}

func callI18NCheck(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root string `json:"root"`
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Mode == "" {
		args.Mode = mcpI18NCheckDev
	}
	if args.Mode != mcpI18NCheckDev && args.Mode != mcpI18NCheckRelease {
		return textResult(fmt.Sprintf("i18n check: unsupported --mode %q; want dev or release", args.Mode), true), nil
	}
	root, err := filepath.Abs(args.Root)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	_, typed, err := loadMCPI18NReport(root)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res := buildMCPI18NCheckResult(root, args.Mode, typed)
	out := textResult(formatI18NCheckSummary(res), !res.OK)
	out["root"] = res.Root
	out["mode"] = res.Mode
	out["ok"] = res.OK
	out["sourceLocale"] = res.SourceLocale
	out["schema"] = res.Schema
	out["summary"] = res.Summary
	out["failures"] = res.Failures
	out["warnings"] = res.Warnings
	out["nextSteps"] = res.NextSteps
	return out, nil
}

func callProtolint(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root  string   `json:"root"`
		Files []string `json:"files"`
		Rules []string `json:"rules"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Root) == "" {
		return textResult("protolint: root is required", true), nil
	}
	if len(args.Files) == 0 {
		return textResult("protolint: at least one file is required", true), nil
	}
	root, err := filepath.Abs(args.Root)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := protolint.Run(ctx, protolint.RunOptions{Root: root, Files: args.Files, RuleIDs: args.Rules})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res.Root = root
	var text strings.Builder
	if err := protolint.WriteText(&text, res); err != nil {
		return textResult(err.Error(), true), nil
	}
	out := textResult(text.String(), !res.OK)
	out["root"] = res.Root
	out["files"] = res.Files
	out["rulesRun"] = res.RulesRun
	out["ok"] = res.OK
	out["summary"] = res.Summary
	out["diagnostics"] = res.Diagnostics
	return out, nil
}

func callAddInfra(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Kind   string `json:"kind"`
		Force  bool   `json:"force"`
		Wire   bool   `json:"wire"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	res, err := infra.Add(infra.Options{Root: args.Root, Kind: args.Kind, Force: args.Force, Wire: args.Wire, DryRun: args.DryRun})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	out := textResult(infra.FormatAddResultText(res), false)
	for key, value := range infra.AddResultFields(res) {
		out[key] = value
	}
	return out, nil
}

func callAddMethod(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root  string `json:"root"`
		Spec  string `json:"spec"`
		Layer string `json:"in"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	res, err := method.Add(method.Options{Root: args.Root, Spec: args.Spec, Layer: args.Layer})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return textResult(fmt.Sprintf("inserted %s.%s into %s", res.Domain, res.Method, res.Path), false), nil
}

func textResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]string{{"type": "text", "text": text}}, "isError": isError}
}

func formatI18NReportSummary(report map[string]any) string {
	source := stringAny(report["source_locale"])
	summary, _ := report["summary"].(map[string]any)
	if summary == nil {
		return fmt.Sprintf("i18n report loaded for %s", nonEmpty(source, "unknown"))
	}
	return fmt.Sprintf(
		"i18n report loaded for %s: locales=%d keys=%d missing=%d stale=%d draft=%d warnings=%d",
		nonEmpty(source, "unknown"),
		intAny(summary["locale_count"]),
		intAny(summary["message_key_count"]),
		intAny(summary["missing_translations_count"]),
		intAny(summary["stale_translations_count"]),
		intAny(summary["draft_translations_count"]),
		intAny(summary["glossary_hints_count"]),
	)
}

func formatI18NCheckSummary(res mcpI18NCheckResult) string {
	status := "ok"
	if !res.OK {
		status = "failed"
	}
	return fmt.Sprintf(
		"i18n check (%s): %s (failures=%d warnings=%d)",
		res.Mode,
		status,
		len(res.Failures),
		len(res.Warnings),
	)
}

func buildMCPI18NCheckResult(root, mode string, report mcpI18NReportData) mcpI18NCheckResult {
	res := mcpI18NCheckResult{
		Root:         root,
		Mode:         mode,
		OK:           true,
		SourceLocale: report.SourceLocale,
		Failures:     []string{},
		Warnings:     []string{},
		Schema: map[string]any{
			"report": map[string]any{"id": mcpI18NReportSchemaID, "path": mcpI18NReportSchemaPath},
		},
		Summary: report.Summary,
		NextSteps: []string{
			"review internal/pkg/i18n/.meta/report.md",
			"make i18n-check",
			"make i18n",
		},
	}
	for _, item := range report.MissingSource {
		res.Failures = append(res.Failures, fmt.Sprintf("missing source message for %s", item.Key))
	}
	for _, item := range report.MissingTranslations {
		if stringAny(item.CurrentText) == "" {
			res.Failures = append(res.Failures, fmt.Sprintf("%s/%s is missing", item.Language, item.Key))
			continue
		}
		if mode == mcpI18NCheckRelease {
			res.Failures = append(res.Failures, fmt.Sprintf("%s/%s is still placeholder text", item.Language, item.Key))
		}
	}
	if mode == mcpI18NCheckRelease {
		for _, item := range report.DraftTranslations {
			res.Failures = append(res.Failures, fmt.Sprintf("%s/%s is draft", item.Language, item.Key))
		}
		for _, item := range report.StaleTranslations {
			res.Failures = append(res.Failures, fmt.Sprintf("%s/%s is stale", item.Language, item.Key))
		}
	}
	for _, hint := range report.GlossaryHints {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s/%s may not use recommended glossary term %q (want %q)", hint.Language, hint.Key, hint.Term, hint.Recommended))
	}
	res.OK = len(res.Failures) == 0
	return res
}

func loadMCPI18NReport(root string) (map[string]any, mcpI18NReportData, error) {
	jsonPath := filepath.Join(root, mcpI18NReportJSONPath)
	reportJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, mcpI18NReportData{}, fmt.Errorf("i18n report: %s not found; run `make i18n-report` in the project first", jsonPath)
		}
		return nil, mcpI18NReportData{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(reportJSON, &raw); err != nil {
		return nil, mcpI18NReportData{}, fmt.Errorf("i18n report: parse %s: %v", jsonPath, err)
	}
	var typed mcpI18NReportData
	if err := json.Unmarshal(reportJSON, &typed); err != nil {
		return nil, mcpI18NReportData{}, fmt.Errorf("i18n report: decode %s: %v", jsonPath, err)
	}
	return raw, typed, nil
}

func stringAny(v any) string {
	s, _ := v.(string)
	return s
}

func intAny(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func objectSchema(props map[string]any, required []string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func stringProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}
func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func enumProp(vals []string) map[string]any { return map[string]any{"type": "string", "enum": vals} }

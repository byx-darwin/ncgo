package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	mcpI18NReportJSONPath   = "internal/pkg/i18n/.meta/report.json"
	mcpI18NReportMDPath     = "internal/pkg/i18n/.meta/report.md"
	mcpI18NReportSchemaID   = "ncgo://schemas/i18n/report-input-v1"
	mcpI18NReportSchemaPath = "schemas/i18n/report-input-v1.schema.json"
	mcpI18NCheckDev         = "dev"
	mcpI18NCheckRelease     = "release"
)

var i18nReportMCPTool = structuredMCPTool[mcpI18NReportResult]{
	name:      "i18n report",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPI18NReportOutput,
	fields:    mcpI18NReportFields,
	isError: func(mcpI18NReportResult) bool {
		return false
	},
}

var i18nCheckMCPTool = structuredMCPTool[mcpI18NCheckResult]{
	name:      "i18n check",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPI18NCheckOutput,
	fields:    mcpI18NCheckFields,
	isError: func(res mcpI18NCheckResult) bool {
		return !res.OK
	},
}

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

type mcpI18NReportResult struct {
	Root               string         `json:"root"`
	SourceLocale       string         `json:"sourceLocale,omitempty"`
	LocalesDir         string         `json:"localesDir"`
	StatusPath         string         `json:"statusPath"`
	GlossaryPath       string         `json:"glossaryPath"`
	ReportPathJSON     string         `json:"reportPathJSON"`
	ReportPathMarkdown string         `json:"reportPathMarkdown"`
	Schema             map[string]any `json:"schema"`
	Report             map[string]any `json:"report"`
	NextSteps          []string       `json:"nextSteps"`
}

func callI18NReport(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := i18nReportMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	root, err := filepath.Abs(args.Root)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	rawReport, _, err := loadMCPI18NReport(root)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res := buildMCPI18NReportResult(root, rawReport)
	out, err := i18nReportMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func callI18NCheck(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Mode   string `json:"mode"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := i18nCheckMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
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
	out, err := i18nCheckMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func formatMCPI18NReportOutput(res mcpI18NReportResult, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			_, err := io.WriteString(w, formatI18NReportSummary(res.Report))
			return err
		},
		mcpOutputJSON: func(w io.Writer) error { return writeJSONOutput(w, res) },
	})
}

func formatMCPI18NCheckOutput(res mcpI18NCheckResult, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			_, err := io.WriteString(w, formatI18NCheckSummary(res))
			return err
		},
		mcpOutputJSON: func(w io.Writer) error { return writeJSONOutput(w, res) },
	})
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
	return fmt.Sprintf("i18n check (%s): %s (failures=%d warnings=%d)", res.Mode, status, len(res.Failures), len(res.Warnings))
}

func buildMCPI18NCheckResult(root, mode string, report mcpI18NReportData) mcpI18NCheckResult {
	res := mcpI18NCheckResult{Root: root, Mode: mode, OK: true, SourceLocale: report.SourceLocale, Failures: []string{}, Warnings: []string{}, Schema: map[string]any{"report": map[string]any{"id": mcpI18NReportSchemaID, "path": mcpI18NReportSchemaPath}}, Summary: report.Summary, NextSteps: []string{"review internal/pkg/i18n/.meta/report.md", "make i18n-check", "make i18n"}}
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

func buildMCPI18NReportResult(root string, report map[string]any) mcpI18NReportResult {
	return mcpI18NReportResult{Root: root, SourceLocale: stringAny(report["source_locale"]), LocalesDir: filepath.Join(root, "internal", "pkg", "i18n", "locales"), StatusPath: filepath.Join(root, "internal", "pkg", "i18n", ".meta", "status.json"), GlossaryPath: filepath.Join(root, "internal", "pkg", "i18n", "glossary.json"), ReportPathJSON: filepath.Join(root, mcpI18NReportJSONPath), ReportPathMarkdown: filepath.Join(root, mcpI18NReportMDPath), Schema: map[string]any{"id": mcpI18NReportSchemaID, "path": mcpI18NReportSchemaPath}, Report: report, NextSteps: []string{"review internal/pkg/i18n/.meta/report.md", "update internal/pkg/i18n/locales/*.json or invoke an i18n agent", "make i18n-check", "make i18n"}}
}

func mcpI18NReportFields(res mcpI18NReportResult) map[string]any {
	return map[string]any{"root": res.Root, "sourceLocale": res.SourceLocale, "localesDir": res.LocalesDir, "statusPath": res.StatusPath, "glossaryPath": res.GlossaryPath, "reportPathJSON": res.ReportPathJSON, "reportPathMarkdown": res.ReportPathMarkdown, "schema": res.Schema, "report": res.Report, "nextSteps": res.NextSteps}
}

func mcpI18NCheckFields(res mcpI18NCheckResult) map[string]any {
	return map[string]any{"root": res.Root, "mode": res.Mode, "ok": res.OK, "sourceLocale": res.SourceLocale, "schema": res.Schema, "summary": res.Summary, "failures": res.Failures, "warnings": res.Warnings, "nextSteps": res.NextSteps}
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

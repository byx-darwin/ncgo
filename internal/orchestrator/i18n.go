package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultI18NReportJSON = "internal/pkg/i18n/.meta/report.json"
	defaultI18NReportMD   = "internal/pkg/i18n/.meta/report.md"
	i18nReportSchemaID    = "ncgo://schemas/i18n/report-input-v1"
	i18nReportSchemaPath  = "schemas/i18n/report-input-v1.schema.json"
	I18nCheckDev          = "dev"
	I18nCheckRelease      = "release"
)

// I18nReportOptions configures an i18n report load.
type I18nReportOptions struct {
	Root string
}

// I18nReportResult is the structured result of reading the i18n report.
type I18nReportResult struct {
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

// I18nCheckOptions configures an i18n check run.
type I18nCheckOptions struct {
	Root string
	Mode string
}

// I18nCheckResult is the structured result of an i18n check.
type I18nCheckResult struct {
	Root         string            `json:"root"`
	Mode         string            `json:"mode"`
	OK           bool              `json:"ok"`
	SourceLocale string            `json:"sourceLocale,omitempty"`
	Schema       map[string]any    `json:"schema"`
	Summary      I18nReportSummary `json:"summary"`
	Failures     []string          `json:"failures"`
	Warnings     []string          `json:"warnings"`
	NextSteps    []string          `json:"nextSteps"`
}

// I18nReportSummary mirrors the summary block in report.json.
type I18nReportSummary struct {
	LocaleCount              int `json:"locale_count"`
	MessageKeyCount          int `json:"message_key_count"`
	MissingSourceCount       int `json:"missing_source_count"`
	MissingTranslationsCount int `json:"missing_translations_count"`
	StaleTranslationsCount   int `json:"stale_translations_count"`
	DraftTranslationsCount   int `json:"draft_translations_count"`
	ExtraKeysCount           int `json:"extra_keys_count"`
	GlossaryHintsCount       int `json:"glossary_hints_count"`
}

// I18nReportItem describes a single translation status entry.
type I18nReportItem struct {
	Language    string `json:"language"`
	Key         string `json:"key"`
	SourceText  string `json:"source_text"`
	CurrentText string `json:"current_text"`
	Status      string `json:"status"`
}

// I18nGlossaryHint describes a glossary compliance hint.
type I18nGlossaryHint struct {
	Language    string `json:"language"`
	Key         string `json:"key"`
	Term        string `json:"term"`
	Recommended string `json:"recommended"`
	CurrentText string `json:"current_text"`
}

// I18nReportData is the typed structure parsed from report.json.
type I18nReportData struct {
	SourceLocale        string             `json:"source_locale"`
	Summary             I18nReportSummary  `json:"summary"`
	MissingSource       []I18nReportItem   `json:"missing_source"`
	MissingTranslations []I18nReportItem   `json:"missing_translations"`
	StaleTranslations   []I18nReportItem   `json:"stale_translations"`
	DraftTranslations   []I18nReportItem   `json:"draft_translations"`
	ExtraKeys           []I18nReportItem   `json:"extra_keys"`
	GlossaryHints       []I18nGlossaryHint `json:"glossary_hints"`
}

// RunI18nReport loads the i18n report from a project root.
func RunI18nReport(ctx context.Context, opts I18nReportOptions) (*I18nReportResult, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	jsonPath := filepath.Join(root, defaultI18NReportJSON)
	mdPath := filepath.Join(root, defaultI18NReportMD)

	raw, _, err := loadI18nReportFile(root)
	if err != nil {
		return nil, err
	}

	return &I18nReportResult{
		Root:               root,
		SourceLocale:       stringFromMap(raw, "source_locale"),
		LocalesDir:         filepath.Join(root, "internal", "pkg", "i18n", "locales"),
		StatusPath:         filepath.Join(root, "internal", "pkg", "i18n", ".meta", "status.json"),
		GlossaryPath:       filepath.Join(root, "internal", "pkg", "i18n", "glossary.json"),
		ReportPathJSON:     jsonPath,
		ReportPathMarkdown: mdPath,
		Schema: map[string]any{
			"id":   i18nReportSchemaID,
			"path": i18nReportSchemaPath,
		},
		Report: raw,
		NextSteps: []string{
			"review internal/pkg/i18n/.meta/report.md",
			"update internal/pkg/i18n/locales/*.json or invoke an i18n agent",
			"make i18n-check",
			"make i18n",
		},
	}, nil
}

// RunI18nCheck loads the i18n report and evaluates it for dev or release workflows.
func RunI18nCheck(ctx context.Context, opts I18nCheckOptions) (*I18nCheckResult, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Mode != I18nCheckDev && opts.Mode != I18nCheckRelease {
		return nil, fmt.Errorf("i18n check: unsupported --mode %q; want dev or release", opts.Mode)
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	_, typed, err := loadI18nReportFile(root)
	if err != nil {
		return nil, err
	}
	return buildI18NCheckResult(root, opts.Mode, typed), nil
}

func loadI18nReportFile(root string) (map[string]any, I18nReportData, error) {
	jsonPath := filepath.Join(root, defaultI18NReportJSON)
	reportJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, I18nReportData{}, fmt.Errorf("i18n report: %s not found; run `make i18n-report` in the project first", jsonPath)
		}
		return nil, I18nReportData{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(reportJSON, &raw); err != nil {
		return nil, I18nReportData{}, fmt.Errorf("i18n report: parse %s: %w", jsonPath, err)
	}
	var typed I18nReportData
	if err := json.Unmarshal(reportJSON, &typed); err != nil {
		return nil, I18nReportData{}, fmt.Errorf("i18n report: decode %s: %w", jsonPath, err)
	}
	return raw, typed, nil
}

func buildI18NCheckResult(root, mode string, report I18nReportData) *I18nCheckResult {
	res := &I18nCheckResult{
		Root:         root,
		Mode:         mode,
		OK:           true,
		SourceLocale: report.SourceLocale,
		Schema: map[string]any{
			"report": map[string]any{"id": i18nReportSchemaID, "path": i18nReportSchemaPath},
		},
		Summary:  report.Summary,
		Failures: []string{},
		Warnings: []string{},
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
		current := strings.TrimSpace(item.CurrentText)
		if current == "" {
			res.Failures = append(res.Failures, fmt.Sprintf("%s/%s is missing", item.Language, item.Key))
			continue
		}
		if mode == I18nCheckRelease {
			res.Failures = append(res.Failures, fmt.Sprintf("%s/%s is still placeholder text", item.Language, item.Key))
		}
	}
	if mode == I18nCheckRelease {
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

func stringFromMap(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

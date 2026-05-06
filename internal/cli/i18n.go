package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	defaultI18NReportJSON = "internal/pkg/i18n/.meta/report.json"
	defaultI18NReportMD   = "internal/pkg/i18n/.meta/report.md"
	i18nReportSchemaID    = "ncgo://schemas/i18n/report-input-v1"
	i18nReportSchemaPath  = "schemas/i18n/report-input-v1.schema.json"
	i18nCheckDev          = "dev"
	i18nCheckRelease      = "release"
)

func newI18NCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "i18n",
		Short: "Inspect project i18n artifacts and emit agent-friendly structured output",
	}
	cmd.AddCommand(newI18NReportCmd())
	cmd.AddCommand(newI18NCheckCmd())
	return cmd
}

type i18nReportOptions struct {
	root   string
	output string
}

type i18nCheckOptions struct {
	root   string
	output string
	mode   string
}

type i18nReportSummary struct {
	LocaleCount              int `json:"locale_count"`
	MessageKeyCount          int `json:"message_key_count"`
	MissingSourceCount       int `json:"missing_source_count"`
	MissingTranslationsCount int `json:"missing_translations_count"`
	StaleTranslationsCount   int `json:"stale_translations_count"`
	DraftTranslationsCount   int `json:"draft_translations_count"`
	ExtraKeysCount           int `json:"extra_keys_count"`
	GlossaryHintsCount       int `json:"glossary_hints_count"`
}

type i18nReportItem struct {
	Language    string `json:"language"`
	Key         string `json:"key"`
	SourceText  string `json:"source_text"`
	CurrentText string `json:"current_text"`
	Status      string `json:"status"`
}

type i18nGlossaryHint struct {
	Language    string `json:"language"`
	Key         string `json:"key"`
	Term        string `json:"term"`
	Recommended string `json:"recommended"`
	CurrentText string `json:"current_text"`
}

type i18nReportData struct {
	SourceLocale        string             `json:"source_locale"`
	Summary             i18nReportSummary  `json:"summary"`
	MissingSource       []i18nReportItem   `json:"missing_source"`
	MissingTranslations []i18nReportItem   `json:"missing_translations"`
	StaleTranslations   []i18nReportItem   `json:"stale_translations"`
	DraftTranslations   []i18nReportItem   `json:"draft_translations"`
	ExtraKeys           []i18nReportItem   `json:"extra_keys"`
	GlossaryHints       []i18nGlossaryHint `json:"glossary_hints"`
}

type i18nCheckResult struct {
	Root         string            `json:"root"`
	Mode         string            `json:"mode"`
	OK           bool              `json:"ok"`
	SourceLocale string            `json:"sourceLocale,omitempty"`
	Schema       map[string]any    `json:"schema"`
	Summary      i18nReportSummary `json:"summary"`
	Failures     []string          `json:"failures"`
	Warnings     []string          `json:"warnings"`
	NextSteps    []string          `json:"nextSteps"`
}

func newI18NReportCmd() *cobra.Command {
	opts := &i18nReportOptions{}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Read the generated i18n report from a project",
		Long: "Load internal/pkg/i18n/.meta/report.json and report.md from an ncgo project. " +
			"Use --output json to emit a structured wrapper around report.json for AI agents and other tooling.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runI18NReport(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing internal/pkg/i18n/.meta/report.json")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func newI18NCheckCmd() *cobra.Command {
	opts := &i18nCheckOptions{}
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Evaluate i18n report health for dev or release workflows",
		Long: "Load internal/pkg/i18n/.meta/report.json from an ncgo project and summarize the " +
			"blocking failures and non-blocking warnings implied by the current i18n report.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runI18NCheck(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing internal/pkg/i18n/.meta/report.json")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	f.StringVar(&opts.mode, "mode", i18nCheckDev, "Check mode: dev or release")
	return cmd
}

func runI18NReport(cmd *cobra.Command, opts *i18nReportOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return fmt.Errorf("i18n report: unsupported --output %q; want text or json", opts.output)
	}
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	jsonPath, mdPath, report, _, err := loadI18NReport(root)
	if err != nil {
		return err
	}
	if opts.output == "json" {
		return writeI18NReportJSON(cmd.OutOrStdout(), root, jsonPath, mdPath, report)
	}
	reportMD, err := os.ReadFile(mdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("i18n report: %s not found; run `make i18n-report` in the project first", mdPath)
		}
		return err
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), string(reportMD))
	return err
}

func runI18NCheck(cmd *cobra.Command, opts *i18nCheckOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return fmt.Errorf("i18n check: unsupported --output %q; want text or json", opts.output)
	}
	if opts.mode != i18nCheckDev && opts.mode != i18nCheckRelease {
		return fmt.Errorf("i18n check: unsupported --mode %q; want dev or release", opts.mode)
	}
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	_, _, _, typed, err := loadI18NReport(root)
	if err != nil {
		return err
	}
	result := buildI18NCheckResult(root, opts.mode, typed)
	out := cmd.OutOrStdout()
	if opts.output == "json" {
		if err := writeI18NCheckJSON(out, result); err != nil {
			return err
		}
	} else {
		if err := writeI18NCheckText(out, result); err != nil {
			return err
		}
	}
	if !result.OK {
		return errSilentFailure
	}
	return nil
}

func writeI18NReportJSON(out io.Writer, root, jsonPath, mdPath string, report map[string]any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
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
	}{
		Root:               root,
		SourceLocale:       stringFromMap(report, "source_locale"),
		LocalesDir:         filepath.Join(root, "internal", "pkg", "i18n", "locales"),
		StatusPath:         filepath.Join(root, "internal", "pkg", "i18n", ".meta", "status.json"),
		GlossaryPath:       filepath.Join(root, "internal", "pkg", "i18n", "glossary.json"),
		ReportPathJSON:     jsonPath,
		ReportPathMarkdown: mdPath,
		Schema: map[string]any{
			"id":   i18nReportSchemaID,
			"path": i18nReportSchemaPath,
		},
		Report: report,
		NextSteps: []string{
			"review internal/pkg/i18n/.meta/report.md",
			"update internal/pkg/i18n/locales/*.json or invoke an i18n agent",
			"make i18n-check",
			"make i18n",
		},
	})
}

func writeI18NCheckJSON(out io.Writer, res i18nCheckResult) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func writeI18NCheckText(out io.Writer, res i18nCheckResult) error {
	status := "ok"
	if !res.OK {
		status = "failed"
	}
	if _, err := fmt.Fprintf(out, "i18n check (%s): %s\n", res.Mode, status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "source locale: %s\n", nonEmpty(res.SourceLocale, "unknown")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "summary: locales=%d keys=%d failures=%d warnings=%d\n", res.Summary.LocaleCount, res.Summary.MessageKeyCount, len(res.Failures), len(res.Warnings)); err != nil {
		return err
	}
	if len(res.Failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
			return err
		}
		for _, item := range res.Failures {
			if _, err := fmt.Fprintf(out, "  - %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(res.Warnings) > 0 {
		if _, err := fmt.Fprintln(out, "\nwarnings:"); err != nil {
			return err
		}
		for _, item := range res.Warnings {
			if _, err := fmt.Fprintf(out, "  - %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(res.NextSteps) > 0 {
		if _, err := fmt.Fprintln(out, "\nnext steps:"); err != nil {
			return err
		}
		for _, item := range res.NextSteps {
			if _, err := fmt.Fprintf(out, "  - %s\n", item); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildI18NCheckResult(root, mode string, report i18nReportData) i18nCheckResult {
	res := i18nCheckResult{
		Root:         root,
		Mode:         mode,
		OK:           true,
		SourceLocale: report.SourceLocale,
		Schema: map[string]any{
			"report": map[string]any{"id": i18nReportSchemaID, "path": i18nReportSchemaPath},
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
		current := strings.TrimSpace(item.CurrentText)
		if current == "" {
			res.Failures = append(res.Failures, fmt.Sprintf("%s/%s is missing", item.Language, item.Key))
			continue
		}
		if mode == i18nCheckRelease {
			res.Failures = append(res.Failures, fmt.Sprintf("%s/%s is still placeholder text", item.Language, item.Key))
		}
	}
	if mode == i18nCheckRelease {
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

func loadI18NReport(root string) (string, string, map[string]any, i18nReportData, error) {
	jsonPath := filepath.Join(root, defaultI18NReportJSON)
	mdPath := filepath.Join(root, defaultI18NReportMD)
	reportJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil, i18nReportData{}, fmt.Errorf("i18n report: %s not found; run `make i18n-report` in the project first", jsonPath)
		}
		return "", "", nil, i18nReportData{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(reportJSON, &raw); err != nil {
		return "", "", nil, i18nReportData{}, fmt.Errorf("i18n report: parse %s: %w", jsonPath, err)
	}
	var typed i18nReportData
	if err := json.Unmarshal(reportJSON, &typed); err != nil {
		return "", "", nil, i18nReportData{}, fmt.Errorf("i18n report: decode %s: %w", jsonPath, err)
	}
	return jsonPath, mdPath, raw, typed, nil
}

func stringFromMap(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

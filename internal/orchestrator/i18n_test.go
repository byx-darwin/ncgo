package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedI18NReportProject(t *testing.T, reportJSON string) string {
	t.Helper()
	root := t.TempDir()
	metaDir := filepath.Join(root, "internal", "pkg", "i18n", ".meta")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("mkdir meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "report.json"), []byte(reportJSON), 0o644); err != nil {
		t.Fatalf("write report.json: %v", err)
	}
	return root
}

func basicReportJSON() string {
	return `{
  "summary": {
    "locale_count": 2,
    "message_key_count": 1,
    "missing_source_count": 0,
    "missing_translations_count": 1,
    "stale_translations_count": 0,
    "draft_translations_count": 0,
    "extra_keys_count": 0,
    "glossary_hints_count": 1
  },
  "source_locale": "zh-CN",
  "missing_source": [],
  "missing_translations": [{"language": "it-IT", "key": "internal_error", "source_text": "内部错误", "current_text": "__TODO__: 内部错误", "status": "draft"}],
  "stale_translations": [],
  "draft_translations": [],
  "extra_keys": [],
  "glossary_hints": [{"language": "it-IT", "key": "internal_error", "term": "signature", "recommended": "firma", "current_text": "errore interno"}]
}`
}

func releaseReportJSON() string {
	return `{
  "summary": {
    "locale_count": 2,
    "message_key_count": 1,
    "missing_source_count": 0,
    "missing_translations_count": 1,
    "stale_translations_count": 1,
    "draft_translations_count": 1,
    "extra_keys_count": 0,
    "glossary_hints_count": 1
  },
  "source_locale": "zh-CN",
  "missing_source": [],
  "missing_translations": [{"language": "it-IT", "key": "internal_error", "source_text": "内部错误", "current_text": "__TODO__: 内部错误", "status": "draft"}],
  "stale_translations": [{"language": "it-IT", "key": "internal_error", "source_text": "内部错误", "current_text": "errore interno", "status": "stale"}],
  "draft_translations": [{"language": "it-IT", "key": "internal_error", "source_text": "内部错误", "current_text": "errore interno", "status": "draft"}],
  "extra_keys": [],
  "glossary_hints": [{"language": "it-IT", "key": "internal_error", "term": "signature", "recommended": "firma", "current_text": "errore interno"}]
}`
}

func TestRunI18nReport(t *testing.T) {
	root := seedI18NReportProject(t, basicReportJSON())

	result, err := RunI18nReport(context.Background(), I18nReportOptions{Root: root})
	if err != nil {
		t.Fatalf("RunI18nReport: %v", err)
	}
	if result.Root != root {
		t.Fatalf("root = %q, want %q", result.Root, root)
	}
	if result.SourceLocale != "zh-CN" {
		t.Fatalf("sourceLocale = %q, want zh-CN", result.SourceLocale)
	}
	if result.ReportPathJSON != filepath.Join(root, defaultI18NReportJSON) {
		t.Fatalf("reportPathJSON = %q", result.ReportPathJSON)
	}
	if result.ReportPathMarkdown != filepath.Join(root, defaultI18NReportMD) {
		t.Fatalf("reportPathMarkdown = %q", result.ReportPathMarkdown)
	}
	schema := result.Schema
	if schema["id"] != i18nReportSchemaID || schema["path"] != i18nReportSchemaPath {
		t.Fatalf("schema = %+v", schema)
	}
	report := result.Report
	if report["source_locale"] != "zh-CN" {
		t.Fatalf("report.source_locale = %+v", report["source_locale"])
	}
	if len(result.NextSteps) == 0 {
		t.Fatalf("nextSteps empty")
	}
	if result.NextSteps[0] != "review internal/pkg/i18n/.meta/report.md" {
		t.Fatalf("nextSteps = %v", result.NextSteps)
	}
}

func TestRunI18nReportMissing(t *testing.T) {
	root := t.TempDir()

	_, err := RunI18nReport(context.Background(), I18nReportOptions{Root: root})
	if err == nil || !strings.Contains(err.Error(), "run `make i18n-report`") {
		t.Fatalf("err = %v, want guidance to run make i18n-report", err)
	}
}

func TestRunI18nCheckDev(t *testing.T) {
	root := seedI18NReportProject(t, basicReportJSON())

	result, err := RunI18nCheck(context.Background(), I18nCheckOptions{Root: root, Mode: I18nCheckDev})
	if err != nil {
		t.Fatalf("RunI18nCheck dev: %v", err)
	}
	if result.Mode != I18nCheckDev || !result.OK {
		t.Fatalf("check result = %+v", result)
	}
	if result.SourceLocale != "zh-CN" {
		t.Fatalf("sourceLocale = %q", result.SourceLocale)
	}
	if len(result.Failures) != 0 || len(result.Warnings) != 1 {
		t.Fatalf("failures/warnings = %v/%v", result.Failures, result.Warnings)
	}
	reportSchema, _ := result.Schema["report"].(map[string]any)
	if reportSchema["id"] != i18nReportSchemaID || reportSchema["path"] != i18nReportSchemaPath {
		t.Fatalf("schema = %+v", result.Schema)
	}
}

func TestRunI18nCheckRelease(t *testing.T) {
	root := seedI18NReportProject(t, releaseReportJSON())

	result, err := RunI18nCheck(context.Background(), I18nCheckOptions{Root: root, Mode: I18nCheckRelease})
	if err != nil {
		t.Fatalf("RunI18nCheck release: %v", err)
	}
	if result.OK {
		t.Fatalf("release check unexpectedly ok: %+v", result)
	}
	if len(result.Failures) < 3 {
		t.Fatalf("release failures too short: %+v", result.Failures)
	}
	joined := strings.Join(result.Failures, "\n")
	for _, want := range []string{"still placeholder text", "is draft", "is stale"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failures missing %q:\n%s", want, joined)
		}
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("warnings = %v, want 1", result.Warnings)
	}
}

func TestRunI18nCheckInvalidMode(t *testing.T) {
	root := seedI18NReportProject(t, basicReportJSON())

	_, err := RunI18nCheck(context.Background(), I18nCheckOptions{Root: root, Mode: "strict"})
	if err == nil || !strings.Contains(err.Error(), "unsupported --mode") {
		t.Fatalf("err = %v, want unsupported mode", err)
	}
}

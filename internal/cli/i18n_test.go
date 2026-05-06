package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunI18NReportJSON(t *testing.T) {
	root := seedI18NReportProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runI18NReport(cmd, &i18nReportOptions{root: root, output: "json"})
	if err != nil {
		t.Fatalf("runI18NReport json: %v", err)
	}
	var got struct {
		Root           string         `json:"root"`
		SourceLocale   string         `json:"sourceLocale"`
		ReportPathJSON string         `json:"reportPathJSON"`
		ReportPathMD   string         `json:"reportPathMarkdown"`
		NextSteps      []string       `json:"nextSteps"`
		Schema         map[string]any `json:"schema"`
		Report         map[string]any `json:"report"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid json: %v\n%s", err, out.String())
	}
	if got.Root != root {
		t.Fatalf("root = %q, want %q", got.Root, root)
	}
	if got.SourceLocale != "zh-CN" {
		t.Fatalf("sourceLocale = %q, want zh-CN", got.SourceLocale)
	}
	if got.ReportPathJSON != filepath.Join(root, defaultI18NReportJSON) {
		t.Fatalf("reportPathJSON = %q", got.ReportPathJSON)
	}
	if got.ReportPathMD != filepath.Join(root, defaultI18NReportMD) {
		t.Fatalf("reportPathMarkdown = %q", got.ReportPathMD)
	}
	if got.Schema["id"] != i18nReportSchemaID || got.Schema["path"] != i18nReportSchemaPath {
		t.Fatalf("schema = %+v", got.Schema)
	}
	if got.Report["source_locale"] != "zh-CN" {
		t.Fatalf("report.source_locale = %+v", got.Report["source_locale"])
	}
	if len(got.NextSteps) == 0 {
		t.Fatalf("nextSteps empty")
	}
	if got.NextSteps[0] != "review internal/pkg/i18n/.meta/report.md" {
		t.Fatalf("nextSteps = %v", got.NextSteps)
	}
}

func TestRunI18NReportText(t *testing.T) {
	root := seedI18NReportProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runI18NReport(cmd, &i18nReportOptions{root: root, output: "text"})
	if err != nil {
		t.Fatalf("runI18NReport text: %v", err)
	}
	if !strings.Contains(out.String(), "# i18n Report") || !strings.Contains(out.String(), "glossary hints") {
		t.Fatalf("text output missing report markdown:\n%s", out.String())
	}
}

func TestRunI18NReportRejectsInvalidOutput(t *testing.T) {
	root := seedI18NReportProject(t)
	cmd := &cobra.Command{}
	err := runI18NReport(cmd, &i18nReportOptions{root: root, output: "yaml"})
	if err == nil || !strings.Contains(err.Error(), "unsupported --output") {
		t.Fatalf("err = %v, want unsupported output", err)
	}
}

func TestRunI18NReportMissingJSON(t *testing.T) {
	root := t.TempDir()
	cmd := &cobra.Command{}
	err := runI18NReport(cmd, &i18nReportOptions{root: root, output: "json"})
	if err == nil || !strings.Contains(err.Error(), "run `make i18n-report`") {
		t.Fatalf("err = %v, want guidance to run make i18n-report", err)
	}
}

func TestRunI18NCheckJSONDevOKWithWarnings(t *testing.T) {
	root := seedI18NReportProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runI18NCheck(cmd, &i18nCheckOptions{root: root, mode: i18nCheckDev, output: "json"})
	if err != nil {
		t.Fatalf("runI18NCheck dev json: %v", err)
	}
	var got struct {
		Mode         string         `json:"mode"`
		OK           bool           `json:"ok"`
		SourceLocale string         `json:"sourceLocale"`
		Schema       map[string]any `json:"schema"`
		Failures     []string       `json:"failures"`
		Warnings     []string       `json:"warnings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("check output is not valid json: %v\n%s", err, out.String())
	}
	if got.Mode != i18nCheckDev || !got.OK {
		t.Fatalf("check result = %+v", got)
	}
	if got.SourceLocale != "zh-CN" {
		t.Fatalf("sourceLocale = %q", got.SourceLocale)
	}
	if len(got.Failures) != 0 || len(got.Warnings) != 1 {
		t.Fatalf("failures/warnings = %v/%v", got.Failures, got.Warnings)
	}
	reportSchema, _ := got.Schema["report"].(map[string]any)
	if reportSchema["id"] != i18nReportSchemaID || reportSchema["path"] != i18nReportSchemaPath {
		t.Fatalf("schema = %+v", got.Schema)
	}
}

func TestRunI18NCheckJSONReleaseFails(t *testing.T) {
	root := seedI18NCheckReleaseProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runI18NCheck(cmd, &i18nCheckOptions{root: root, mode: i18nCheckRelease, output: "json"})
	if !errors.Is(err, errSilentFailure) {
		t.Fatalf("err = %v, want errSilentFailure", err)
	}
	var got struct {
		OK       bool     `json:"ok"`
		Failures []string `json:"failures"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("check release output is not valid json: %v\n%s", err, out.String())
	}
	if got.OK {
		t.Fatalf("release check unexpectedly ok: %+v", got)
	}
	if len(got.Failures) < 3 {
		t.Fatalf("release failures too short: %+v", got.Failures)
	}
	joined := strings.Join(got.Failures, "\n")
	for _, want := range []string{"still placeholder text", "is draft", "is stale"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failures missing %q:\n%s", want, joined)
		}
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("warnings = %v, want 1", got.Warnings)
	}
}

func TestRunI18NCheckText(t *testing.T) {
	root := seedI18NReportProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runI18NCheck(cmd, &i18nCheckOptions{root: root, mode: i18nCheckDev, output: "text"})
	if err != nil {
		t.Fatalf("runI18NCheck text: %v", err)
	}
	if !strings.Contains(out.String(), "i18n check (dev): ok") || !strings.Contains(out.String(), "warnings:") {
		t.Fatalf("text output missing expected content:\n%s", out.String())
	}
}

func TestRunI18NCheckRejectsInvalidMode(t *testing.T) {
	root := seedI18NReportProject(t)
	cmd := &cobra.Command{}
	err := runI18NCheck(cmd, &i18nCheckOptions{root: root, mode: "strict", output: "json"})
	if err == nil || !strings.Contains(err.Error(), "unsupported --mode") {
		t.Fatalf("err = %v, want unsupported mode", err)
	}
}

func seedI18NReportProject(t *testing.T) string {
	t.Helper()
	return seedI18NProjectWithReport(t, `{
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
	}`)
}

func seedI18NCheckReleaseProject(t *testing.T) string {
	t.Helper()
	return seedI18NProjectWithReport(t, `{
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
	}`)
}

func seedI18NProjectWithReport(t *testing.T, reportJSON string) string {
	t.Helper()
	root := t.TempDir()
	metaDir := filepath.Join(root, "internal", "pkg", "i18n", ".meta")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("mkdir meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "report.json"), []byte(reportJSON), 0o644); err != nil {
		t.Fatalf("write report.json: %v", err)
	}
	reportMD := "# i18n Report\n\n- source locale: `zh-CN`\n- glossary hints: 1\n"
	if err := os.WriteFile(filepath.Join(metaDir, "report.md"), []byte(reportMD), 0o644); err != nil {
		t.Fatalf("write report.md: %v", err)
	}
	return root
}

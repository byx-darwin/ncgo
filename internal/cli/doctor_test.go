package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

func TestResolveDoctorOutput(t *testing.T) {
	t.Run("default text", func(t *testing.T) {
		got, err := resolveDoctorOutput(&doctorOptions{})
		if err != nil || got != "text" {
			t.Fatalf("resolveDoctorOutput() = (%q, %v), want (text, nil)", got, err)
		}
	})
	t.Run("json alias", func(t *testing.T) {
		got, err := resolveDoctorOutput(&doctorOptions{asJSON: true})
		if err != nil || got != "json" {
			t.Fatalf("resolveDoctorOutput() = (%q, %v), want (json, nil)", got, err)
		}
	})
	t.Run("conflicting alias", func(t *testing.T) {
		_, err := resolveDoctorOutput(&doctorOptions{asJSON: true, output: "sarif"})
		if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("err = %v, want conflict error", err)
		}
	})
	t.Run("unsupported output", func(t *testing.T) {
		_, err := resolveDoctorOutput(&doctorOptions{output: "xml"})
		if err == nil || !strings.Contains(err.Error(), "unsupported --output") {
			t.Fatalf("err = %v, want unsupported output error", err)
		}
	})
}

func TestRunDoctorJSON(t *testing.T) {
	old := runDoctorReport
	runDoctorReport = func(context.Context, doctor.Options) *doctor.Report {
		return &doctor.Report{
			Root:  "/repo/demo",
			Scope: doctor.ScopeService,
			Summary: doctor.ReportSummary{
				CheckCount: 3, PassedCount: 2, FailedCount: 1, ErrorCount: 1,
			},
			Checks: []doctor.Check{{ID: "tool.hz", OK: true, Severity: doctor.SeverityError, Message: "hz v0.9.7"}, {ID: "protolint.pio301.1", OK: false, Severity: doctor.SeverityError, Message: "transport envelope missing", Rule: "PIO301", File: "/repo/demo/idl/demo.proto", Line: 8}},
		}
	}
	defer func() { runDoctorReport = old }()

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runDoctor(cmd, &doctorOptions{root: "/repo/demo", output: "json"})
	if !errors.Is(err, errSilentFailure) {
		t.Fatalf("err = %v, want errSilentFailure", err)
	}
	var got struct {
		Root    string `json:"root"`
		Scope   string `json:"scope"`
		Summary struct {
			CheckCount int `json:"checkCount"`
			ErrorCount int `json:"errorCount"`
		} `json:"summary"`
		Checks []struct {
			ID   string `json:"id"`
			Rule string `json:"rule"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if got.Root != "/repo/demo" || got.Scope != string(doctor.ScopeService) {
		t.Fatalf("report header = %+v", got)
	}
	if got.Summary.CheckCount != 3 || got.Summary.ErrorCount != 1 {
		t.Fatalf("summary = %+v", got.Summary)
	}
	if len(got.Checks) != 2 || got.Checks[1].Rule != "PIO301" {
		t.Fatalf("checks = %+v", got.Checks)
	}
}

func TestRunDoctorSARIF(t *testing.T) {
	old := runDoctorReport
	runDoctorReport = func(context.Context, doctor.Options) *doctor.Report {
		return &doctor.Report{
			Root:  "/repo/demo",
			Scope: doctor.ScopeService,
			Summary: doctor.ReportSummary{
				CheckCount: 2, PassedCount: 1, FailedCount: 1, WarningCount: 1,
			},
			Checks: []doctor.Check{{ID: "tool.hz", OK: true, Severity: doctor.SeverityError, Message: "hz v0.9.7"}, {ID: "manifest.data.consistent", OK: false, Severity: doctor.SeverityWarn, Message: "template/data.json drift", File: "/repo/demo/template/data.json"}},
		}
	}
	defer func() { runDoctorReport = old }()

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runDoctor(cmd, &doctorOptions{root: "/repo/demo", output: "sarif"})
	if err != nil {
		t.Fatalf("runDoctor sarif: %v", err)
	}
	var got struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name string `json:"name"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid sarif json: %v\n%s", err, out.String())
	}
	if got.Schema == "" || got.Version != "2.1.0" || len(got.Runs) != 1 {
		t.Fatalf("sarif header = %+v", got)
	}
	if got.Runs[0].Tool.Driver.Name != "ncgo doctor" {
		t.Fatalf("driver.name = %q", got.Runs[0].Tool.Driver.Name)
	}
	if len(got.Runs[0].Results) != 1 || got.Runs[0].Results[0].RuleID != "manifest.data.consistent" || got.Runs[0].Results[0].Level != "warning" {
		t.Fatalf("results = %+v", got.Runs[0].Results)
	}
	if !strings.Contains(got.Runs[0].Results[0].Message.Text, "drift") {
		t.Fatalf("message = %+v", got.Runs[0].Results[0].Message)
	}
}

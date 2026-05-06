package protolint

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteSARIFIncludesRuleMetadataAndTaxonomy(t *testing.T) {
	res := &Result{
		Root:     "/repo/demo",
		Files:    []string{"invalid.proto"},
		RulesRun: []string{"PIO301", "PIO402"},
		OK:       false,
		Diagnostics: []Diagnostic{
			{RuleID: "PIO301", Level: LevelError, Phase: Phase1, File: "invalid.proto", Line: 12, Column: 1, Service: "Order", RPC: "GetOrder", Summary: "transport envelope detected", Hint: "remove envelope fields from the top-level response message"},
			{RuleID: "PIO402", Level: LevelWarning, Phase: Phase2, File: "invalid.proto", Line: 5, Column: 3, Service: "Order", RPC: "Search", Field: "keyword", Summary: "free-text field lacks PGV length constraints", Hint: "add max_len or another explicit string constraint"},
		},
	}

	var out bytes.Buffer
	if err := WriteSARIF(&out, res); err != nil {
		t.Fatalf("WriteSARIF: %v", err)
	}

	var got struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID                   string `json:"id"`
						HelpURI              string `json:"helpUri"`
						DefaultConfiguration struct {
							Level string `json:"level"`
						} `json:"defaultConfiguration"`
						Properties struct {
							Phase string   `json:"phase"`
							Group string   `json:"group"`
							Tags  []string `json:"tags"`
							Taxa  []string `json:"taxa"`
						} `json:"properties"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Taxonomies []struct {
				Name string `json:"name"`
				Taxa []struct {
					ID string `json:"id"`
				} `json:"taxa"`
			} `json:"taxonomies"`
			Results []struct {
				RuleID string `json:"ruleId"`
				Level  string `json:"level"`
				Taxa   []struct {
					ID string `json:"id"`
				} `json:"taxa"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid sarif json: %v\n%s", err, out.String())
	}
	if got.Schema == "" || got.Version != "2.1.0" || len(got.Runs) != 1 {
		t.Fatalf("sarif header = %+v", got)
	}
	run := got.Runs[0]
	if run.Tool.Driver.Name != "ncgo protolint" {
		t.Fatalf("driver.name = %q", run.Tool.Driver.Name)
	}
	if len(run.Taxonomies) != 1 || run.Taxonomies[0].Name != protolintSARIFTaxonomyName {
		t.Fatalf("taxonomies = %+v", run.Taxonomies)
	}
	if len(run.Taxonomies[0].Taxa) != 5 {
		t.Fatalf("taxonomy taxa = %+v", run.Taxonomies[0].Taxa)
	}
	if len(run.Tool.Driver.Rules) != 2 {
		t.Fatalf("rules = %+v", run.Tool.Driver.Rules)
	}
	if run.Tool.Driver.Rules[0].ID != "PIO301" || run.Tool.Driver.Rules[0].DefaultConfiguration.Level != "error" {
		t.Fatalf("rule[0] = %+v", run.Tool.Driver.Rules[0])
	}
	if run.Tool.Driver.Rules[0].Properties.Group != protolintTaxonKitexRPCContract || len(run.Tool.Driver.Rules[0].Properties.Taxa) != 1 {
		t.Fatalf("rule[0].properties = %+v", run.Tool.Driver.Rules[0].Properties)
	}
	if run.Tool.Driver.Rules[0].HelpURI == "" || len(run.Tool.Driver.Rules[0].Properties.Tags) == 0 {
		t.Fatalf("rule[0] metadata incomplete: %+v", run.Tool.Driver.Rules[0])
	}
	if run.Tool.Driver.Rules[1].ID != "PIO402" || run.Tool.Driver.Rules[1].DefaultConfiguration.Level != "warning" {
		t.Fatalf("rule[1] = %+v", run.Tool.Driver.Rules[1])
	}
	if run.Tool.Driver.Rules[1].Properties.Group != protolintTaxonPGVConstraints || run.Tool.Driver.Rules[1].Properties.Phase != string(Phase2) {
		t.Fatalf("rule[1].properties = %+v", run.Tool.Driver.Rules[1].Properties)
	}
	if len(run.Tool.Driver.Rules[1].Properties.Taxa) != 2 || run.Tool.Driver.Rules[1].Properties.Taxa[1] != protolintTaxonHeuristicGuidance {
		t.Fatalf("rule[1].taxa = %+v", run.Tool.Driver.Rules[1].Properties.Taxa)
	}
	if len(run.Results) != 2 {
		t.Fatalf("results = %+v", run.Results)
	}
	if run.Results[0].RuleID != "PIO301" || run.Results[0].Level != "error" || len(run.Results[0].Taxa) != 1 || run.Results[0].Taxa[0].ID != protolintTaxonKitexRPCContract {
		t.Fatalf("result[0] = %+v", run.Results[0])
	}
	if run.Results[1].RuleID != "PIO402" || run.Results[1].Level != "warning" || len(run.Results[1].Taxa) != 2 || run.Results[1].Taxa[1].ID != protolintTaxonHeuristicGuidance {
		t.Fatalf("result[1] = %+v", run.Results[1])
	}
}

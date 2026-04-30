package infra

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAddResultRenderer(t *testing.T) {
	res := &Result{
		WrittenPath:  "/project/internal/base/data/redis.go",
		WrittenPaths: []string{"/project/internal/base/data/redis.go"},
		WiredPaths:   []string{"/project/server.go"},
		NextSteps:    []string{"go get github.com/redis/go-redis/v9"},
		Plan:         []PlanItem{{Kind: "file", Action: "create", Path: "/project/internal/base/data/redis.go"}},
		Updated:      true,
		DryRun:       true,
	}

	text := FormatAddResultText(res)
	for _, want := range []string{
		"would write /project/internal/base/data/redis.go",
		"would wire /project/server.go",
		"dry-run: manifest would be updated",
		"  $ go get github.com/redis/go-redis/v9",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
	if strings.HasSuffix(text, "\n") {
		t.Fatalf("text should not end with newline: %q", text)
	}

	fields := AddResultFields(res)
	if fields["writtenPath"] != res.WrittenPath || fields["dryRun"] != true || fields["updated"] != true {
		t.Fatalf("fields missing stable values: %+v", fields)
	}

	var out bytes.Buffer
	if err := WriteAddResultJSON(&out, res); err != nil {
		t.Fatalf("WriteAddResultJSON: %v", err)
	}
	var got AddResultView
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, out.String())
	}
	if got.WrittenPath != res.WrittenPath || len(got.WrittenPaths) != 1 || len(got.Plan) != 1 {
		t.Fatalf("json view = %+v", got)
	}
}

package protolint

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckReqRespRulesOnValidProto(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "scaffold", "mono", "testdata", "mono-default", "idl"))
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"app/demo.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO101", "PIO102"}})
	if len(diags) != 0 {
		t.Fatalf("got %d diagnostics, want 0: %+v", len(diags), diags)
	}
}

func TestCheckReqRespRulesOnInvalidProto(t *testing.T) {
	root := filepath.Join("testdata", "reqresp")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO101", "PIO102"}})
	if len(diags) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO101", LevelError, Phase1, "Demo", "Ping")
	assertDiag(t, diags[1], "PIO102", LevelError, Phase1, "Demo", "Ping")
	assertDiag(t, diags[2], "PIO101", LevelError, Phase1, "Demo", "Pong")
	assertDiag(t, diags[3], "PIO102", LevelError, Phase1, "Demo", "Pong")

	if got := diags[1].Message; got != "PingRequest" {
		t.Fatalf("diag[1].Message = %q, want PingRequest", got)
	}
	if got := diags[3].Message; got != "Result" {
		t.Fatalf("diag[3].Message = %q, want Result", got)
	}
}

func TestCheckFiltersRulesAndPhases(t *testing.T) {
	root := filepath.Join("testdata", "reqresp")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO102"}, Phases: []Phase{Phase1}, Levels: []Level{LevelError}})
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(diags), diags)
	}
	for _, d := range diags {
		if d.RuleID != "PIO102" {
			t.Fatalf("got rule %s, want PIO102", d.RuleID)
		}
	}
}

func TestCheckPIO103OnDynamicTopLevelIO(t *testing.T) {
	root := filepath.Join("testdata", "dynamicio")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO103"}})
	if len(diags) != 3 {
		t.Fatalf("got %d diagnostics, want 3: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO103", LevelError, Phase1, "Demo", "Search")
	assertDiag(t, diags[1], "PIO103", LevelError, Phase1, "Demo", "Put")
	assertDiag(t, diags[2], "PIO103", LevelError, Phase1, "Demo", "Echo")
	if diags[0].Message != "google.protobuf.Struct" {
		t.Fatalf("got %q, want google.protobuf.Struct", diags[0].Message)
	}
	if diags[1].Message != "google.protobuf.Any" {
		t.Fatalf("got %q, want google.protobuf.Any", diags[1].Message)
	}
	if diags[2].Message != "google.protobuf.Value" {
		t.Fatalf("got %q, want google.protobuf.Value", diags[2].Message)
	}
}

func TestCheckPIO201OnInvalidHertzHTTPMethods(t *testing.T) {
	root := filepath.Join("testdata", "httpmethods")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO201"}})
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO201", LevelError, Phase1, "Demo", "Ping")
	assertDiag(t, diags[1], "PIO201", LevelError, Phase1, "Demo", "Pong")
}

func TestCheckPIO202OnMismatchedPathParams(t *testing.T) {
	root := filepath.Join("testdata", "pathparams")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO202"}})
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO202", LevelError, Phase1, "Demo", "GetUser")
	assertDiag(t, diags[1], "PIO202", LevelError, Phase1, "Demo", "ListUsers")
}

func TestCheckPIO203OnBodyBindingsForDisallowedMethods(t *testing.T) {
	root := filepath.Join("testdata", "httpbody")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO203"}})
	if len(diags) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO203", LevelError, Phase1, "Demo", "GetItem")
	assertDiag(t, diags[1], "PIO203", LevelError, Phase1, "Demo", "DeleteItem")
	assertDiag(t, diags[2], "PIO203", LevelError, Phase1, "Demo", "HeadItem")
	assertDiag(t, diags[3], "PIO203", LevelError, Phase1, "Demo", "HeadItem")
	if diags[0].Field != "payload" {
		t.Fatalf("diag[0].Field = %q, want payload", diags[0].Field)
	}
	if diags[1].Field != "raw" {
		t.Fatalf("diag[1].Field = %q, want raw", diags[1].Field)
	}
}

func TestCheckPIO204OnMultipleBindings(t *testing.T) {
	root := filepath.Join("testdata", "multibinding")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO204"}})
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO204", LevelError, Phase1, "Demo", "Find")
	assertDiag(t, diags[1], "PIO204", LevelError, Phase1, "Demo", "Echo")
	if diags[0].Field != "user_id" {
		t.Fatalf("diag[0].Field = %q, want user_id", diags[0].Field)
	}
	if diags[1].Field != "payload" {
		t.Fatalf("diag[1].Field = %q, want payload", diags[1].Field)
	}
}

func TestCheckPIO205OnMultipleRawBodyFields(t *testing.T) {
	root := filepath.Join("testdata", "rawbody")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO205"}})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO205", LevelError, Phase1, "Demo", "Upload")
	if diags[0].Message != "UploadReq" {
		t.Fatalf("diag[0].Message = %q, want UploadReq", diags[0].Message)
	}
}

func TestCheckPIO206OnResponseBindingAnnotations(t *testing.T) {
	root := filepath.Join("testdata", "responsebindings")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO206"}})
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO206", LevelError, Phase1, "Demo", "Get")
	assertDiag(t, diags[1], "PIO206", LevelError, Phase1, "Demo", "Get")
	if diags[0].Message != "GetResp" {
		t.Fatalf("diag[0].Message = %q, want GetResp", diags[0].Message)
	}
	if diags[0].Field != "message" {
		t.Fatalf("diag[0].Field = %q, want message", diags[0].Field)
	}
	if diags[1].Field != "trace" {
		t.Fatalf("diag[1].Field = %q, want trace", diags[1].Field)
	}
}

func TestCheckPIO301OnKitexTransportEnvelopeResponse(t *testing.T) {
	root := filepath.Join("testdata", "kitexenvelope")
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO301"}})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO301", LevelError, Phase1, "Demo", "GetUser")
	if diags[0].Message != "GetUserResp" {
		t.Fatalf("diag[0].Message = %q, want GetUserResp", diags[0].Message)
	}
}

func TestCheckPIO111OnGoogleProtobufEmptyTopLevelIO(t *testing.T) {
	root := filepath.Join("testdata", "phase2warnings")
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"invalid.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO111"}})
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO111", LevelWarning, Phase2, "Demo", "Health")
	assertDiag(t, diags[1], "PIO111", LevelWarning, Phase2, "Demo", "Ping")
	if diags[0].Message != "google.protobuf.Empty" || diags[1].Message != "google.protobuf.Empty" {
		t.Fatalf("messages = %q / %q, want google.protobuf.Empty", diags[0].Message, diags[1].Message)
	}
}

func TestCheckPIO112OnGenericTopLevelMessages(t *testing.T) {
	root := filepath.Join("testdata", "phase2warnings")
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"invalid.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO112"}})
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO112", LevelWarning, Phase2, "Demo", "GetUser")
	assertDiag(t, diags[1], "PIO112", LevelWarning, Phase2, "Demo", "Search")
	if diags[0].Message != "CommonReq" {
		t.Fatalf("diag[0].Message = %q, want CommonReq", diags[0].Message)
	}
	if diags[1].Message != "Result" {
		t.Fatalf("diag[1].Message = %q, want Result", diags[1].Message)
	}
}

func TestCheckPIO113OnLargeRequestMessages(t *testing.T) {
	root := filepath.Join("testdata", "phase2warnings")
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"invalid.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO113"}})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO113", LevelWarning, Phase2, "Demo", "Search")
	if diags[0].Message != "SearchReq" {
		t.Fatalf("diag[0].Message = %q, want SearchReq", diags[0].Message)
	}
}

func TestRunWarningRulesDoNotFailResult(t *testing.T) {
	root := filepath.Join("testdata", "phase2warnings")
	res, err := Run(context.Background(), RunOptions{
		Root:    root,
		Files:   []string{"invalid.proto"},
		RuleIDs: []string{"PIO111", "PIO112", "PIO113"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.OK {
		t.Fatalf("warning-only run should be OK: %+v", res)
	}
	if res.Summary.DiagnosticsCount != 5 || res.Summary.ErrorCount != 0 || res.Summary.WarningCount != 5 {
		t.Fatalf("summary = %+v, want diagnostics=5 errors=0 warnings=5", res.Summary)
	}
}

func TestCheckPIO211OnUnboundHertzRequestFields(t *testing.T) {
	root := filepath.Join("testdata", "unboundfields")
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"invalid.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO211"}})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO211", LevelWarning, Phase2, "Demo", "List")
	if diags[0].Message != "ListReq" {
		t.Fatalf("diag[0].Message = %q, want ListReq", diags[0].Message)
	}
	if diags[0].Field != "limit" {
		t.Fatalf("diag[0].Field = %q, want limit", diags[0].Field)
	}
}

func TestCheckPIO212OnValidHertzOpenAPIMetadata(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "scaffold", "mono", "testdata", "mono-default", "idl"))
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"app/demo.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO212"}})
	if len(diags) != 0 {
		t.Fatalf("got %d diagnostics, want 0: %+v", len(diags), diags)
	}
}

func TestCheckPIO212OnMissingHertzOpenAPIMetadata(t *testing.T) {
	root := filepath.Join("testdata", "openapimissing")
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"invalid.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO212"}})
	if len(diags) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO212", LevelWarning, Phase2, "Demo", "List")
	assertDiag(t, diags[1], "PIO212", LevelWarning, Phase2, "Demo", "List")
	assertDiag(t, diags[2], "PIO212", LevelWarning, Phase2, "Demo", "List")
	assertDiag(t, diags[3], "PIO212", LevelWarning, Phase2, "Demo", "Create")
	if diags[0].Message != "List" {
		t.Fatalf("diag[0].Message = %q, want List", diags[0].Message)
	}
	if diags[1].Message != "ListReq" || diags[1].Field != "keyword" {
		t.Fatalf("diag[1] = %+v, want ListReq.keyword", diags[1])
	}
	if diags[2].Message != "ListResp" {
		t.Fatalf("diag[2].Message = %q, want ListResp", diags[2].Message)
	}
	if diags[3].Message != "CreateResp" {
		t.Fatalf("diag[3].Message = %q, want CreateResp", diags[3].Message)
	}
}

func TestCheckPIO302OnKitexListSearchQueryWithoutPagination(t *testing.T) {
	root := filepath.Join("testdata", "paginationmissing")
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"invalid.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO302"}})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO302", LevelWarning, Phase2, "Demo", "ListUsers")
	if diags[0].Message != "ListUsersReq" {
		t.Fatalf("diag[0].Message = %q, want ListUsersReq", diags[0].Message)
	}
}

func TestCheckPIO401OnPaginationFieldsMissingPGVRange(t *testing.T) {
	root := filepath.Join("testdata", "pgvpagination")
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"invalid.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO401"}})
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO401", LevelWarning, Phase2, "Demo", "ListUsers")
	assertDiag(t, diags[1], "PIO401", LevelWarning, Phase2, "Demo", "GetUser")
	if diags[0].Message != "ListUsersReq" || diags[0].Field != "limit" {
		t.Fatalf("diag[0] = %+v, want ListUsersReq.limit", diags[0])
	}
	if diags[1].Message != "GetUserReq" || diags[1].Field != "page" {
		t.Fatalf("diag[1] = %+v, want GetUserReq.page", diags[1])
	}
}

func TestCheckPIO303OnUniversalKitexRequest(t *testing.T) {
	root := filepath.Join("testdata", "universalrequest")
	model, err := Load(context.Background(), LoadOptions{Root: root, Files: []string{"invalid.proto"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := Check(model, CheckOptions{RuleIDs: []string{"PIO303"}})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	assertDiag(t, diags[0], "PIO303", LevelWarning, Phase2, "Demo", "SearchUsers")
	if diags[0].Message != "SearchUsersReq" {
		t.Fatalf("diag[0].Message = %q, want SearchUsersReq", diags[0].Message)
	}
	if !strings.Contains(diags[0].Summary, "filter") || !strings.Contains(diags[0].Summary, "pagination") {
		t.Fatalf("summary = %q, want concern categories", diags[0].Summary)
	}
}

func assertDiag(t *testing.T, d Diagnostic, ruleID string, level Level, phase Phase, service, rpc string) {
	t.Helper()
	if d.RuleID != ruleID {
		t.Fatalf("RuleID = %q, want %q", d.RuleID, ruleID)
	}
	if d.Level != level {
		t.Fatalf("Level = %q, want %q", d.Level, level)
	}
	if d.Phase != phase {
		t.Fatalf("Phase = %q, want %q", d.Phase, phase)
	}
	if d.Service != service {
		t.Fatalf("Service = %q, want %q", d.Service, service)
	}
	if d.RPC != rpc {
		t.Fatalf("RPC = %q, want %q", d.RPC, rpc)
	}
	if d.File == "" || d.Line <= 0 {
		t.Fatalf("diagnostic location = file:%q line:%d, want populated", d.File, d.Line)
	}
	if d.Summary == "" {
		t.Fatalf("Summary should not be empty")
	}
}

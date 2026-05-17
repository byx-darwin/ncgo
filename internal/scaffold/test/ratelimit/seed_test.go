package ratelimit

import (
	"strings"
	"testing"
)

func TestBuildSeedSQLBasic(t *testing.T) {
	sql := buildSeedSQL("my-service", 10, 60)

	// Should contain DELETE
	if !strings.Contains(sql, "DELETE FROM rate_limit_rules WHERE service = 'my-service'") {
		t.Errorf("expected DELETE for my-service, got:\n%s", sql)
	}

	// Should contain wildcard rule
	if !strings.Contains(sql, "'*', 'exact', '*', '*'") {
		t.Errorf("expected wildcard rule, got:\n%s", sql)
	}

	// Should contain healthz rule (path_pattern is empty for exact match)
	if !strings.Contains(sql, "'GET', 'exact', '/healthz', ''") {
		t.Errorf("expected healthz rule, got:\n%s", sql)
	}

	// Should have exactly 3 INSERT value rows (pre_auth x2 + grpc x1)
	rows := strings.Count(sql, "fixed_window")
	if rows != 3 {
		t.Errorf("expected 3 INSERT rows, found %d", rows)
	}
}

func TestBuildSeedSQLContainsGRPCRule(t *testing.T) {
	sql := buildSeedSQL("my-service", 10, 60)
	if !strings.Contains(sql, "'my-service', 'grpc', '*', 'exact', '*', '*'") {
		t.Errorf("expected grpc rule for my-service, got:\n%s", sql)
	}
}

func TestBuildSeedSQLSanitizesServiceName(t *testing.T) {
	sql := buildSeedSQL("it's-a-svc", 5, 30)
	// Single quotes should be escaped
	if !strings.Contains(sql, "'it''s-a-svc'") {
		t.Errorf("expected sanitized service name, got:\n%s", sql)
	}
}

func TestBuildSeedSQLParameters(t *testing.T) {
	tests := []struct {
		service     string
		maxRequests int
		windowSecs  int
		wantParams  string
	}{
		{"svc", 10, 60, "60, 10"},
		{"svc", 100, 120, "120, 100"},
	}
	for _, tt := range tests {
		t.Run(tt.service, func(t *testing.T) {
			sql := buildSeedSQL(tt.service, tt.maxRequests, tt.windowSecs)
			if !strings.Contains(sql, tt.wantParams) {
				t.Errorf("expected params %q in:\n%s", tt.wantParams, sql)
			}
		})
	}
}

func TestSanitizeSQLString(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"normal", "normal"},
		{"it's", "it''s"},
		{"a'b'c", "a''b''c"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := sanitizeSQLString(tt.input); got != tt.want {
			t.Errorf("sanitizeSQLString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

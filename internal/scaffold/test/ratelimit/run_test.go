package ratelimit

import (
	"testing"
)

func TestBuildTargets(t *testing.T) {
	tests := []struct {
		host  string
		port  int
		paths []string
		want  []string
	}{
		{
			host:  "localhost",
			port:  8080,
			paths: []string{"/healthz", "/"},
			want:  []string{"GET http://localhost:8080/healthz", "GET http://localhost:8080/"},
		},
		{
			host:  "example.com",
			port:  3000,
			paths: []string{"/api/v1"},
			want:  []string{"GET http://example.com:3000/api/v1"},
		},
		{
			host:  "localhost",
			port:  8080,
			paths: []string{},
			want:  nil,
		},
	}
	for _, tt := range tests {
		got := buildTargets(tt.host, tt.port, tt.paths)
		if len(got) != len(tt.want) {
			t.Errorf("buildTargets(%s, %d, %v) = %d targets, want %d",
				tt.host, tt.port, tt.paths, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("buildTargets()[%d] = %q, want %q", i, got[i], tt.want[i])
			}
		}
	}
}

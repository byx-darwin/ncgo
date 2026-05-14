package template

import (
	"testing"
)

func TestDefaultServiceName(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"idl/app/user-api.proto", "UserApi"},
		{"idl/userrpc.proto", "Userrpc"},
		{"idl/svc.proto", "Svc"},
	}
	for _, tt := range tests {
		if got := defaultServiceName(tt.path); got != tt.want {
			t.Errorf("defaultServiceName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestPkgRefName(t *testing.T) {
	tests := []struct {
		module, want string
	}{
		{"github.com/acme/user-api", "user-api"},
		{"github.com/acme/commerce/services/user-rpc", "user-rpc"},
	}
	for _, tt := range tests {
		if got := pkgRefName(tt.module); got != tt.want {
			t.Errorf("pkgRefName(%q) = %q, want %q", tt.module, got, tt.want)
		}
	}
}

// ParseAllServices tests require the protobuf compiler with standard imports
// available. The integration-level export/apply tests cover the full flow.

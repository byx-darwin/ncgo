package rulecenter

import (
	"strings"
	"testing"
)

func TestAddRequiresAddr(t *testing.T) {
	_, err := Add(Options{Addr: ""})
	if err == nil {
		t.Fatal("expected error for missing addr")
	}
	if !strings.Contains(err.Error(), "addr is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

package ai

import (
	"strings"
	"testing"
)

func TestErrorCodesHertz(t *testing.T) {
	got := ErrorCodes("hertz")
	if got == "" {
		t.Fatal("ErrorCodes(hertz) returned empty")
	}
	for _, want := range []string{"10000", "10001", "10002", "40100"} {
		if !strings.Contains(got, want) {
			t.Errorf("ErrorCodes(hertz) missing code %s:\n%s", want, got)
		}
	}
}

func TestErrorCodesKitex(t *testing.T) {
	got := ErrorCodes("kitex")
	if got == "" {
		t.Fatal("ErrorCodes(kitex) returned empty")
	}
	for _, want := range []string{"10000", "10001", "10002", "40100"} {
		if !strings.Contains(got, want) {
			t.Errorf("ErrorCodes(kitex) missing code %s:\n%s", want, got)
		}
	}
}

func TestErrorCodesUnknown(t *testing.T) {
	got := ErrorCodes("unknown")
	if got == "" {
		t.Fatal("ErrorCodes(unknown) returned empty")
	}
	if !strings.Contains(got, "10000") {
		t.Errorf("ErrorCodes(unknown) missing CodeSystem:\n%s", got)
	}
}

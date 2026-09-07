package ai

import (
	"strings"
	"testing"
)

func TestMergeCustomAnchorsNoAnchors(t *testing.T) {
	old := "<!-- ncgo:managed -->\nold body, no anchors\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	merged, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) != 0 {
		t.Fatalf("expected no malformed anchors, got %v", malformed)
	}
	if merged != rendered {
		t.Errorf("expected rendered content unchanged, got %q", merged)
	}
}

func TestMergeCustomAnchorsSingleAnchor(t *testing.T) {
	old := "<!-- ncgo:managed -->\n" +
		"## Local Docker Development\n\n" +
		"<!-- ncgo:custom:docker-dev:start -->\n" +
		"docker compose up -d\n" +
		"<!-- ncgo:custom:docker-dev:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	merged, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) != 0 {
		t.Fatalf("expected no malformed anchors, got %v", malformed)
	}
	if !strings.Contains(merged, "## Custom Notes") {
		t.Errorf("expected Custom Notes section, got %q", merged)
	}
	if !strings.Contains(merged, "docker compose up -d") {
		t.Errorf("expected preserved anchor body, got %q", merged)
	}
	if !strings.Contains(merged, "<!-- ncgo:custom:docker-dev:start -->") ||
		!strings.Contains(merged, "<!-- ncgo:custom:docker-dev:end -->") {
		t.Errorf("expected preserved anchor markers, got %q", merged)
	}
}

func TestMergeCustomAnchorsMultipleNamedAnchorsPreserveOrder(t *testing.T) {
	old := "<!-- ncgo:managed -->\n" +
		"<!-- ncgo:custom:first:start -->\nfirst body\n<!-- ncgo:custom:first:end -->\n" +
		"<!-- ncgo:custom:second:start -->\nsecond body\n<!-- ncgo:custom:second:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	merged, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) != 0 {
		t.Fatalf("expected no malformed anchors, got %v", malformed)
	}
	firstIdx := strings.Index(merged, "first body")
	secondIdx := strings.Index(merged, "second body")
	if firstIdx == -1 || secondIdx == -1 || firstIdx > secondIdx {
		t.Errorf("expected first before second in merged output, got %q", merged)
	}
}

func TestMergeCustomAnchorsMissingEndIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n<!-- ncgo:custom:oops:start -->\nunterminated\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for missing end marker")
	}
	if !strings.Contains(malformed[0], "oops") {
		t.Errorf("expected malformed reason to name the anchor, got %v", malformed)
	}
}

func TestMergeCustomAnchorsEndWithoutStartIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n<!-- ncgo:custom:oops:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for end without start")
	}
}

func TestMergeCustomAnchorsDuplicateNameIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n" +
		"<!-- ncgo:custom:dup:start -->\na\n<!-- ncgo:custom:dup:end -->\n" +
		"<!-- ncgo:custom:dup:start -->\nb\n<!-- ncgo:custom:dup:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for duplicate name")
	}
}

func TestMergeCustomAnchorsInvalidNameIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n<!-- ncgo:custom:Bad_Name:start -->\nx\n<!-- ncgo:custom:Bad_Name:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for invalid name")
	}
}

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

func TestMergeCustomAnchorsNestedStartIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n<!-- ncgo:custom:outer:start -->\nouter body\n<!-- ncgo:custom:inner:start -->\ninner body\n<!-- ncgo:custom:inner:end -->\n<!-- ncgo:custom:outer:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for nested start")
	}
	if !strings.Contains(malformed[0], "inner") {
		t.Errorf("expected malformed reason to name the nested anchor, got %v", malformed)
	}
}

func TestMergeCustomAnchorsSkipsAnchorAlreadyInRendered(t *testing.T) {
	// Reproduces the AGENTS.local.md + anchor oscillation: an anchor named
	// "foo" was previously inlined into the old file via "## Local Notes"
	// (because AGENTS.local.md contained it verbatim), and the freshly
	// rendered output ALSO contains it again via a fresh read of
	// AGENTS.local.md. mergeCustomAnchors must not re-append it under a new
	// "## Custom Notes" section, since it's already present in rendered.
	old := "<!-- ncgo:managed -->\n" +
		"## Local Notes\n\n" +
		"<!-- ncgo:custom:foo:start -->\n" +
		"foo body\n" +
		"<!-- ncgo:custom:foo:end -->\n"
	rendered := "<!-- ncgo:managed -->\n" +
		"new body\n\n" +
		"## Local Notes\n\n" +
		"<!-- ncgo:custom:foo:start -->\n" +
		"foo body\n" +
		"<!-- ncgo:custom:foo:end -->\n"
	merged, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) != 0 {
		t.Fatalf("expected no malformed anchors, got %v", malformed)
	}
	if strings.Contains(merged, "## Custom Notes") {
		t.Errorf("expected no Custom Notes section since anchor already present in rendered, got %q", merged)
	}
	if n := strings.Count(merged, "<!-- ncgo:custom:foo:start -->"); n != 1 {
		t.Errorf("expected anchor foo to appear exactly once, got %d occurrences in %q", n, merged)
	}
	if merged != rendered {
		t.Errorf("expected merged output unchanged from rendered, got %q", merged)
	}
}

func TestMergeCustomAnchorsOnlyAppendsAnchorsNotAlreadyInRendered(t *testing.T) {
	// "foo" already appears in rendered (e.g. via Local Notes) and must not
	// be duplicated, but "bar" is genuinely new and must still be preserved
	// under "## Custom Notes".
	old := "<!-- ncgo:managed -->\n" +
		"<!-- ncgo:custom:foo:start -->\nfoo body\n<!-- ncgo:custom:foo:end -->\n" +
		"<!-- ncgo:custom:bar:start -->\nbar body\n<!-- ncgo:custom:bar:end -->\n"
	rendered := "<!-- ncgo:managed -->\n" +
		"new body\n\n" +
		"<!-- ncgo:custom:foo:start -->\nfoo body\n<!-- ncgo:custom:foo:end -->\n"
	merged, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) != 0 {
		t.Fatalf("expected no malformed anchors, got %v", malformed)
	}
	if n := strings.Count(merged, "<!-- ncgo:custom:foo:start -->"); n != 1 {
		t.Errorf("expected anchor foo to appear exactly once, got %d occurrences in %q", n, merged)
	}
	if !strings.Contains(merged, "## Custom Notes") || !strings.Contains(merged, "bar body") {
		t.Errorf("expected bar to be preserved under Custom Notes, got %q", merged)
	}
}

func TestMergeCustomAnchorsEndNameMismatchIsMalformed(t *testing.T) {
	old := "<!-- ncgo:managed -->\n<!-- ncgo:custom:foo:start -->\nbody\n<!-- ncgo:custom:bar:end -->\n"
	rendered := "<!-- ncgo:managed -->\nnew body\n"
	_, malformed := mergeCustomAnchors([]byte(old), []byte(rendered))
	if len(malformed) == 0 {
		t.Fatal("expected malformed anchor for end/start name mismatch")
	}
	if !strings.Contains(malformed[0], "bar") {
		t.Errorf("expected malformed reason to name the mismatched end marker, got %v", malformed)
	}
}

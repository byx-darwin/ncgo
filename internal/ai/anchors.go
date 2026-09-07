package ai

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	customAnchorNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	customAnchorLineRE = regexp.MustCompile(`^<!-- ncgo:custom:([^:]+):(start|end) -->$`)
)

type customAnchor struct {
	name  string
	block string // full text including its start/end marker lines
}

// mergeCustomAnchors extracts well-formed `<!-- ncgo:custom:<name>:start -->`
// ... `<!-- ncgo:custom:<name>:end -->` blocks from oldContent and appends
// them, in first-seen order, under a new "## Custom Notes" section at the
// end of rendered. Anchors whose name is already present in rendered (for
// example because AGENTS.local.md's content — including its own anchor
// markers — was inlined verbatim under "## Local Notes") are skipped, since
// they're already there and re-appending them would duplicate the name and
// eventually make extractCustomAnchors flag the file as malformed. When at
// least one malformed anchor is found in oldContent (missing end marker, end
// without a matching start, invalid name, duplicate name), merged is "" and
// malformed carries a human-readable reason per problem — callers must
// refuse to overwrite rather than merge, mirroring the existing "file exists
// without ncgo:managed marker" skip semantics.
func mergeCustomAnchors(oldContent, rendered []byte) (merged string, malformed []string) {
	anchors, malformed := extractCustomAnchors(string(oldContent))
	if len(malformed) > 0 {
		return "", malformed
	}
	if len(anchors) == 0 {
		return string(rendered), nil
	}
	renderedAnchors, _ := extractCustomAnchors(string(rendered))
	renderedNames := make(map[string]bool, len(renderedAnchors))
	for _, a := range renderedAnchors {
		renderedNames[a.name] = true
	}
	kept := anchors[:0:0]
	for _, a := range anchors {
		if renderedNames[a.name] {
			continue
		}
		kept = append(kept, a)
	}
	anchors = kept
	if len(anchors) == 0 {
		return string(rendered), nil
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(string(rendered), "\n"))
	b.WriteString("\n\n## Custom Notes\n\n")
	for i, a := range anchors {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(a.block)
	}
	b.WriteString("\n")
	return b.String(), nil
}

// extractCustomAnchors scans content line by line for ncgo:custom marker
// pairs, returning them in first-seen order. malformed reports an
// unterminated start, an end without a matching start, an invalid name, or
// a name reused across more than one anchor.
func extractCustomAnchors(content string) (anchors []customAnchor, malformed []string) {
	lines := strings.Split(content, "\n")
	seen := map[string]bool{}
	openName := ""
	openStart := 0
	for i, line := range lines {
		m := customAnchorLineRE.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		name, kind := m[1], m[2]
		if kind == "start" {
			if openName != "" {
				malformed = append(malformed, fmt.Sprintf("anchor %q starts before anchor %q ends (line %d)", name, openName, i+1))
				continue
			}
			if !customAnchorNameRE.MatchString(name) {
				malformed = append(malformed, fmt.Sprintf("anchor name %q is invalid (line %d)", name, i+1))
				continue
			}
			if seen[name] {
				malformed = append(malformed, fmt.Sprintf("anchor %q appears more than once", name))
				continue
			}
			openName, openStart = name, i
			continue
		}
		// kind == "end"
		if openName == "" || openName != name {
			malformed = append(malformed, fmt.Sprintf("anchor %q end marker has no matching start (line %d)", name, i+1))
			continue
		}
		anchors = append(anchors, customAnchor{name: name, block: strings.Join(lines[openStart:i+1], "\n")})
		seen[name] = true
		openName = ""
	}
	if openName != "" {
		malformed = append(malformed, fmt.Sprintf("anchor %q is missing its end marker", openName))
	}
	return anchors, malformed
}

package ai

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/byx-darwin/ncgo/internal/scan"
)

// ContextState describes whether one enabled Agent context target agrees with
// the current project metadata and embedded workflow assets.
type ContextState string

const (
	ContextCurrent   ContextState = "current"
	ContextMissing   ContextState = "missing"
	ContextUnmanaged ContextState = "unmanaged"
	ContextStale     ContextState = "stale"
	ContextUnsafe    ContextState = "unsafe"
)

// ContextAudit is the machine-readable status of one enabled Agent context
// file. All target groups are enabled for generated projects; --target only
// narrows an explicit sync invocation and does not disable the other groups.
type ContextAudit struct {
	Group  string       `json:"group"`
	Path   string       `json:"path"`
	State  ContextState `json:"state"`
	Reason string       `json:"reason,omitempty"`
}

// AuditContexts validates every enabled Agent context file without writing.
// Managed files are compared with a freshly rendered result while ignoring
// only the generated-at timestamp and preserving well-formed custom anchors.
func AuditContexts(root, lang string) ([]ContextAudit, error) {
	if root == "" {
		root = "."
	}
	if lang != "" && lang != LangEN && lang != LangZhCN {
		return nil, fmt.Errorf("ai audit: language %q is invalid (en|zh-CN)", lang)
	}
	local, err := readLocalNotes(root)
	if err != nil {
		return nil, err
	}
	languages := []string{lang}
	if lang == "" {
		languages = []string{LangEN, LangZhCN}
	}
	methods := methodsFromScan(root)
	inputVariants := make([]renderInputs, 0, len(languages))
	for _, candidateLang := range languages {
		source, err := resolveSyncSource(root, candidateLang)
		if err != nil {
			return nil, err
		}
		inputVariants = append(inputVariants, renderInputsForSource(source, root, local, candidateLang, methods))
	}

	audits := make([]ContextAudit, 0, len(targets()))
	for _, target := range targets() {
		audit := ContextAudit{Group: target.Group, Path: target.RelPath}
		full, unsafe, err := safeWritePath(root, target.RelPath)
		if err != nil {
			return nil, fmt.Errorf("ai audit: resolve %s: %w", target.RelPath, err)
		}
		if unsafe != "" {
			audit.State = ContextUnsafe
			audit.Reason = unsafe
			audits = append(audits, audit)
			continue
		}
		existing, err := os.ReadFile(full)
		if errors.Is(err, fs.ErrNotExist) {
			audit.State = ContextMissing
			audit.Reason = "enabled context file is missing"
			audits = append(audits, audit)
			continue
		}
		if err != nil {
			audit.State = ContextStale
			audit.Reason = "read context file: " + err.Error()
			audits = append(audits, audit)
			continue
		}
		if !isManaged(existing) {
			audit.State = ContextUnmanaged
			audit.Reason = "path is user-owned because it has no ncgo:managed marker"
			audits = append(audits, audit)
			continue
		}

		current := false
		var malformed []string
		for _, inputs := range inputVariants {
			expected, problems := mergeCustomAnchors(existing, []byte(target.Render(inputs)))
			if len(problems) > 0 {
				malformed = problems
				break
			}
			if bytes.Equal(stripGeneratedAt(existing), stripGeneratedAt([]byte(expected))) {
				current = true
				break
			}
		}
		if len(malformed) > 0 {
			audit.State = ContextStale
			audit.Reason = "malformed ncgo:custom anchor(s): " + strings.Join(malformed, "; ")
		} else if current {
			audit.State = ContextCurrent
		} else {
			audit.State = ContextStale
			audit.Reason = "managed content differs from the current project facts or workflow assets"
		}
		audits = append(audits, audit)
	}
	return audits, nil
}

func stripGeneratedAt(content []byte) []byte {
	lines := bytes.Split(content, []byte("\n"))
	out := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(string(line)), scan.GeneratedAtMarker) {
			continue
		}
		out = append(out, line)
	}
	return bytes.Join(out, []byte("\n"))
}

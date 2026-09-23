package doctor

import (
	"fmt"
	"path/filepath"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scan"
)

// RunCheck validates AI context integrity and manifest consistency for the
// ncgo service or micro workspace rooted at root. Service checks verify
// method anchors and manifest consistency; both scopes audit every enabled
// Agent context target. It returns an error only when root is neither a valid
// service nor a valid workspace.
func RunCheck(root string) (*Report, error) {
	rep := &Report{Root: root}
	if _, manifestErr := manifest.Load(root); manifestErr == nil {
		s, err := scan.Scan(root)
		if err != nil {
			return nil, err
		}
		rep.Scope = ScopeService
		rep.Checks = append(rep.Checks, checkAnchors(s)...)
		rep.Checks = append(rep.Checks, checkConsistency(s)...)
	} else {
		if _, workspaceErr := manifest.LoadWorkspace(root); workspaceErr != nil {
			return nil, manifestErr
		}
		rep.Scope = ScopeWorkspace
	}
	contexts, err := ai.AuditContexts(root, "")
	if err != nil {
		return nil, err
	}
	rep.Checks = append(rep.Checks, contextChecks(root, contexts)...)
	rep.Summary = Summarize(rep.Checks)
	return rep, nil
}

func checkAnchors(s *scan.ScanResult) []Check {
	var out []Check
	bad := 0
	for _, d := range s.Domains {
		if d.UsecaseExists && !d.AnchorsOK {
			bad++
			out = append(out, Check{
				ID: "check.anchor", OK: false, Severity: SeverityError,
				Message: fmt.Sprintf("domain %s has unpaired method anchors", d.Name),
				Hint:    "run `ncgo add method <domain>.X` or fix the // ncgo:methods:start|end markers",
			})
		}
	}
	if bad == 0 {
		out = append(out, Check{
			ID: "check.anchor", OK: true, Severity: SeverityError,
			Message: "all usecase files have paired method anchors",
		})
	}
	return out
}

func checkConsistency(s *scan.ScanResult) []Check {
	var out []Check
	bad := 0
	for _, i := range s.Issues {
		if i.Kind != scan.IssueMissingUsecase && i.Kind != scan.IssueUndeclaredDomain {
			continue
		}
		bad++
		out = append(out, Check{
			ID: "check.manifest.consistency", OK: false, Severity: SeverityError,
			Message: i.Message, File: i.File,
		})
	}
	if bad == 0 {
		out = append(out, Check{
			ID: "check.manifest.consistency", OK: true, Severity: SeverityError,
			Message: "manifest domains match internal/usecase/*/ directories",
		})
	}
	return out
}

func contextChecks(root string, audits []ai.ContextAudit) []Check {
	out := make([]Check, 0, len(audits))
	syncHint := fmt.Sprintf("run `ncgo ai sync --target all --root %q`", root)
	for _, audit := range audits {
		path := filepath.Join(root, filepath.FromSlash(audit.Path))
		check := Check{File: path, Severity: SeverityError}
		switch audit.State {
		case ai.ContextCurrent:
			check.ID = "check.context.current"
			check.OK = true
			check.Message = fmt.Sprintf("%s context is current: %s", audit.Group, audit.Path)
		case ai.ContextMissing:
			check.ID = "check.context.missing"
			check.Severity = SeverityWarn
			check.Message = fmt.Sprintf("enabled %s context is missing: %s", audit.Group, audit.Path)
			check.Hint = syncHint
		case ai.ContextUnmanaged:
			check.ID = "check.context.unmanaged"
			check.Severity = SeverityWarn
			check.Message = fmt.Sprintf("enabled %s path is user-owned and was not validated: %s", audit.Group, audit.Path)
			check.Hint = fmt.Sprintf("preserve the file or explicitly replace it with `ncgo ai sync --target all --root %q --force`", root)
		case ai.ContextUnsafe:
			check.ID = "check.context.unsafe"
			check.Message = fmt.Sprintf("enabled %s context path is unsafe: %s (%s)", audit.Group, audit.Path, audit.Reason)
			check.Hint = "remove or retarget the escaping symlink, then " + syncHint
		default:
			check.ID = "check.context.stale"
			check.Message = fmt.Sprintf("enabled %s context is stale: %s (%s)", audit.Group, audit.Path, audit.Reason)
			check.Hint = syncHint
		}
		out = append(out, check)
	}
	return out
}

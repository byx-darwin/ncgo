package doctor

// Static layer-rule scanner (PRD §8). Each rule walks one tree under
// internal/<layer>/ and parses .go files with go/parser. Violations are
// emitted as warn-severity Checks so doctor's exit code reflects only
// "broken setup" (manifest, tools), not stylistic violations.
//
// All four rules share the schema documented in nc-skills-golang/SKILL.md
// under "layer-rules"; the Rule field on each Check carries that anchor so
// agents can fetch the canonical explanation.

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

const ruleAnchor = "nc-skills-golang/SKILL.md#layer-rules"

// scanLayers runs every layer rule and returns one Check per rule plus one
// Check per violation. A clean layer reports a single OK Check.
func scanLayers(root string, m *manifest.Manifest) []Check {
	var out []Check
	out = append(out, scanHandlerNoRepo(root, m)...)
	out = append(out, scanHandlerNoData(root, m)...)
	out = append(out, scanUsecaseNoHertz(root)...)
	out = append(out, scanUsecaseNoKitex(root)...)
	out = append(out, scanUsecaseNoRequestContext(root)...)
	out = append(out, scanRepoNoRawSQL(root)...)
	out = append(out, scanRepoNoUsecase(root, m)...)
	return out
}

// scanHandlerNoRepo asserts that no file under internal/handler imports a
// path beginning with <module>/internal/repository. Direct repo access from
// handlers bypasses the usecase layer.
func scanHandlerNoRepo(root string, m *manifest.Manifest) []Check {
	const id = "layer.handler.no-repo"
	dir := filepath.Join(root, "internal", "handler")
	if !dirExists(dir) {
		return []Check{okCheck(id, "internal/handler not present (skipped)")}
	}
	forbidden := m.Module + "/internal/repository"
	violations, scanErr := walkImports(dir, func(path string) bool {
		return path == forbidden || strings.HasPrefix(path, forbidden+"/")
	})
	if scanErr != nil {
		return []Check{scanErrCheck(id, scanErr)}
	}
	if len(violations) == 0 {
		return []Check{okCheck(id, "internal/handler does not import internal/repository")}
	}
	return violationChecks(id, "handler imports repository: %s", violations)
}

// scanHandlerNoData asserts that handlers do not import internal/base/data.
// Handlers should delegate to usecases instead of grabbing infrastructure
// clients directly. This rule applies to both Hertz and Kitex handlers.
func scanHandlerNoData(root string, m *manifest.Manifest) []Check {
	const id = "layer.handler.no-data"
	dir := filepath.Join(root, "internal", "handler")
	if !dirExists(dir) {
		return []Check{okCheck(id, "internal/handler not present (skipped)")}
	}
	forbidden := m.Module + "/internal/base/data"
	violations, scanErr := walkImports(dir, func(path string) bool {
		return path == forbidden || strings.HasPrefix(path, forbidden+"/")
	})
	if scanErr != nil {
		return []Check{scanErrCheck(id, scanErr)}
	}
	if len(violations) == 0 {
		return []Check{okCheck(id, "internal/handler does not import internal/base/data")}
	}
	return violationChecks(id, "handler imports base data: %s", violations)
}

// scanUsecaseNoHertz asserts that no file under internal/usecase imports the
// hertz module. Usecases must be transport-agnostic.
func scanUsecaseNoHertz(root string) []Check {
	const id = "layer.usecase.no-hertz"
	dir := filepath.Join(root, "internal", "usecase")
	if !dirExists(dir) {
		return []Check{okCheck(id, "internal/usecase not present (skipped)")}
	}
	violations, scanErr := walkImports(dir, func(path string) bool {
		return strings.HasPrefix(path, "github.com/cloudwego/hertz")
	})
	if scanErr != nil {
		return []Check{scanErrCheck(id, scanErr)}
	}
	if len(violations) == 0 {
		return []Check{okCheck(id, "internal/usecase is hertz-free")}
	}
	return violationChecks(id, "usecase imports hertz: %s", violations)
}

// scanUsecaseNoKitex asserts that no file under internal/usecase imports the
// kitex module. Usecases must stay transport-agnostic for RPC projects too.
func scanUsecaseNoKitex(root string) []Check {
	const id = "layer.usecase.no-kitex"
	dir := filepath.Join(root, "internal", "usecase")
	if !dirExists(dir) {
		return []Check{okCheck(id, "internal/usecase not present (skipped)")}
	}
	violations, scanErr := walkImports(dir, func(path string) bool {
		return strings.HasPrefix(path, "github.com/cloudwego/kitex")
	})
	if scanErr != nil {
		return []Check{scanErrCheck(id, scanErr)}
	}
	if len(violations) == 0 {
		return []Check{okCheck(id, "internal/usecase is kitex-free")}
	}
	return violationChecks(id, "usecase imports kitex: %s", violations)
}

// scanUsecaseNoRequestContext catches the specific pattern of leaking
// *app.RequestContext into usecase signatures or fields. The import-level
// check above is broader; this one pinpoints the exact selector and is the
// rule users hit most often.
func scanUsecaseNoRequestContext(root string) []Check {
	const id = "layer.usecase.no-request-context"
	dir := filepath.Join(root, "internal", "usecase")
	if !dirExists(dir) {
		return []Check{okCheck(id, "internal/usecase not present (skipped)")}
	}
	violations, scanErr := walkSelectors(dir, "app", "RequestContext")
	if scanErr != nil {
		return []Check{scanErrCheck(id, scanErr)}
	}
	if len(violations) == 0 {
		return []Check{okCheck(id, "no app.RequestContext leak in internal/usecase")}
	}
	return violationChecks(id, "usecase references app.RequestContext: %s", violations)
}

// scanRepoNoRawSQL asserts that no string literal under internal/repository
// looks like a SQL statement. The heuristic favours precision: the trimmed,
// upper-cased literal must start with one of SELECT / INSERT / UPDATE /
// DELETE / WITH followed by whitespace. sqlc-generated queries live in
// internal/db/, so this rule is meant to catch ad-hoc SQL that bypasses sqlc.
func scanRepoNoRawSQL(root string) []Check {
	const id = "layer.repo.no-sql-string"
	dir := filepath.Join(root, "internal", "repository")
	if !dirExists(dir) {
		return []Check{okCheck(id, "internal/repository not present (skipped)")}
	}
	violations, scanErr := walkStringLiterals(dir, looksLikeSQL)
	if scanErr != nil {
		return []Check{scanErrCheck(id, scanErr)}
	}
	if len(violations) == 0 {
		return []Check{okCheck(id, "internal/repository has no raw SQL strings")}
	}
	return violationChecks(id, "raw SQL string in repository: %s", violations)
}

// scanRepoNoUsecase asserts that repository implementations do not import
// internal/usecase. Ports are declared in usecase packages, but concrete
// repositories should be wired from base/server rather than depending back on
// the layer above.
func scanRepoNoUsecase(root string, m *manifest.Manifest) []Check {
	const id = "layer.repo.no-usecase"
	dir := filepath.Join(root, "internal", "repository")
	if !dirExists(dir) {
		return []Check{okCheck(id, "internal/repository not present (skipped)")}
	}
	forbidden := m.Module + "/internal/usecase"
	violations, scanErr := walkImports(dir, func(path string) bool {
		return path == forbidden || strings.HasPrefix(path, forbidden+"/")
	})
	if scanErr != nil {
		return []Check{scanErrCheck(id, scanErr)}
	}
	if len(violations) == 0 {
		return []Check{okCheck(id, "internal/repository does not import internal/usecase")}
	}
	return violationChecks(id, "repository imports usecase: %s", violations)
}

// looksLikeSQL is the SQL detection heuristic. It is intentionally narrow to
// avoid flagging error messages and log lines that happen to contain SQL
// words.
func looksLikeSQL(lit string) bool {
	s := strings.TrimSpace(lit)
	if len(s) < 10 {
		return false
	}
	prefix := strings.ToUpper(s)
	if len(prefix) > 32 {
		prefix = prefix[:32]
	}
	for _, p := range []string{"SELECT ", "SELECT\n", "SELECT\t", "INSERT INTO ", "INSERT\nINTO", "UPDATE ", "DELETE FROM ", "DELETE\nFROM", "WITH "} {
		if strings.HasPrefix(prefix, p) {
			return true
		}
	}
	return false
}

func dirExists(p string) bool {
	info, err := fileStat(p)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// fileStat is split out so tests can swap it; production uses os.Stat.
var fileStat = func(p string) (fs.FileInfo, error) {
	info, err := osStat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return info, err
}

package mono

import (
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/assets"
	scaffoldtemplate "github.com/byx-darwin/ncgo/internal/scaffold/template"
)

// TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior locks Issue #117's
// fix: any kitex-template file in the rule-center preset whose body invites
// hand-editing ("Edit business logic here") must declare
// update_behavior: skip, or `make update` (which invokes the vendored kitex
// -template-dir binary directly, bypassing ncgo) silently overwrites the
// user's business logic on every regeneration.
func TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior(t *testing.T) {
	const handEditMarker = "Edit business logic here"
	srcFS := assets.FS()
	entries, err := fs.ReadDir(srcFS, "kitex/kitex-template")
	if err != nil {
		t.Fatalf("read kitex-template dir: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := fs.ReadFile(srcFS, "kitex/kitex-template/"+e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var tpl scaffoldtemplate.TemplateFile
		if err := yaml.Unmarshal(b, &tpl); err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		if !strings.Contains(tpl.Body, handEditMarker) {
			continue
		}
		checked++
		if tpl.UpdateBehavior.Type != "skip" {
			t.Errorf("%s: body invites hand-editing (%q) but update_behavior.type = %q, want \"skip\" — make update will silently overwrite user edits",
				e.Name(), handEditMarker, tpl.UpdateBehavior.Type)
		}
	}
	if checked == 0 {
		t.Fatal("no kitex-template file matched the hand-edit marker — test is not exercising the intended files; did the marker text change?")
	}
}

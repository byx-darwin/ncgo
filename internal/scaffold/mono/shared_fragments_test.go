package mono

import (
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/assets"
)

func TestSharedRateLimitFragmentsParse(t *testing.T) {
	srcFS := assets.FS()
	for _, frag := range []struct {
		name       string
		wantPath   string
		wantSymbol string
	}{
		{"ratelimit/resolver", "internal/pkg/ratelimit/resolver.go", "func NewResolver("},
		{"ratelimit/resolver_test", "internal/pkg/ratelimit/resolver_test.go", "func TestResolver"},
	} {
		b, err := fs.ReadFile(srcFS, frag.name+".yaml")
		if err != nil {
			t.Fatalf("read %s: %v", frag.name, err)
		}
		var doc struct {
			Path string `yaml:"path"`
			Body string `yaml:"body"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			t.Fatalf("parse %s: %v", frag.name, err)
		}
		if doc.Path != frag.wantPath {
			t.Errorf("%s path = %q, want %q", frag.name, doc.Path, frag.wantPath)
		}
		if !strings.Contains(doc.Body, frag.wantSymbol) {
			t.Errorf("%s body missing %q", frag.name, frag.wantSymbol)
		}
		if strings.Contains(doc.Body, "{{.GoModule}}") {
			t.Errorf("%s body must use {{.Module}}, found {{.GoModule}}", frag.name)
		}
	}
}

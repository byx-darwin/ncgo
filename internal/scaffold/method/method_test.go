package method

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/domain"
)

func seedDomainProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.3.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: "github.com/acme/demo",
		Service: manifest.Service{
			Name: "demo", Kind: manifest.KindHertz, IDL: "idl/app/demo.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	if _, err := domain.Add(domain.Options{Root: root, Name: "device"}); err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return root
}

func TestAddMethodResultHasNextSteps(t *testing.T) {
	root := seedDomainProject(t)
	res, err := Add(Options{Root: root, Spec: "device.Get"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(res.NextSteps) == 0 {
		t.Fatalf("Result.NextSteps must not be empty")
	}
	joined := strings.Join(res.NextSteps, "\n")
	if !strings.Contains(joined, "ncgo ai sync --root .") {
		t.Fatalf("NextSteps missing ai sync hint:\n%s", joined)
	}
}

func TestAddUsecaseMethod(t *testing.T) {
	root := seedDomainProject(t)
	res, err := Add(Options{Root: root, Spec: "device.ListThemes", Layer: LayerUsecase})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if res.Domain != "device" || res.Method != "ListThemes" {
		t.Errorf("result = %+v", res)
	}
	body, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("read usecase: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"// ncgo:methods:start",
		"// ListThemes is a usecase method scaffolded by ncgo.",
		"func (u *UseCase) ListThemes() error {",
		"return nil",
		"// ncgo:methods:end",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("usecase missing %q\n---\n%s", want, s)
		}
	}
	if strings.Index(s, "func (u *UseCase) ListThemes()") > strings.Index(s, "// ncgo:methods:end") {
		t.Errorf("method was not inserted before end marker\n---\n%s", s)
	}
}

func TestAddRejectsDuplicateMethod(t *testing.T) {
	root := seedDomainProject(t)
	if _, err := Add(Options{Root: root, Spec: "device.ListThemes"}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	_, err := Add(Options{Root: root, Spec: "device.ListThemes"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestAddRejectsInvalidInputs(t *testing.T) {
	root := seedDomainProject(t)
	cases := []struct {
		name string
		opts Options
		want string
	}{
		{"bad spec", Options{Root: root, Spec: "ListThemes"}, "<domain>.<Method>"},
		{"bad domain", Options{Root: root, Spec: "bad-domain.ListThemes"}, "domain"},
		{"bad method", Options{Root: root, Spec: "device.listThemes"}, "method"},
		{"bad layer", Options{Root: root, Spec: "device.ListThemes", Layer: "repository"}, "only usecase"},
		{"unlisted domain", Options{Root: root, Spec: "theme.ListThemes"}, "not listed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Add(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Add error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestAddRequiresMarkers(t *testing.T) {
	root := seedDomainProject(t)
	p := filepath.Join(root, "internal", "usecase", "device", "device.go")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read usecase: %v", err)
	}
	body = []byte(strings.ReplaceAll(string(body), "// ncgo:methods:end", "// missing:end"))
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatalf("write usecase: %v", err)
	}
	_, err = Add(Options{Root: root, Spec: "device.ListThemes"})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("Add error = %v, want missing marker", err)
	}
}

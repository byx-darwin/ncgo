package mono

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestExpandIncludes(t *testing.T) {
	fragment := "# shared\npath: internal/pkg/ratelimit/resolver.go\nupdate_behavior:\n  type: cover\nbody: |-\n  package ratelimit\n\n  // module {{.Module}}/internal/base/conf\n"
	fsys := fstest.MapFS{
		"ratelimit/resolver.yaml": &fstest.MapFile{Data: []byte(fragment)},
	}
	layout := "layouts:\n  - path: internal/handler/\n    delims: [\"\", \"\"]\n    body: \"\"\n  # {{include: ratelimit/resolver}}\n  - path: internal/usecase/\n    delims: [\"\", \"\"]\n    body: \"\"\n"

	out, err := expandIncludes([]byte(layout), fsys)
	if err != nil {
		t.Fatalf("expandIncludes: %v", err)
	}
	got := string(out)

	wantEntry := "  - path: internal/pkg/ratelimit/resolver.go\n    delims: [\"{{\", \"}}\"]\n    body: |-\n      package ratelimit\n\n      // module {{.GoModule}}/internal/base/conf\n"
	if !strings.Contains(got, wantEntry) {
		t.Errorf("expanded entry mismatch\ngot:\n%s\nwant substring:\n%s", got, wantEntry)
	}
	if strings.Contains(got, "{{include:") {
		t.Errorf("directive not consumed:\n%s", got)
	}
	if !strings.Contains(got, "  - path: internal/usecase/") {
		t.Errorf("following entries lost:\n%s", got)
	}
}

func TestExpandIncludesMissingFragment(t *testing.T) {
	fsys := fstest.MapFS{}
	_, err := expandIncludes([]byte("layouts:\n  # {{include: ratelimit/missing}}\n"), fsys)
	if err == nil || !strings.Contains(err.Error(), "ratelimit/missing") {
		t.Fatalf("want missing-fragment error, got %v", err)
	}
}

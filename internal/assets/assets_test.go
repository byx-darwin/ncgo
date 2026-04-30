package assets

import (
	"io/fs"
	"strings"
	"testing"
)

func TestVersionParsesAssetsVersion(t *testing.T) {
	v := Version()
	if v == "" || v == "unknown" {
		t.Fatalf("Version() = %q, want a parsed assets version", v)
	}
	if strings.ContainsRune(v, '/') || strings.ContainsRune(v, ' ') {
		t.Fatalf("Version() = %q, looks malformed", v)
	}
}

func TestEmbeddedFilesPresent(t *testing.T) {
	want := []string{
		"VERSION",
		"docs/hertz/design-doc.en.md",
		"docs/hertz/design-doc.zh-CN.md",
		"docs/kitex/design-doc.en.md",
		"docs/kitex/design-doc.zh-CN.md",
		"hertz/data.json",
		"hertz/layout.yaml",
		"hertz/package.yaml",
		"hertz/sqlc.yaml",
		"hertz/optional/redis.go",
		"hertz/optional/kafka.go",
		"hertz/optional/es.go",
		"hertz/optional/clickhouse.go",
		"kitex/sqlc.yaml",
		"kitex/kitex-template/server.yaml",
		"kitex/kitex-template/handler.yaml",
		"kitex/kitex-template/usecase.yaml",
		"kitex/kitex-template/repository.yaml",
		"kitex/optional/registry_etcd.go",
		"optional/observability_otel.go",
	}
	for _, p := range want {
		if _, err := fs.Stat(FS(), p); err != nil {
			t.Errorf("missing embedded file %q: %v", p, err)
		}
	}
}

func TestEmbeddedFilesNonEmpty(t *testing.T) {
	count := 0
	err := fs.WalkDir(FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	// Snapshot has VERSION + 4 design docs + hertz (5 + 4 optional) + kitex templates/optionals + common optionals.
	if count < 29 {
		t.Fatalf("embedded file count = %d, expected >= 29", count)
	}
}

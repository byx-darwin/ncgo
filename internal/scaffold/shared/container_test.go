package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestRenderComposeProjectEmpty(t *testing.T) {
	body, err := renderComposeProject("commerce", nil)
	if err != nil {
		t.Fatalf("renderComposeProject: %v", err)
	}
	if body != "name: commerce\nservices: {}\n" {
		t.Fatalf("unexpected empty compose body:\n%s", body)
	}
}

func TestRenderComposeProjectIncludesDependenciesAndProfiles(t *testing.T) {
	body, err := renderComposeProject("commerce", []composeApp{
		{
			Name:         "web-bff",
			Kind:         manifest.KindHertz,
			Context:      "./services/web-bff",
			HostPort:     18080,
			WithDatabase: true,
			Infra:        []string{"redis", "kafka", "es", "clickhouse"},
		},
		{
			Name:     "user-rpc",
			Kind:     manifest.KindKitex,
			Context:  "./services/user-rpc",
			HostPort: 18888,
			Infra:    []string{"registry_etcd"},
		},
	})
	if err != nil {
		t.Fatalf("renderComposeProject: %v", err)
	}
	for _, want := range []string{
		"web-bff:",
		"./services/web-bff",
		"GO_ENV: docker",
		"DATABASE_URL: postgres://postgres:postgres@postgres:5432/web-bff?sslmode=disable",
		"depends_on:",
		"- postgres",
		"- redis",
		"- kafka",
		"- elasticsearch",
		"- clickhouse",
		"18080:8080",
		"user-rpc:",
		"18888:8888",
		"- etcd",
		"postgres:",
		"redis:",
		"kafka:",
		"elasticsearch:",
		"clickhouse:",
		"etcd:",
		"profiles:",
		"config-center-nacos",
		"config-center-polaris",
		"postgres-data:",
		"redis-data:",
		"kafka-data:",
		"elasticsearch-data:",
		"clickhouse-data:",
		"etcd-data:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("compose body missing %q\n---\n%s", want, body)
		}
	}
}

func TestWriteServiceDockerConfigForHertz(t *testing.T) {
	root := t.TempDir()
	err := WriteServiceDockerConfig(root, &manifest.Manifest{
		Service: manifest.Service{Name: "demo", Kind: manifest.KindHertz, WithDatabase: true},
		Infra:   []string{"redis", "kafka", "es", "clickhouse"},
	})
	if err != nil {
		t.Fatalf("WriteServiceDockerConfig: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "conf", "docker", "conf.yaml"))
	if err != nil {
		t.Fatalf("read conf/docker/conf.yaml: %v", err)
	}
	for _, want := range []string{
		"env: docker",
		"config_center:",
		"server_addr: nacos:8848",
		"addresses:",
		"- polaris:8093",
		"release:",
		"- polaris:8091",
		`dsn: "postgres://postgres:postgres@postgres:5432/demo?sslmode=disable"`,
		"- redis:6379",
		"- kafka:9092",
		"- http://elasticsearch:9200",
		"- clickhouse:9000",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("docker conf missing %q\n---\n%s", want, body)
		}
	}
}

func TestWriteServiceDockerConfigForKitex(t *testing.T) {
	root := t.TempDir()
	err := WriteServiceDockerConfig(root, &manifest.Manifest{
		Service: manifest.Service{Name: "user-rpc", Kind: manifest.KindKitex, WithDatabase: true},
	})
	if err != nil {
		t.Fatalf("WriteServiceDockerConfig: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "conf", "docker", "conf.yaml"))
	if err != nil {
		t.Fatalf("read conf/docker/conf.yaml: %v", err)
	}
	for _, want := range []string{
		"env: docker",
		`dsn: "postgres://postgres:postgres@postgres:5432/user-rpc?sslmode=disable"`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("docker conf missing %q\n---\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"config_center:", "release:"} {
		if strings.Contains(string(body), unwanted) {
			t.Fatalf("kitex docker conf should not include %q\n---\n%s", unwanted, body)
		}
	}
}

func TestWriteWorkspaceComposeLoadsServiceManifestDependencies(t *testing.T) {
	root := t.TempDir()
	w := &manifest.Workspace{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test-assets"},
		Mode:        manifest.ModeMicro,
		Name:        "commerce",
		Module:      "github.com/acme/commerce",
		Services:    []manifest.WorkspaceService{{Name: "web-bff", Kind: manifest.KindHertz, Dir: "services/web-bff"}},
		GeneratedAt: time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
	}
	if err := manifest.SaveWorkspace(root, w); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	serviceRoot := filepath.Join(root, "services", "web-bff")
	if err := manifest.Save(serviceRoot, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test-assets"},
		Mode:   manifest.ModeMono,
		Module: "github.com/acme/commerce/services/web-bff",
		Service: manifest.Service{
			Name:         "web-bff",
			Kind:         manifest.KindHertz,
			WithDatabase: true,
			IDL:          "idl/app/web-bff.proto",
		},
		Infra:       []string{"redis"},
		GeneratedAt: time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Save service manifest: %v", err)
	}
	if err := WriteWorkspaceCompose(root, w); err != nil {
		t.Fatalf("WriteWorkspaceCompose: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose.yaml: %v", err)
	}
	for _, want := range []string{"postgres:", "redis:", "18080:8080", "config-center-nacos", "GO_ENV: docker"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("workspace compose missing %q\n---\n%s", want, body)
		}
	}
}

package releasecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseGateContract(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	release := readRepoFile(t, ".github/workflows/release.yml")
	assets := readRepoFile(t, "scripts/verify-release-assets.sh")

	for _, want := range []string{
		"workflow_call:",
		"release_gate:",
		"./scripts/test-unit.sh",
		"./scripts/test-generated.sh",
		"go test -short -race -covermode=atomic",
		"HZ_VERSION: v0.9.7",
		"KITEX_VERSION: v0.16.1",
		"SQLC_VERSION: v1.30.0",
		"PROTOC_VERSION: '28.3'",
		"go-version: '1.26.5'",
	} {
		if !strings.Contains(ci, want) {
			t.Errorf("CI release contract missing %q", want)
		}
	}
	if strings.Contains(ci, "sqlc@latest") || strings.Contains(ci, "apt-get install -y protobuf-compiler") {
		t.Error("CI must not install floating sqlc or protoc versions")
	}

	for _, want := range []string{
		"uses: ./.github/workflows/ci.yml",
		"release_gate: true",
		"needs: verify",
		"./scripts/build-release.sh",
		"actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6",
		"artifact-metadata: write",
		"./scripts/verify-release-assets.sh",
		"dist/provenance.json",
	} {
		if !strings.Contains(release, want) {
			t.Errorf("release workflow contract missing %q", want)
		}
	}
	if !strings.Contains(assets, "sha256sum -c checksums.txt") {
		t.Error("release asset verifier must validate the published checksums")
	}
	for _, floating := range []string{
		"actions/checkout@v",
		"actions/setup-go@v",
		"actions/cache@v",
		"actions/upload-artifact@v",
		"actions/download-artifact@v",
		"actions/attest@v",
		"arduino/setup-protoc@v",
	} {
		if strings.Contains(ci, floating) || strings.Contains(release, floating) {
			t.Errorf("required release path contains mutable action ref %q", floating)
		}
	}
}

func readRepoFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", filepath.FromSlash(name))
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(body)
}

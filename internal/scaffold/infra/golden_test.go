package infra

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/testutil/golden"
)

// TestGenerateGoldenInfraRedis locks the output of redis add-on for a Hertz service.
func TestGenerateGoldenInfraRedis(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: "redis", Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add redis: %v", err)
	}
	m, _ := manifest.Load(root)
	if !strings.Contains(strings.Join(m.Infra, ","), "redis") {
		t.Error("manifest infra should include redis")
	}
	for _, p := range res.WrittenPaths {
		golden.File(t, filepath.Join("infra-redis", filepath.Base(p)), goldenReadFile(t, p))
	}
}

// TestGenerateGoldenInfraLoggingHertz locks the logging add-on for a Hertz service.
func TestGenerateGoldenInfraLoggingHertz(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindObservabilityLog, Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add logging: %v", err)
	}
	for _, p := range res.WrittenPaths {
		golden.File(t, filepath.Join("infra-logging-hertz", filepath.Base(p)), goldenReadFile(t, p))
	}
}

// TestGenerateGoldenInfraLoggingKitex locks the logging add-on for a Kitex service.
func TestGenerateGoldenInfraLoggingKitex(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindObservabilityLog, Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add logging kitex: %v", err)
	}
	for _, p := range res.WrittenPaths {
		golden.File(t, filepath.Join("infra-logging-kitex", filepath.Base(p)), goldenReadFile(t, p))
	}
}

// TestGenerateGoldenInfraCanary locks the canary-release add-on.
func TestGenerateGoldenInfraCanary(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindReleaseCanary, Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add canary: %v", err)
	}
	for _, p := range res.WrittenPaths {
		golden.File(t, filepath.Join("infra-canary", filepath.Base(p)), goldenReadFile(t, p))
	}
}

// TestGenerateGoldenInfraCanaryKitex locks the canary-release add-on for a Kitex service.
// It shares the infra-canary/ directory with the Hertz variant so that a single
// `-run TestGenerateGoldenInfraCanary` regenerates both adapters.
func TestGenerateGoldenInfraCanaryKitex(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindReleaseCanary, Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add canary kitex: %v", err)
	}
	for _, p := range res.WrittenPaths {
		base := filepath.Base(p)
		// Skip the SDK-neutral seam and ops file; they are byte-identical to the
		// Hertz variant and already locked by TestGenerateGoldenInfraCanary.
		if base == "canary.go" || base == "ops.go" {
			continue
		}
		golden.File(t, filepath.Join("infra-canary", base), goldenReadFile(t, p))
	}
}

// TestGenerateGoldenInfraPolarisAdapter locks the polaris_adapter add-on for a Kitex service.
func TestGenerateGoldenInfraPolarisAdapter(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindPolarisAdapter, Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add polaris_adapter: %v", err)
	}
	for _, p := range res.WrittenPaths {
		golden.File(t, filepath.Join("infra-polaris-adapter", filepath.Base(p)), goldenReadFile(t, p))
	}
}

func goldenReadFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return body
}

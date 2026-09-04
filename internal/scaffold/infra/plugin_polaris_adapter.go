package infra

import (
	"fmt"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(polarisAdapterPlugin{}) }

type polarisAdapterPlugin struct{}

func (polarisAdapterPlugin) Kind() string         { return KindPolarisAdapter }
func (polarisAdapterPlugin) Aliases() []string    { return []string{KindPolarisAdapterAlias} }
func (polarisAdapterPlugin) ServiceScope() string { return "kitex" }
func (polarisAdapterPlugin) GoGetDeps() []string {
	return []string{"github.com/polarismesh/polaris-go", "gopkg.in/yaml.v3", "github.com/byx-darwin/go-tools/go-common", "go.opentelemetry.io/otel", "go.opentelemetry.io/otel/metric"}
}
func (polarisAdapterPlugin) SetupSteps() []string {
	return []string{
		"go get github.com/polarismesh/polaris-go",
		"go get gopkg.in/yaml.v3",
		"go get github.com/byx-darwin/go-tools/go-common",
		"go get go.opentelemetry.io/otel/metric",
		"set POLARIS_TOKEN / POLARIS_NAMESPACE env vars (never hardcode credentials)",
		"wire release.NewPolarisSelector(...) into KitexCanaryLoadBalancer.RuleProvider",
		"verify kitex resolver returns full stable+canary instance set (see troubleshooting)",
		"go mod tidy",
	}
}
func (polarisAdapterPlugin) HertzConfigKey() string { return "" }

func (polarisAdapterPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	if serviceKind != manifest.KindKitex {
		return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", KindPolarisAdapter)
	}
	return []addOnFile{
		{SourcePath: "kitex/optional/polaris_canary_adapter.go", OutputRelPath: "internal/base/release/polaris_adapter.go"},
		{SourcePath: "kitex/optional/polaris_canary_observer_otel.go", OutputRelPath: "internal/base/release/polaris_observer_otel.go"},
	}, nil
}

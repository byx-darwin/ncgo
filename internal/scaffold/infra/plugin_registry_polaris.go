package infra

import (
	"fmt"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(registryPolarisPlugin{}) }

type registryPolarisPlugin struct{}

func (registryPolarisPlugin) Kind() string         { return KindRegistryPolaris }
func (registryPolarisPlugin) Aliases() []string    { return nil }
func (registryPolarisPlugin) ServiceScope() string { return "kitex" }
func (registryPolarisPlugin) GoGetDeps() []string {
	return []string{"github.com/kitex-contrib/polaris", "github.com/byx-darwin/go-tools/go-common"}
}
func (registryPolarisPlugin) SetupSteps() []string   { return nil }
func (registryPolarisPlugin) HertzConfigKey() string { return "" }

func (registryPolarisPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	if serviceKind != manifest.KindKitex {
		return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", KindRegistryPolaris)
	}
	return []addOnFile{
		{SourcePath: "kitex/optional/registry_polaris.go", OutputRelPath: "internal/base/registry/polaris.go"},
		{SourcePath: "kitex/optional/registry_polaris.yaml", OutputRelPath: "polaris.yaml"},
	}, nil
}

func (registryPolarisPlugin) WireKitexServer(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/registry", "", plan)
	if err != nil {
		return "", err
	}
	return insertOnceMarkerOrAnchorWithPlan(s, "kitexserver.WithRegistry(", markerRegistryServer, "\topts = append(opts, extraOptions...)\n", kitexRegistryServer(), "", plan, "insert_registry_server", "registry.NewRegistry")
}

func (registryPolarisPlugin) WireKitexClient(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/registry", "", plan)
	if err != nil {
		return "", err
	}
	anchor := "\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"
	return insertOnceMarkerOrAnchorWithPlan(s, "kitexclient.WithResolver(", markerRegistryClient, anchor, kitexRegistryClient(), "", plan, "insert_registry_client", "registry.NewResolver")
}

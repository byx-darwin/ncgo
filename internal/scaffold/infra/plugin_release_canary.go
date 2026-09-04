package infra

import (
	"fmt"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(releaseCanaryPlugin{}) }

type releaseCanaryPlugin struct{}

func (releaseCanaryPlugin) Kind() string           { return KindReleaseCanary }
func (releaseCanaryPlugin) Aliases() []string      { return []string{KindCanaryAlias} }
func (releaseCanaryPlugin) ServiceScope() string   { return "common" }
func (releaseCanaryPlugin) GoGetDeps() []string    { return nil }
func (releaseCanaryPlugin) SetupSteps() []string   { return nil }
func (releaseCanaryPlugin) HertzConfigKey() string { return "" }

func (releaseCanaryPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	files := []addOnFile{{SourcePath: "optional/" + KindReleaseCanary + ".go", OutputRelPath: "internal/base/release/canary.go"}}
	switch serviceKind {
	case manifest.KindHertz, manifest.KindKitex:
		files = append(files, addOnFile{SourcePath: serviceKind + "/optional/" + KindReleaseCanary + ".go", OutputRelPath: frameworkAdapterName(KindReleaseCanary, serviceKind)})
	default:
		return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
	}
	files = append(files, addOnFile{SourcePath: "optional/release_ops.go", OutputRelPath: "internal/base/release/ops.go"})
	return files, nil
}

func (releaseCanaryPlugin) WireHertzServer(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/release", "", plan)
	if err != nil {
		return "", err
	}
	return insertAfterMarkerOrAnyWithPlan(s, "release.HertzTraffic()", markerCanaryServerTraffic, []string{
		"\th.Use(logging.HertzRequestID())\n",
		"\th.Use(middleware.RequestID())\n",
	}, hertzCanaryTraffic(), "", plan, "insert_traffic_middleware", "release.HertzTraffic")
}

func (releaseCanaryPlugin) WireKitexServer(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/release", "", plan)
	if err != nil {
		return "", err
	}
	return insertAfterMarkerOrAnyWithPlan(s, "release.KitexTraffic()", markerCanaryServerTraffic, []string{
		"\t\t\tlogging.KitexRequestID(),\n",
		"\t\t\tinterceptor.RequestID(),\n",
	}, "\t\t\trelease.KitexTraffic(),\n", "", plan, "insert_traffic_middleware", "release.KitexTraffic")
}

func (releaseCanaryPlugin) WireKitexClient(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/release", "", plan)
	if err != nil {
		return "", err
	}
	anchor := "\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"
	return insertOnceMarkerOrAnchorWithPlan(s, "release.KitexTraffic()", markerKitexClientMiddleware, anchor, "\toptions = append(options, kitexclient.WithMiddleware(release.KitexTraffic()))\n", "", plan, "insert_client_middleware", "release.KitexTraffic")
}

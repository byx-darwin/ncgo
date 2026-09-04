package infra

import (
	"fmt"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(observabilityLoggingPlugin{}) }

type observabilityLoggingPlugin struct{}

func (observabilityLoggingPlugin) Kind() string         { return KindObservabilityLog }
func (observabilityLoggingPlugin) Aliases() []string    { return []string{KindLoggingAlias} }
func (observabilityLoggingPlugin) ServiceScope() string { return "common" }
func (observabilityLoggingPlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-common"}
}
func (observabilityLoggingPlugin) SetupSteps() []string   { return nil }
func (observabilityLoggingPlugin) HertzConfigKey() string { return "" }

func (observabilityLoggingPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	files := []addOnFile{{SourcePath: "optional/" + KindObservabilityLog + ".go", OutputRelPath: "internal/base/logging/logging.go"}}
	switch serviceKind {
	case manifest.KindHertz, manifest.KindKitex:
		files = append(files, addOnFile{SourcePath: serviceKind + "/optional/" + KindObservabilityLog + ".go", OutputRelPath: frameworkAdapterName(KindObservabilityLog, serviceKind)})
	default:
		return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
	}
	return files, nil
}

func (observabilityLoggingPlugin) WireHertzServer(s, module string, plan *[]PlanItem) (string, error) {
	var err error
	s, err = addGoImportWithPlan(s, module+"/internal/base/logging", "", plan)
	if err != nil {
		return "", err
	}
	s, err = insertOnceMarkerOrAnchorWithPlan(s, "logging.Init(", markerLoggingInit, "\tdo.ProvideValue(injector, cfg)\n", hertzLoggingInit(), "", plan, "insert_logging_init", "logging.Init")
	if err != nil {
		return "", err
	}
	s, err = replaceOnceStrictWithPlan(s, "h.Use(logging.HertzRecovery())", "h.Use(middleware.Recovery())", "h.Use(logging.HertzRecovery())", "", plan, "hertz recovery")
	if err != nil {
		return "", err
	}
	s, err = replaceOnceStrictWithPlan(s, "h.Use(logging.HertzRequestID())", "h.Use(middleware.RequestID())", "h.Use(logging.HertzRequestID())", "", plan, "hertz request id")
	if err != nil {
		return "", err
	}
	return replaceOnceStrictWithPlan(s, "h.Use(logging.HertzAccessLog())", "h.Use(middleware.AccessLog())", "h.Use(logging.HertzAccessLog())", "", plan, "hertz access log")
}

func (observabilityLoggingPlugin) WireKitexServer(s, module string, plan *[]PlanItem) (string, error) {
	var err error
	s, err = addGoImportWithPlan(s, module+"/internal/base/logging", "", plan)
	if err != nil {
		return "", err
	}
	s, err = insertOnceMarkerOrAnchorWithPlan(s, "logging.Init(", markerLoggingInit, "\tif cfg == nil {\n\t\tcfg = conf.Default()\n\t}\n", kitexLoggingInit(), "", plan, "insert_logging_init", "logging.Init")
	if err != nil {
		return "", err
	}
	s, err = replaceOnceStrictWithPlan(s, "logging.KitexRequestID(),", "interceptor.RequestID(),", "logging.KitexRequestID(),", "", plan, "kitex request id")
	if err != nil {
		return "", err
	}
	s, err = replaceOnceStrictWithPlan(s, "logging.KitexAccessLog(),", "interceptor.AccessLog(),", "logging.KitexAccessLog(),", "", plan, "kitex access log")
	if err != nil {
		return "", err
	}
	return replaceOnceStrictWithPlan(s, "logging.KitexRecovery(),", "interceptor.Recovery(),", "logging.KitexRecovery(),", "", plan, "kitex recovery")
}

func (observabilityLoggingPlugin) WireKitexClient(s, module string, plan *[]PlanItem) (string, error) {
	var err error
	s, err = addGoImportWithPlan(s, module+"/internal/base/logging", "", plan)
	if err != nil {
		return "", err
	}
	anchor := "\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"
	loggingBlock := "\toptions = append(options, kitexclient.WithMiddleware(endpoint.Chain(\n\t\tlogging.KitexRequestID(),\n\t\tlogging.KitexAccessLog(),\n\t)))\n"
	return insertOnceMarkerOrAnchorWithPlan(s, "logging.KitexAccessLog()", markerKitexClientMiddleware, anchor, loggingBlock, "", plan, "insert_client_middleware", "logging.KitexAccessLog")
}

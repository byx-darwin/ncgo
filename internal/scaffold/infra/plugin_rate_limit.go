package infra

import (
	"fmt"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(rateLimitPlugin{}) }

type rateLimitPlugin struct{}

func (rateLimitPlugin) Kind() string         { return KindRateLimit }
func (rateLimitPlugin) Aliases() []string    { return []string{KindRateLimitAlias} }
func (rateLimitPlugin) ServiceScope() string { return "kitex" }
func (rateLimitPlugin) GoGetDeps() []string  { return nil }
func (rateLimitPlugin) SetupSteps() []string {
	return []string{
		"review conf/dev/conf.yaml rate_limit block (source.type: config|database|rule_center)",
		"observe shadow logs (grep 'ratelimit shadow denied'), then set mode: enforce",
		"optional: set static.max_qps / static.max_connections for a global safety net",
		"go mod tidy",
	}
}
func (rateLimitPlugin) HertzConfigKey() string { return "" }

func (rateLimitPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	if serviceKind != manifest.KindKitex {
		return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", KindRateLimit)
	}
	return rateLimitAssetFiles()
}

func (rateLimitPlugin) WireKitexServer(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/middleware", "", plan)
	if err != nil {
		return "", err
	}
	s, err = insertAfterMarkerOrAnyWithPlan(s, "\t\t\tmiddleware.RateLimit(cfg.RateLimit),\n", markerRateLimitServerMiddleware, []string{
		"\t\t\tinterceptor.RequestID(),\n",
	}, "\t\t\tmiddleware.RateLimit(cfg.RateLimit),\n", "", plan, "insert_ratelimit_middleware", "middleware.RateLimit")
	if err != nil {
		return "", err
	}
	return insertOnceMarkerOrAnchorWithPlan(s, "middleware.StaticLimitOption(", markerRateLimitStaticLimit, "\topts = append(opts, extraOptions...)\n", "\tif opt := middleware.StaticLimitOption(cfg.RateLimit.Static); opt != nil {\n\t\topts = append(opts, opt)\n\t}\n", "", plan, "insert_ratelimit_static", "middleware.StaticLimitOption")
}

package infra

func init() { Register(esPlugin{}) }

type esPlugin struct{}

func (esPlugin) Kind() string         { return KindES }
func (esPlugin) Aliases() []string    { return nil }
func (esPlugin) ServiceScope() string { return "common" }
func (esPlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
}
func (esPlugin) SetupSteps() []string   { return nil }
func (esPlugin) HertzConfigKey() string { return "es" }
func (esPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	return frameworkAssetFiles(KindES, "internal/base/data/es.go")(serviceKind)
}

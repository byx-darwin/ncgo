package infra

func init() { Register(clickhousePlugin{}) }

type clickhousePlugin struct{}

func (clickhousePlugin) Kind() string         { return KindClickHouse }
func (clickhousePlugin) Aliases() []string    { return nil }
func (clickhousePlugin) ServiceScope() string { return "common" }
func (clickhousePlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
}
func (clickhousePlugin) SetupSteps() []string   { return nil }
func (clickhousePlugin) HertzConfigKey() string { return "clickhouse" }
func (clickhousePlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	return frameworkAssetFiles(KindClickHouse, "internal/base/data/clickhouse.go")(serviceKind)
}

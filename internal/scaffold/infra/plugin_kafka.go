package infra

func init() { Register(kafkaPlugin{}) }

type kafkaPlugin struct{}

func (kafkaPlugin) Kind() string         { return KindKafka }
func (kafkaPlugin) Aliases() []string    { return nil }
func (kafkaPlugin) ServiceScope() string { return "common" }
func (kafkaPlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
}
func (kafkaPlugin) SetupSteps() []string   { return nil }
func (kafkaPlugin) HertzConfigKey() string { return "kafka" }
func (kafkaPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	return frameworkAssetFiles(KindKafka, "internal/base/data/kafka.go")(serviceKind)
}

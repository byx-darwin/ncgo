package infra

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(redisPlugin{}) }

type redisPlugin struct{}

func (redisPlugin) Kind() string         { return KindRedis }
func (redisPlugin) Aliases() []string    { return nil }
func (redisPlugin) ServiceScope() string { return "common" }
func (redisPlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"}
}
func (redisPlugin) SetupSteps() []string   { return nil }
func (redisPlugin) HertzConfigKey() string { return "redis" }

func (redisPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	return frameworkAssetFiles(KindRedis, "internal/base/data/redis.go")(serviceKind)
}

// ExtraFiles appends the Hertz redis shared helper if it is not already
// present in the project (kitex has no equivalent shared helper).
func (redisPlugin) ExtraFiles(root, serviceKind string) ([]addOnFile, error) {
	if serviceKind != manifest.KindHertz {
		return nil, nil
	}
	helperPath := filepath.Join(root, filepath.FromSlash(hertzRedisSharedHelperRelPath))
	if _, err := os.Stat(helperPath); err == nil {
		return nil, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return []addOnFile{{SourcePath: "hertz/optional/redis_shared.go", OutputRelPath: filepath.FromSlash(hertzRedisSharedHelperRelPath)}}, nil
}

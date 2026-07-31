package shared

import (
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadSharedFragmentBody reads a shared fragment yaml (canonical kitex format:
// path/update_behavior/body fields, body 2-space indented, {{.Module}}
// placeholder) and returns the body with the module placeholder rendered.
//
// The fragment yaml lives under internal/assets/_data/<name>.yaml. Callers
// pass the embedded assets.FS() as srcFS and the target module path as
// module; the {{.Module}} placeholder in the body is replaced before the
// bytes are returned.
func ReadSharedFragmentBody(srcFS fs.FS, name, module string) ([]byte, error) {
	b, err := fs.ReadFile(srcFS, name+".yaml")
	if err != nil {
		return nil, fmt.Errorf("read shared fragment %s: %w", name, err)
	}
	var frag struct {
		Body string `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &frag); err != nil {
		return nil, fmt.Errorf("parse shared fragment %s: %w", name, err)
	}
	return []byte(strings.ReplaceAll(frag.Body, "{{.Module}}", module)), nil
}

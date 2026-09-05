package framework

import "strings"

// hasTopLevelConfigKey reports whether src has a non-indented, non-comment
// line beginning with "<key>:". Moved verbatim from
// internal/scaffold/infra/infra.go so both adapters' config-merge logic can
// use it without infra depending back on framework for it.
func hasTopLevelConfigKey(src, key string) bool {
	needle := key + ":"
	for _, line := range strings.Split(src, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if strings.HasPrefix(line, needle) {
			return true
		}
	}
	return false
}

package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// FuncMap returns the template functions available in template bodies.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"ToLower":    strings.ToLower,
		"ToUpper":    strings.ToUpper,
		"LowerFirst": lowerFirst,
		"exportName": exportName,
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func exportName(s string) string {
	out := make([]byte, 0, len(s))
	upper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' || c == '_' {
			upper = true
			continue
		}
		if upper && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
		upper = false
	}
	return string(out)
}

// Render executes a template body with the given data.
func Render(body string, data RenderData) (string, error) {
	tmpl, err := template.New("template").Funcs(FuncMap()).Parse(body)
	if err != nil {
		return "", fmt.Errorf("template parse: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execute: %w", err)
	}
	return buf.String(), nil
}

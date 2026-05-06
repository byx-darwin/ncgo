package protolint

import (
	"fmt"
	"strings"
)

// rulePIO402 warns when a string field whose name suggests free-text input is
// declared in a request message without any PGV length constraint.
type rulePIO402 struct{}

func (rulePIO402) Meta() RuleMeta {
	return RuleMeta{ID: "PIO402", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO402) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if rpc.InputMessage == nil {
			continue
		}
		for _, field := range rpc.InputMessage.Fields {
			if !field.IsString || !isPGVTextInputFieldName(field.Name) || field.HasPGVLenConstraint {
				continue
			}
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("request field %s looks like a free-text input but has no PGV string length constraint", field.Name),
				"declare validate.rules with min_len/max_len (or pattern) on free-text string fields such as keyword/name/title/description/content",
			)
			d.Message = rpc.InputMessage.Name
			d.Field = field.Name
			out = append(out, d)
		}
	}
	return out
}

// isPGVTextInputFieldName returns true for field names that commonly carry
// user-supplied free-text and therefore benefit from an explicit length bound.
func isPGVTextInputFieldName(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "_", ""))
	switch normalized {
	case "keyword", "keywords", "query", "q", "search",
		"name", "title", "description", "content",
		"text", "comment", "remark", "note", "notes",
		"message", "reason", "detail", "details",
		"label", "tag":
		return true
	default:
		return false
	}
}

// rulePIO403 warns when a repeated or map field in a request message has no
// PGV item-count / pair-count constraint.
type rulePIO403 struct{}

func (rulePIO403) Meta() RuleMeta {
	return RuleMeta{ID: "PIO403", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO403) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if rpc.InputMessage == nil {
			continue
		}
		for _, field := range rpc.InputMessage.Fields {
			if (!field.IsRepeated && !field.IsMap) || field.HasPGVItemsConstraint {
				continue
			}
			kind := "repeated"
			hint := "declare validate.rules with max_items on repeated request fields to prevent unbounded list payloads"
			if field.IsMap {
				kind = "map"
				hint = "declare validate.rules with max_pairs on map request fields to prevent unbounded map payloads"
			}
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("request %s field %s has no PGV item count constraint", kind, field.Name),
				hint,
			)
			d.Message = rpc.InputMessage.Name
			d.Field = field.Name
			out = append(out, d)
		}
	}
	return out
}

// rulePIO404 warns when an enum-typed field in a request message has no PGV
// defined_only constraint.  Without it, proto3 wire values outside the known
// enum range are silently accepted.
type rulePIO404 struct{}

func (rulePIO404) Meta() RuleMeta {
	return RuleMeta{ID: "PIO404", Level: LevelWarning, Phase: Phase2}
}

func (r rulePIO404) Check(model *Model) []Diagnostic {
	meta := r.Meta()
	var out []Diagnostic
	for _, rpc := range model.RPCs() {
		if rpc.InputMessage == nil {
			continue
		}
		for _, field := range rpc.InputMessage.Fields {
			if !field.IsEnum || field.HasPGVDefinedOnly {
				continue
			}
			d := rpcDiagnostic(
				meta,
				rpc,
				fmt.Sprintf("request enum field %s has no PGV defined_only constraint; proto3 accepts out-of-range values", field.Name),
				"declare validate.rules with defined_only:true on enum request fields to reject unknown enum values",
			)
			d.Message = rpc.InputMessage.Name
			d.Field = field.Name
			out = append(out, d)
		}
	}
	return out
}

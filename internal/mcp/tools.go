package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/doctor"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/method"
)

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) tools() []tool {
	return []tool{
		{Name: "ncgo_version", Description: "Return ncgo and embedded assets versions.", InputSchema: objectSchema(nil, nil)},
		{Name: "ncgo_doctor", Description: "Run ncgo doctor and return the structured report.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root; empty skips project checks.")}, nil)},
		{Name: "ncgo_ai_sync", Description: "Render AI context files for a project.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root"), "lang": enumProp([]string{ai.LangEN, ai.LangZhCN}), "force": boolProp("Overwrite unmanaged files"), "dryRun": boolProp("Report without writing")}, []string{"root"})},
		{Name: "ncgo_add_infra", Description: "Install an optional infrastructure add-on into an ncgo project.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root"), "kind": enumProp(infra.SupportedKinds()), "force": boolProp("Overwrite existing generated add-on file"), "wire": boolProp("Opt-in: update generated server/client wiring when supported"), "dryRun": boolProp("Preview intended add-on writes and --wire changes without modifying files")}, []string{"root", "kind"})},
		{Name: "ncgo_add_method", Description: "Insert a usecase method stub at ncgo anchors.", InputSchema: objectSchema(map[string]any{"root": stringProp("Project root"), "spec": stringProp("<domain>.<Method>"), "in": enumProp([]string{method.LayerUsecase})}, []string{"root", "spec"})},
	}
}

func (s *Server) callTool(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var p callParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	switch p.Name {
	case "ncgo_version":
		return textResult(fmt.Sprintf("ncgo %s (assets: %s)", s.NCGOVersion, s.AssetsVersion), false), nil
	case "ncgo_doctor":
		return s.callDoctor(ctx, p.Arguments)
	case "ncgo_ai_sync":
		return callAISync(p.Arguments)
	case "ncgo_add_infra":
		return callAddInfra(p.Arguments)
	case "ncgo_add_method":
		return callAddMethod(p.Arguments)
	default:
		return nil, fmt.Errorf("unknown tool %q", p.Name)
	}
}

func (s *Server) callDoctor(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root string `json:"root"`
	}
	_ = json.Unmarshal(raw, &args)
	rep := doctor.Run(ctx, doctor.Options{Root: args.Root})
	b, _ := json.MarshalIndent(rep, "", "  ")
	return textResult(string(b), !rep.OK()), nil
}

func callAISync(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Lang   string `json:"lang"`
		Force  bool   `json:"force"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	res, err := ai.Sync(ai.Options{Root: args.Root, Lang: args.Lang, Force: args.Force, DryRun: args.DryRun})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return textResult(string(b), false), nil
}

func callAddInfra(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Kind   string `json:"kind"`
		Force  bool   `json:"force"`
		Wire   bool   `json:"wire"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	res, err := infra.Add(infra.Options{Root: args.Root, Kind: args.Kind, Force: args.Force, Wire: args.Wire, DryRun: args.DryRun})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	var b strings.Builder
	writeVerb, wireVerb := "wrote", "wired"
	if res.DryRun {
		writeVerb, wireVerb = "would write", "would wire"
	}
	for _, p := range writtenInfraPaths(res) {
		fmt.Fprintf(&b, "%s %s\n", writeVerb, p)
	}
	for _, p := range res.WiredPaths {
		fmt.Fprintf(&b, "%s %s\n", wireVerb, p)
	}
	if res.DryRun && res.Updated {
		b.WriteString("(dry-run: manifest would be updated)\n")
	} else if !res.Updated {
		b.WriteString("(manifest already lists this infra)\n")
	}
	if res.DryRun {
		b.WriteString("(dry-run: no files were written)\n")
	}
	b.WriteString("\nnext steps:\n")
	for _, step := range res.NextSteps {
		fmt.Fprintf(&b, "  $ %s\n", step)
	}
	out := textResult(strings.TrimRight(b.String(), "\n"), false)
	out["dryRun"] = res.DryRun
	out["updated"] = res.Updated
	out["writtenPaths"] = res.WrittenPaths
	out["wiredPaths"] = res.WiredPaths
	out["nextSteps"] = res.NextSteps
	out["plan"] = res.Plan
	return out, nil
}

func writtenInfraPaths(res *infra.Result) []string {
	if len(res.WrittenPaths) > 0 {
		return res.WrittenPaths
	}
	return []string{res.WrittenPath}
}

func callAddMethod(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root  string `json:"root"`
		Spec  string `json:"spec"`
		Layer string `json:"in"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	res, err := method.Add(method.Options{Root: args.Root, Spec: args.Spec, Layer: args.Layer})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return textResult(fmt.Sprintf("inserted %s.%s into %s", res.Domain, res.Method, res.Path), false), nil
}

func textResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]string{{"type": "text", "text": text}}, "isError": isError}
}

func objectSchema(props map[string]any, required []string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func stringProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}
func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func enumProp(vals []string) map[string]any { return map[string]any{"type": "string", "enum": vals} }

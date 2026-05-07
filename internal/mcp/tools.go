package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/method"
)

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) tools() []tool {
	return []tool{
		{Name: "ncgo_version", Description: "Return ncgo, build, and embedded assets versions.", InputSchema: schemaObject(nil)},
		{Name: "ncgo_doctor", Description: "Run ncgo doctor and return the structured report.", InputSchema: schemaObject(nil, rootField("Project root; empty skips project checks."), outputTextJSONSARIFField())},
		{Name: "ncgo_ai_init_claude", Description: "Bootstrap the hand-authored .claude starter set for a repository.", InputSchema: schemaObject([]string{"root"}, rootField("Repository root where .claude/ should be bootstrapped"), enumField("preset", []string{ai.InitPresetMinimal, ai.InitPresetTeam}), boolField("force", "Overwrite existing starter files"), boolField("dryRun", "Report without writing"), outputTextJSONField())},
		{Name: "ncgo_ai_sync", Description: "Render AI context files for an ncgo service or micro workspace.", InputSchema: schemaObject([]string{"root"}, rootField("Service root with .ncgo/manifest.yaml or micro workspace root with ncgo.workspace"), enumField("lang", []string{ai.LangEN, ai.LangZhCN}), boolField("force", "Overwrite unmanaged files"), boolField("dryRun", "Report without writing"), outputTextJSONField())},
		{Name: "ncgo_i18n_report", Description: "Read the generated i18n report for a project and return structured payload for agents.", InputSchema: schemaObject([]string{"root"}, rootField("Project root"), outputTextJSONField())},
		{Name: "ncgo_i18n_check", Description: "Evaluate the generated i18n report for dev or release workflows.", InputSchema: schemaObject([]string{"root"}, rootField("Project root"), enumField("mode", []string{mcpI18NCheckDev, mcpI18NCheckRelease}), outputTextJSONField())},
		{Name: "ncgo_protolint", Description: "Lint selected .proto files with ncgo's Proto I/O rules and return structured diagnostics.", InputSchema: schemaObject([]string{"root"}, rootField("Import root used to resolve the proto files"), stringArrayField("files", "Optional proto entry files relative to root; omit to auto-discover from an ncgo service or micro workspace"), stringArrayField("rules", "Optional rule IDs to run"), stringArrayField("ignoreRules", "Optional rule IDs to suppress from the returned diagnostics"), stringArrayField("ignoreFiles", "Optional proto files whose diagnostics should be suppressed"), outputTextJSONSARIFField())},
		{Name: "ncgo_add_infra", Description: "Install an optional infrastructure add-on into an ncgo project.", InputSchema: schemaObject([]string{"root", "kind"}, rootField("Project root"), enumField("kind", infra.SupportedKinds()), boolField("force", "Overwrite existing generated add-on file"), boolField("wire", "Opt-in: update generated server/client wiring when supported"), boolField("dryRun", "Preview intended add-on writes and --wire changes without modifying files"), outputTextJSONField())},
		{Name: "ncgo_add_method", Description: "Insert a usecase method stub at ncgo anchors.", InputSchema: schemaObject([]string{"root", "spec"}, rootField("Project root"), stringField("spec", "<domain>.<Method>"), enumField("in", []string{method.LayerUsecase}))},
	}
}

func (s *Server) callTool(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var p callParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	switch p.Name {
	case "ncgo_version":
		return textResult(versionText(s.NCGOVersion, s.AssetsVersion, s.BuildVersion, s.BuildTime), false), nil
	case "ncgo_doctor":
		return s.callDoctor(ctx, p.Arguments)
	case "ncgo_ai_init_claude":
		return callAIInitClaude(p.Arguments)
	case "ncgo_ai_sync":
		return callAISync(p.Arguments)
	case "ncgo_i18n_report":
		return callI18NReport(p.Arguments)
	case "ncgo_i18n_check":
		return callI18NCheck(p.Arguments)
	case "ncgo_protolint":
		return callProtolint(ctx, p.Arguments)
	case "ncgo_add_infra":
		return callAddInfra(p.Arguments)
	case "ncgo_add_method":
		return callAddMethod(p.Arguments)
	default:
		return nil, fmt.Errorf("unknown tool %q", p.Name)
	}
}

func versionText(ncgoVersion, assetsVersion, buildVersion, buildTime string) string {
	return fmt.Sprintf("ncgo %s (build: %s, built: %s, assets: %s)", nonEmpty(ncgoVersion, "unknown"), nonEmpty(buildVersion, "unknown"), nonEmpty(buildTime, "unknown"), nonEmpty(assetsVersion, "unknown"))
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

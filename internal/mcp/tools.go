package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/method"
)

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) tools() []tool {
	tools := []tool{
		{Name: "ncgo_version", Description: "Return ncgo, build, and embedded assets versions.", InputSchema: schemaObject(nil), Reference: ref("返回 ncgo、构建与内嵌 assets 版本。", "ncgo version", []string{"content[0].text"}, "None.", "无。")},
		{Name: "ncgo_doctor", Description: "Run ncgo doctor and return the structured report.", InputSchema: schemaObject(nil, rootField("Project root; empty skips project checks."), outputTextJSONSARIFField()), Reference: ref("运行 ncgo doctor 并返回结构化报告。", "ncgo doctor", []string{"root", "scope", "summary", "checks", "ok"}, "Read-only checks; may execute installed diagnostic tools.", "只读检查；可能执行已安装的诊断工具。")},
		{Name: "ncgo_check", Description: "Validate every enabled Agent context plus service manifest consistency (read-only).", InputSchema: schemaObject([]string{"root"}, rootField("Service root with .ncgo/manifest.yaml or micro workspace root with ncgo.workspace"), outputTextJSONField()), Reference: ref("只读校验全部启用的 Agent 上下文和服务 manifest 一致性。", "ncgo check", []string{"root", "scope", "summary", "checks", "ok"}, "None; reads project metadata and generated Agent context.", "无；只读取项目元数据和生成的 Agent 上下文。")},
		{Name: "ncgo_new", Description: "Scaffold a new ncgo service or micro workspace. No dryRun is available because the authoritative hz/kitex generators do not expose a non-mutating render API; run in a disposable target directory when a preview is required.", InputSchema: schemaObject([]string{"name", "module"}, stringField("name", "Service name, e.g. \"user-api\""), stringField("module", "Go module path, e.g. \"github.com/acme/user-api\""), stringField("dir", "Target directory relative to the MCP workspace, default ./<name>"), enumField("mode", []string{manifest.ModeMono, manifest.ModeMicro}), enumField("kind", []string{manifest.KindHertz, manifest.KindKitex}), enumField("db", []string{"postgres", "none"}), stringArrayField("infra", "Infra add-ons (currently: redis)"), boolField("noGenerate", "Skip generator invocation"), stringField("aiTarget", "AI sync target for post-generation: all (default) | agents | claude | cursor | none"), boolField("noAutoSteps", "Skip automatic post-generation steps (go mod tidy, ai sync)"), stringField("preset", "Preset name: rule-center (Kitex with rate-limiting CRUD schema)"), stringField("ruleCenterAddr", "Rule-center gRPC address; sets Hertz source.type=rule_center"), stringField("template", "Template package name from registry"), stringField("templateDir", "Template package directory relative to the MCP workspace"), outputTextJSONField()), Reference: ref("生成新的 ncgo 服务或 micro workspace。", "ncgo new", []string{"dir", "mode", "ranGenerate", "autoSteps", "nextSteps"}, "Creates a project tree and may run hz/kitex, Go commands, and Agent sync. No dry-run; use a disposable target to preview.", "创建项目目录，并可能运行 hz/kitex、Go 命令和 Agent 同步。无 dry-run；预览时使用临时目标目录。")},
		{Name: "ncgo_add_domain", Description: "Add a domain usecase/repository to an ncgo project.", InputSchema: schemaObject([]string{"name", "root"}, rootField("Project root containing .ncgo/manifest.yaml"), stringField("name", "Domain name, e.g. \"device\""), boolField("force", "Overwrite existing generated files"), boolField("dryRun", "Preview intended writes without modifying files"), outputTextJSONField()), Reference: ref("向 ncgo 项目添加 domain usecase/repository。", "ncgo add domain", []string{"dryRun", "updated", "writtenPaths", "plan", "nextSteps"}, "Writes domain files and updates the manifest; dryRun reports planned effects.", "写入 domain 文件并更新 manifest；dryRun 返回计划中的 effects。")},
		{Name: "ncgo_ai_init_claude", Description: "Bootstrap the hand-authored .claude starter set for a repository.", InputSchema: schemaObject([]string{"root"}, rootField("Repository root where .claude/ should be bootstrapped"), enumField("preset", []string{ai.InitPresetMinimal, ai.InitPresetTeam}), boolField("force", "Overwrite existing starter files"), boolField("dryRun", "Report without writing"), outputTextJSONField()), Reference: ref("为仓库初始化手写 `.claude` 起始文件。", "ncgo ai init claude", []string{"written", "skipped", "notes", "nextSteps"}, "Writes Claude starter files; dryRun reports planned writes.", "写入 Claude 起始文件；dryRun 返回计划写入。")},
		{Name: "ncgo_ai_sync", Description: "Render AI context files for an ncgo service or micro workspace.", InputSchema: schemaObject([]string{"root"}, rootField("Service root with .ncgo/manifest.yaml or micro workspace root with ncgo.workspace"), enumField("target", []string{ai.TargetAll, ai.TargetAgents, ai.TargetClaude, ai.TargetCursor}), enumField("lang", []string{ai.LangEN, ai.LangZhCN}), boolField("force", "Overwrite unmanaged files"), boolField("dryRun", "Report without writing"), outputTextJSONField()), Reference: ref("为 ncgo 服务或 micro workspace 渲染 AI 上下文文件。", "ncgo ai sync", []string{"target", "written", "skipped", "notes", "scope", "sourceRef", "workspace", "nextSteps"}, "Writes managed Agent context files; dryRun reports planned writes.", "写入托管的 Agent 上下文文件；dryRun 返回计划写入。")},
		{Name: "ncgo_ai_context", Description: "Scan real code and return structured context (domains/methods/anchors/consistency) for an ncgo service.", InputSchema: schemaObject([]string{"root"}, rootField("Service root containing .ncgo/manifest.yaml"), outputTextJSONField()), Reference: ref("扫描真实代码并返回 domain、method、anchor 与一致性上下文。", "", []string{"root", "domains", "methods", "anchors", "issues"}, "None; this is an MCP-only read model over the project.", "无；这是面向项目的 MCP 专用只读模型。")},
		{Name: "ncgo_i18n_report", Description: "Read the generated i18n report for a project and return structured payload for agents.", InputSchema: schemaObject([]string{"root"}, rootField("Project root"), outputTextJSONField()), Reference: ref("读取生成的 i18n 报告并返回适合 Agent 的结构化数据。", "ncgo i18n report", []string{"root", "sourceLocale", "localesDir", "statusPath", "glossaryPath", "reportPathJSON", "reportPathMarkdown", "schema", "report", "nextSteps"}, "None; reads generated i18n artifacts.", "无；读取生成的 i18n 产物。")},
		{Name: "ncgo_i18n_check", Description: "Evaluate the generated i18n report for dev or release workflows.", InputSchema: schemaObject([]string{"root"}, rootField("Project root"), enumField("mode", []string{mcpI18NCheckDev, mcpI18NCheckRelease}), outputTextJSONField()), Reference: ref("按 dev 或 release 模式评估生成的 i18n 报告。", "ncgo i18n check", []string{"root", "mode", "ok", "sourceLocale", "schema", "summary", "failures", "warnings", "nextSteps"}, "None; evaluates existing i18n artifacts.", "无；评估已有 i18n 产物。")},
		{Name: "ncgo_protolint", Description: "Lint selected .proto files with ncgo's Proto I/O rules and return structured diagnostics.", InputSchema: schemaObject([]string{"root"}, rootField("Import root used to resolve the proto files"), stringArrayField("files", "Optional proto entry files relative to root and confined to it; omit to auto-discover from an ncgo service or micro workspace"), stringArrayField("rules", "Optional rule IDs to run"), stringArrayField("ignoreRules", "Optional rule IDs to suppress from the returned diagnostics"), stringArrayField("ignoreFiles", "Optional proto files whose diagnostics should be suppressed"), outputTextJSONSARIFField()), Reference: ref("使用 ncgo Proto I/O 规则检查 `.proto` 文件并返回结构化诊断。", "ncgo protolint", []string{"root", "files", "rulesRun", "ignoredRules", "ignoredFiles", "ok", "summary", "diagnostics"}, "None; parses proto sources without writing.", "无；只解析 proto 源文件。")},
		{Name: "ncgo_add_infra", Description: "Install an optional infrastructure add-on into an ncgo project.", InputSchema: schemaObject([]string{"root", "kind"}, rootField("Project root"), enumField("kind", infra.SupportedKinds()), boolField("force", "Overwrite existing generated add-on file"), boolField("wire", "Opt-in: update generated server/client wiring when supported"), boolField("dryRun", "Preview intended add-on writes and --wire changes without modifying files"), outputTextJSONField()), Reference: ref("向 ncgo 项目安装可选基础设施插件。", "ncgo add infra", []string{"dryRun", "updated", "writtenPath", "writtenPaths", "wiredPaths", "plan", "nextSteps"}, "Writes add-on, manifest, container, compose, and optional wiring files; dryRun reports the full plan.", "写入插件、manifest、容器、compose 和可选 wiring 文件；dryRun 返回完整计划。")},
		{Name: "ncgo_add_method", Description: "Insert a usecase method stub at ncgo anchors.", InputSchema: schemaObject([]string{"root", "spec"}, rootField("Project root"), stringField("spec", "<domain>.<Method>"), enumField("in", []string{method.LayerUsecase}), boolField("dryRun", "Validate and preview the target write without modifying files"), outputTextJSONField()), Reference: ref("在 ncgo anchor 处插入 usecase 方法桩。", "ncgo add method", []string{"path", "domain", "method", "dryRun", "nextSteps"}, "Writes one usecase file; dryRun validates and renders without writing.", "写入一个 usecase 文件；dryRun 会完成校验和渲染但不写入。")},
		{Name: "ncgo_add_rpc_method", Description: "Append an RPC method stub (signature copied from the already-generated handler) to an existing top-level usecase.go.", InputSchema: schemaObject([]string{"root", "service", "rpc"}, rootField("Project root"), stringField("service", "Service name, must match .ncgo/manifest.yaml service.name"), stringField("rpc", "RPC method name; must already exist in the generated handler (run make update/hz update first)"), boolField("dryRun", "Validate, discover the generated signature, and preview the target write without modifying files"), outputTextJSONField()), Reference: ref("把已生成 handler 的 RPC 签名追加到顶层 usecase.go。", "ncgo add rpc-method", []string{"path", "service", "method", "dryRun", "nextSteps"}, "Writes one usecase file; dryRun still discovers and validates the generated handler signature.", "写入一个 usecase 文件；dryRun 仍会发现并校验生成的 handler 签名。")},
		{Name: "ncgo_add_rule_center", Description: "Add rule-center gRPC client for rate-limit rule queries to an existing Hertz or Kitex service.", InputSchema: schemaObject([]string{"root", "addr"}, rootField("Project root containing .ncgo/manifest.yaml"), stringField("addr", "Rule-center gRPC address (e.g., localhost:8888)"), boolField("force", "Overwrite existing generated files"), boolField("dryRun", "Preview without modifying files"), outputTextJSONField()), Reference: ref("为现有 Hertz 或 Kitex 服务添加 rule-center gRPC 客户端。", "ncgo add rule-center", []string{"dryRun", "writtenPaths", "plannedPaths", "nextSteps"}, "Writes the client and configuration and may wire Hertz server code; dryRun reports exact planned paths.", "写入客户端与配置，并可能连接 Hertz server 代码；dryRun 返回精确计划路径。")},
		{Name: "ncgo_upgrade", Description: "Plan ncgo metadata upgrades for a project or micro workspace. MCP is preview-only and never applies the plan.", InputSchema: schemaObject([]string{"root"}, rootField("Project root containing .ncgo/manifest.yaml or ncgo.workspace"), outputTextJSONField()), Reference: ref("规划项目或 micro workspace 的 ncgo 元数据升级；MCP 始终只预览。", "ncgo upgrade", []string{"upToDate", "root", "mode", "path", "plan", "items"}, "None through MCP; returns a metadata upgrade plan only. CLI can apply the upgrade.", "MCP 调用无副作用；仅返回元数据升级计划。CLI 可应用升级。")},
		{Name: "ncgo_import", Description: "Preview the .ncgo/manifest.yaml an existing hz/kitex project would import. Always preview-only via MCP; never writes files (run `ncgo import` locally to write).", InputSchema: schemaObject([]string{"root"}, rootField("Existing Go project root containing go.mod"), enumField("kind", []string{manifest.KindHertz, manifest.KindKitex})), Reference: ref("预览现有 hz/kitex 项目将生成的 `.ncgo/manifest.yaml`；MCP 不写文件。", "ncgo import", []string{"preview", "module", "mode", "service"}, "None through MCP; CLI import writes the manifest.", "MCP 调用无副作用；CLI import 会写入 manifest。")},
		{Name: "ncgo_extract_domain", Description: "Plan extraction of a domain from a mono service into a separate micro service.", InputSchema: schemaObject([]string{"name", "root"}, rootField("Micro workspace root containing ncgo.workspace"), stringField("name", "Domain name to extract, e.g. \"user\""), stringField("to", "Target service directory confined to Root; defaults to services/<name>"), outputTextJSONField()), Reference: ref("规划把 mono 服务的 domain 提取为独立 micro 服务。", "ncgo extract domain", []string{"name", "targetModule", "toDir", "sources", "nextSteps"}, "None through MCP; returns a plan. CLI requires --apply to copy files.", "MCP 调用无副作用；仅返回计划。CLI 需要 --apply 才复制文件。")},
		{Name: "ncgo_export_templates", Description: "Export code templates from an existing ncgo project to template/<kind>-template/. No dryRun is available because export derives and writes the authoritative reusable template set as one operation; copy the project before previewing.", InputSchema: schemaObject([]string{"root"}, rootField("Project root containing .ncgo/manifest.yaml"), enumField("kind", []string{manifest.KindHertz, manifest.KindKitex}), outputTextJSONField()), Reference: ref("从现有项目导出可复用模板到 `template/`。", "ncgo export templates", []string{"outputDir", "kind", "templates", "idls"}, "Writes derived template and IDL snapshots under template/; no dry-run.", "在 template/ 下写入派生模板和 IDL 快照；无 dry-run。")},
		{Name: "ncgo_template_list", Description: "Refresh the registry cache with git clone/pull, then list available template packages. No dryRun is available because the registry client has no offline or read-only listing mode.", InputSchema: schemaObject(nil, stringField("registry", "Template registry URL (default: NCGO_REGISTRY env or official registry)"), outputTextJSONField()), Reference: ref("刷新 git registry 缓存并列出模板包。", "ncgo template list", []string{"templates"}, "Runs git clone/pull and mutates the local registry cache; requires network for remote registries.", "运行 git clone/pull 并修改本地 registry 缓存；远程 registry 需要网络。")},
		{Name: "ncgo_template_pull", Description: "Fetch a template package into the local registry cache. No dryRun is available because resolving the remote package and populating the cache are performed atomically by the registry client; ncgo_template_list can inspect names but also refreshes that cache.", InputSchema: schemaObject([]string{"name"}, stringField("name", "Template package name to pull, e.g. \"base-kitex\""), stringField("registry", "Template registry URL (default: NCGO_REGISTRY env or official registry)"), outputTextJSONField()), Reference: ref("把指定模板包拉取到本地 registry 缓存。", "ncgo template pull", []string{"name", "dir"}, "Runs git clone/pull and updates the local registry cache; no dry-run.", "运行 git clone/pull 并更新本地 registry 缓存；无 dry-run。")},
		{Name: "ncgo_add_rpc", Description: "Add a Kitex RPC service to a micro workspace.", InputSchema: schemaObject([]string{"name", "root"}, rootField("Micro workspace root containing ncgo.workspace"), stringField("name", "RPC service name, e.g. \"payment-rpc\""), stringField("module", "Go module path; defaults to <workspace.module>/services/<name>"), stringField("dir", "Service directory confined to Root; defaults to services/<name>"), boolField("noGenerate", "Skip kitex invocation"), boolField("dryRun", "Preview without modifying files"), enumField("preset", []string{"rule-center"}), stringField("template", "Template package name from registry (kitex or micro kind)"), stringField("templateDir", "Template package directory relative to the MCP workspace (kitex or micro kind)"), outputTextJSONField()), Reference: ref("向 micro workspace 添加 Kitex RPC 服务。", "ncgo add rpc", []string{"serviceDir", "serviceRel", "module", "updated", "ranGenerate", "dryRun", "plan", "nextSteps"}, "Creates a service, updates the workspace, and may run kitex. Unlike the CLI, MCP does not add per-service .claude directories; dryRun reports the plan.", "创建服务、更新 workspace，并可能运行 kitex。与 CLI 不同，MCP 不添加服务级 `.claude` 目录；dryRun 返回计划。")},
		{Name: "ncgo_add_bff", Description: "Add a Hertz BFF service to a micro workspace.", InputSchema: schemaObject([]string{"name", "root"}, rootField("Micro workspace root containing ncgo.workspace"), stringField("name", "BFF service name, e.g. \"user-api\""), stringField("module", "Go module path; defaults to <workspace.module>/services/<name>"), stringField("dir", "Service directory confined to Root; defaults to services/<name>"), boolField("noGenerate", "Skip hz invocation"), boolField("dryRun", "Preview without modifying files"), enumField("preset", []string{"rule-center"}), stringField("template", "Template package name from registry (hertz or micro kind)"), stringField("templateDir", "Template package directory relative to the MCP workspace (hertz or micro kind)"), outputTextJSONField()), Reference: ref("向 micro workspace 添加 Hertz BFF 服务。", "ncgo add bff", []string{"serviceDir", "serviceRel", "module", "updated", "ranGenerate", "dryRun", "plan", "nextSteps"}, "Creates a service, updates the workspace, and may run hz. Unlike the CLI, MCP does not add per-service .claude directories; dryRun reports the plan.", "创建服务、更新 workspace，并可能运行 hz。与 CLI 不同，MCP 不添加服务级 `.claude` 目录；dryRun 返回计划。")},
	}
	for i := range tools {
		applyToolSafety(&tools[i], mcpToolSafety[tools[i].Name])
	}
	return tools
}

var mcpToolSafety = map[string]toolSafety{
	"ncgo_version":          {ReadOnly: true, Idempotent: true},
	"ncgo_doctor":           {ReadOnly: true, Idempotent: true, ExternalProcess: true},
	"ncgo_check":            {ReadOnly: true, Idempotent: true},
	"ncgo_new":              {Network: true, ExternalProcess: true},
	"ncgo_add_domain":       {Destructive: true, SupportsDryRun: true},
	"ncgo_ai_init_claude":   {Destructive: true, SupportsDryRun: true},
	"ncgo_ai_sync":          {Destructive: true, SupportsDryRun: true},
	"ncgo_ai_context":       {ReadOnly: true, Idempotent: true},
	"ncgo_i18n_report":      {ReadOnly: true, Idempotent: true},
	"ncgo_i18n_check":       {ReadOnly: true, Idempotent: true},
	"ncgo_protolint":        {ReadOnly: true, Idempotent: true},
	"ncgo_add_infra":        {Destructive: true, SupportsDryRun: true},
	"ncgo_add_method":       {SupportsDryRun: true},
	"ncgo_add_rpc_method":   {SupportsDryRun: true},
	"ncgo_add_rule_center":  {Destructive: true, SupportsDryRun: true},
	"ncgo_upgrade":          {ReadOnly: true, Idempotent: true},
	"ncgo_import":           {ReadOnly: true, Idempotent: true},
	"ncgo_extract_domain":   {ReadOnly: true, Idempotent: true},
	"ncgo_export_templates": {Destructive: true, Idempotent: true},
	"ncgo_template_list":    {Destructive: true, Network: true, ExternalProcess: true},
	"ncgo_template_pull":    {Destructive: true, Network: true, ExternalProcess: true},
	"ncgo_add_rpc":          {ExternalProcess: true, SupportsDryRun: true},
	"ncgo_add_bff":          {ExternalProcess: true, SupportsDryRun: true},
}

func applyToolSafety(t *tool, safety toolSafety) {
	t.Annotations = toolAnnotations{
		ReadOnlyHint:    safety.ReadOnly,
		DestructiveHint: safety.Destructive,
		IdempotentHint:  safety.Idempotent,
		OpenWorldHint:   safety.Network,
	}
	t.Meta = map[string]any{
		"io.github.byx-darwin.ncgo/toolBehavior": map[string]bool{
			"networkAccess":   safety.Network,
			"externalProcess": safety.ExternalProcess,
			"supportsDryRun":  safety.SupportsDryRun,
		},
	}
	t.OutputSchema = mcpResultEnvelopeSchema(t.Reference.ResultFields)
}

func mcpResultEnvelopeSchema(resultFields []string) map[string]any {
	dataProperties := map[string]any{}
	for _, name := range resultFields {
		if !reservedEnvelopeField(name) && !strings.Contains(name, ".") {
			dataProperties[name] = map[string]any{}
		}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"schemaVersion", "ok", "data", "effects", "diagnostics", "error", "nextSteps"},
		"properties": map[string]any{
			"schemaVersion": map[string]any{"type": "string", "const": mcpResultSchemaVersion},
			"ok":            map[string]any{"type": "boolean"},
			"data":          map[string]any{"type": "object", "properties": dataProperties},
			"effects": map[string]any{"type": "array", "items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"kind", "action", "status"},
				"properties": map[string]any{
					"kind":   map[string]any{"type": "string"},
					"action": map[string]any{"type": "string"},
					"status": map[string]any{"type": "string", "enum": []string{"planned", "applied", "skipped", "failed"}},
					"path":   map[string]any{"type": "string"},
					"detail": map[string]any{"type": "string"},
				},
			}},
			"diagnostics": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			"error": map[string]any{"anyOf": []any{
				map[string]any{"type": "null"},
				map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"code", "message", "retryable", "remediation"},
					"properties": map[string]any{
						"code":        map[string]any{"type": "string"},
						"message":     map[string]any{"type": "string"},
						"retryable":   map[string]any{"type": "boolean"},
						"path":        map[string]any{"type": "string"},
						"remediation": map[string]any{"type": "string"},
					},
				},
			}},
			"nextSteps": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
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
	case "ncgo_new":
		return callNew(ctx, p.Arguments, s.NCGOVersion, s.AssetsVersion)
	case "ncgo_add_domain":
		return callAddDomain(p.Arguments)
	case "ncgo_doctor":
		return s.callDoctor(ctx, p.Arguments)
	case "ncgo_check":
		return callCheck(p.Arguments)
	case "ncgo_ai_init_claude":
		return callAIInitClaude(p.Arguments)
	case "ncgo_ai_sync":
		return callAISync(p.Arguments)
	case "ncgo_ai_context":
		return callAIContext(p.Arguments)
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
	case "ncgo_add_rpc_method":
		return callAddRPCMethod(p.Arguments)
	case "ncgo_add_rule_center":
		return callAddRuleCenter(p.Arguments)
	case "ncgo_upgrade":
		return callUpgrade(p.Arguments, s.NCGOVersion, s.AssetsVersion)
	case "ncgo_import":
		return callImport(p.Arguments, s.NCGOVersion, s.AssetsVersion)
	case "ncgo_extract_domain":
		return callExtractDomain(p.Arguments)
	case "ncgo_export_templates":
		return callExportTemplates(p.Arguments)
	case "ncgo_template_list":
		return callTemplateList(ctx, p.Arguments)
	case "ncgo_template_pull":
		return callTemplatePull(ctx, p.Arguments)
	case "ncgo_add_rpc":
		return callAddRPC(ctx, p.Arguments, s.NCGOVersion, s.AssetsVersion)
	case "ncgo_add_bff":
		return callAddBFF(ctx, p.Arguments, s.NCGOVersion, s.AssetsVersion)
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

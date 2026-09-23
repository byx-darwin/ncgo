// Package reference renders the generated MCP and CLI capability reference.
package reference

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/byx-darwin/ncgo/internal/cli"
	"github.com/byx-darwin/ncgo/internal/mcp"
)

type locale string

const (
	localeEN locale = "en"
	localeZH locale = "zh-CN"
)

type target struct {
	Path        string
	Locale      locale
	FrontMatter string
}

var targets = []target{
	{Path: "docs/mcp-reference.md", Locale: localeEN},
	{Path: "docs/mcp-reference.zh-CN.md", Locale: localeZH},
	{Path: "website/docs/reference/mcp.md", Locale: localeEN, FrontMatter: "---\nsidebar_position: 2\ntitle: MCP Reference\n---\n\n"},
	{Path: "website/i18n/zh-CN/docusaurus-plugin-content-docs/current/reference/mcp.md", Locale: localeZH, FrontMatter: "---\nsidebar_position: 2\ntitle: MCP 参考\n---\n\n"},
}

type localizedText struct{ EN, ZH string }

// cliOnlyReasons is intentionally only the exception set. The complete CLI
// inventory is always discovered from the live Cobra tree.
var cliOnlyReasons = map[string]localizedText{
	"ncgo add kitex-client":     {"Not exposed through MCP yet; use the CLI dry-run/plan before applying.", "尚未通过 MCP 暴露；请先使用 CLI dry-run/plan，再应用。"},
	"ncgo completion":           {"Shell integration is intentionally CLI-only.", "Shell 集成有意只通过 CLI 提供。"},
	"ncgo help":                 {"Interactive command help is intentionally CLI-only.", "交互式命令帮助有意只通过 CLI 提供。"},
	"ncgo mcp serve":            {"Starts the MCP transport itself and therefore is not an MCP tool.", "用于启动 MCP transport 本身，因此不是 MCP tool。"},
	"ncgo test rate-limit e2e":  {"Orchestrates local dependencies, traffic, and report files; intentionally CLI-only.", "编排本地依赖、流量和报告文件；有意只通过 CLI 提供。"},
	"ncgo test rate-limit run":  {"Runs local traffic tools and network calls; intentionally CLI-only.", "运行本地流量工具和网络调用；有意只通过 CLI 提供。"},
	"ncgo test rate-limit seed": {"Mutates a local test database; intentionally CLI-only.", "修改本地测试数据库；有意只通过 CLI 提供。"},
}

// parityNotes documents only operation-level differences for otherwise mapped
// capabilities. Tool contracts remain the source of truth for full behavior.
var parityNotes = map[string]localizedText{
	"ncgo new":            {"Shared scaffold capability; CLI additionally supports interactive module collection and --idl, which MCP does not expose.", "共享脚手架能力；CLI 还支持交互式收集 module 和 --idl，MCP 未暴露这两项能力。"},
	"ncgo add method":     {"Both apply the edit; MCP additionally supports dryRun, while CLI has no preview flag.", "两者都会应用编辑；MCP 额外支持 dryRun，CLI 没有预览参数。"},
	"ncgo add rpc-method": {"Both apply the edit; MCP additionally supports dryRun, while CLI has no preview flag.", "两者都会应用编辑；MCP 额外支持 dryRun，CLI 没有预览参数。"},
	"ncgo upgrade":        {"MCP is preview-only; CLI can apply metadata updates.", "MCP 仅提供预览；CLI 可应用元数据更新。"},
	"ncgo import":         {"MCP is preview-only; CLI writes .ncgo/manifest.yaml.", "MCP 仅提供预览；CLI 会写入 .ncgo/manifest.yaml。"},
	"ncgo extract domain": {"MCP is plan-only; CLI can copy files with --apply.", "MCP 仅提供计划；CLI 可通过 --apply 复制文件。"},
	"ncgo add rpc":        {"Both scaffold the service; CLI additionally writes per-service .claude directories.", "两者都会生成服务脚手架；CLI 还会为每个服务写入 .claude 目录。"},
	"ncgo add bff":        {"Both scaffold the service; CLI additionally writes per-service .claude directories.", "两者都会生成服务脚手架；CLI 还会为每个服务写入 .claude 目录。"},
}

// Generate writes all generated references, or checks them without mutation.
func Generate(root string, check bool) error {
	tools, commands, err := catalog()
	if err != nil {
		return err
	}
	var stale []string
	for _, target := range targets {
		body, err := render(target.Locale, target.FrontMatter, tools, commands)
		if err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(target.Path))
		if check {
			current, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(current, body) {
				stale = append(stale, target.Path)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return err
		}
	}
	if len(stale) > 0 {
		return fmt.Errorf("generated MCP reference is stale: %s; run `go run ./internal/reference/cmd/generate`", strings.Join(stale, ", "))
	}
	return nil
}

func catalog() ([]mcp.ReferenceTool, []cli.ReferenceCommand, error) {
	tools, err := mcp.ReferenceTools()
	if err != nil {
		return nil, nil, err
	}
	commands := cli.ReferenceCommands()
	commandSet := make(map[string]bool, len(commands))
	for _, command := range commands {
		commandSet[command.Path] = true
	}
	mapped := make(map[string]string)
	for _, tool := range tools {
		for _, path := range tool.CLI {
			if !commandSet[path] {
				return nil, nil, fmt.Errorf("MCP tool %s maps unknown CLI command %q", tool.Name, path)
			}
			if previous := mapped[path]; previous != "" {
				return nil, nil, fmt.Errorf("CLI command %q maps both %s and %s", path, previous, tool.Name)
			}
			mapped[path] = tool.Name
		}
	}
	for path := range cliOnlyReasons {
		if !commandSet[path] {
			return nil, nil, fmt.Errorf("CLI-only reference names unknown command %q", path)
		}
	}
	for _, command := range commands {
		if mapped[command.Path] == "" {
			if _, ok := cliOnlyReasons[command.Path]; !ok {
				return nil, nil, fmt.Errorf("CLI command %q is neither mapped to MCP nor documented CLI-only", command.Path)
			}
		}
	}
	return tools, commands, nil
}

func render(lang locale, frontMatter string, tools []mcp.ReferenceTool, commands []cli.ReferenceCommand) ([]byte, error) {
	var b strings.Builder
	b.WriteString(frontMatter)
	b.WriteString("<!-- Code generated by go run ./internal/reference/cmd/generate; DO NOT EDIT. -->\n\n")
	if lang == localeZH {
		b.WriteString("# MCP 工具参考\n\n本文档直接由已注册的 MCP metadata 与真实 Cobra 命令树生成。修改工具或 CLI 后请重新运行生成器。\n\n")
		b.WriteString("## 通用结果契约\n\n每次调用都保留 `content[0].text`，并返回 `ncgo.mcp.result/v1` 的 `structuredContent`：`schemaVersion`、`ok`、`data`、`effects`、`diagnostics`、`error`、`nextSteps`。失败时 `error` 含稳定的 `code`、`message`、`retryable`、可选 `path` 与 `remediation`。下方“稳定结果字段”列出工具公开字段；非保留字段进入 `data`，`ok`、`effects`、`diagnostics` 与 `nextSteps` 等保留字段位于 envelope。迁移期同名 legacy 顶层字段仍保留。\n\n")
	} else {
		b.WriteString("# MCP Tool Reference\n\nThis document is generated directly from registered MCP metadata and the live Cobra command tree. Regenerate it after changing a tool or CLI command.\n\n")
		b.WriteString("## Common result contract\n\nEvery call preserves `content[0].text` and returns `structuredContent` using `ncgo.mcp.result/v1`: `schemaVersion`, `ok`, `data`, `effects`, `diagnostics`, `error`, and `nextSteps`. Failures expose stable `code`, `message`, `retryable`, optional `path`, and `remediation` fields. “Stable result fields” below lists the tool's public fields: non-reserved fields live in `data`, while reserved `ok`, `effects`, `diagnostics`, and `nextSteps` fields live in the envelope. Matching legacy top-level fields remain during migration.\n\n")
	}
	if lang == localeZH {
		b.WriteString("## 已注册工具\n\n")
	} else {
		b.WriteString("## Registered tools\n\n")
	}
	for _, tool := range tools {
		renderTool(&b, lang, tool)
	}
	renderMatrix(&b, lang, tools, commands)
	return []byte(b.String()), nil
}

func renderTool(b *strings.Builder, lang locale, tool mcp.ReferenceTool) {
	fmt.Fprintf(b, "### `%s`\n\n", tool.Name)
	if lang == localeZH {
		b.WriteString(escapeMDX(tool.DescriptionZH))
	} else {
		b.WriteString(escapeMDX(tool.Description))
	}
	b.WriteString("\n\n")
	cliText := "MCP only"
	if lang == localeZH {
		cliText = "仅 MCP"
	}
	if len(tool.CLI) > 0 {
		quoted := make([]string, len(tool.CLI))
		for i, path := range tool.CLI {
			quoted[i] = "`" + path + "`"
		}
		cliText = strings.Join(quoted, ", ")
	}
	if lang == localeZH {
		fmt.Fprintf(b, "- CLI 对应：%s\n- 输出格式：`%s`\n- 稳定结果字段：%s\n- 安全/执行提示：%s\n- 副作用：%s\n\n", cliText, strings.Join(tool.OutputFormats, "|"), codeList(tool.ResultFields), safetyText(lang, tool.Safety), escapeMDX(tool.SideEffectsZH))
		b.WriteString("| 输入 | 必填 | 类型/取值 | 说明 |\n| --- | --- | --- | --- |\n")
	} else {
		fmt.Fprintf(b, "- CLI equivalent: %s\n- Output formats: `%s`\n- Stable result fields: %s\n- Safety/execution hints: %s\n- Side effects: %s\n\n", cliText, strings.Join(tool.OutputFormats, "|"), codeList(tool.ResultFields), safetyText(lang, tool.Safety), escapeMDX(tool.SideEffects))
		b.WriteString("| Input | Required | Type / values | Description |\n| --- | --- | --- | --- |\n")
	}
	properties, _ := tool.InputSchema["properties"].(map[string]any)
	required := stringSet(tool.InputSchema["required"])
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		if lang == localeZH {
			b.WriteString("| _无_ | — | — | — |\n")
		} else {
			b.WriteString("| _None_ | — | — | — |\n")
		}
	}
	for _, key := range keys {
		property, _ := properties[key].(map[string]any)
		yes, no := "yes", "no"
		if lang == localeZH {
			yes, no = "是", "否"
		}
		req := no
		if required[key] {
			req = yes
		}
		fmt.Fprintf(b, "| `%s` | %s | %s | %s |\n", key, req, escapeTable(propertyType(property)), escapeTable(fmt.Sprint(property["description"])))
	}
	b.WriteString("\n")
}

func renderMatrix(b *strings.Builder, lang locale, tools []mcp.ReferenceTool, commands []cli.ReferenceCommand) {
	toolByCLI := map[string]string{}
	var mcpOnly []string
	for _, tool := range tools {
		if len(tool.CLI) == 0 {
			mcpOnly = append(mcpOnly, tool.Name)
		}
		for _, path := range tool.CLI {
			toolByCLI[path] = tool.Name
		}
	}
	if lang == localeZH {
		b.WriteString("## CLI ↔ MCP 能力矩阵\n\n| CLI 能力 | MCP 工具 | 状态 / 原因 |\n| --- | --- | --- |\n")
	} else {
		b.WriteString("## CLI ↔ MCP capability matrix\n\n| CLI capability | MCP tool | Status / reason |\n| --- | --- | --- |\n")
	}
	for _, command := range commands {
		if name := toolByCLI[command.Path]; name != "" {
			status := "Available through both interfaces; see the tool contract above for MCP-specific preview/apply behavior."
			if lang == localeZH {
				status = "CLI 与 MCP 均可用；MCP 特有的预览/应用语义见上方工具契约。"
			}
			if note, ok := parityNotes[command.Path]; ok {
				status = note.EN
				if lang == localeZH {
					status = note.ZH
				}
			}
			fmt.Fprintf(b, "| `%s` | `%s` | %s |\n", command.Path, name, status)
			continue
		}
		reason := cliOnlyReasons[command.Path].EN
		if lang == localeZH {
			reason = cliOnlyReasons[command.Path].ZH
		}
		fmt.Fprintf(b, "| `%s` | — | %s |\n", command.Path, reason)
	}
	for _, name := range mcpOnly {
		reason := "MCP-only structured project context; there is no direct CLI equivalent."
		if lang == localeZH {
			reason = "MCP 专用的结构化项目上下文；没有直接 CLI 等价命令。"
		}
		fmt.Fprintf(b, "| — | `%s` | %s |\n", name, reason)
	}
}

func stringSet(value any) map[string]bool {
	out := map[string]bool{}
	switch values := value.(type) {
	case []string:
		for _, item := range values {
			out[item] = true
		}
	case []any:
		for _, item := range values {
			out[fmt.Sprint(item)] = true
		}
	}
	return out
}

func propertyType(property map[string]any) string {
	if values, ok := property["enum"].([]string); ok && len(values) > 0 {
		return strings.Join(values, " | ")
	}
	if values, ok := property["enum"].([]any); ok && len(values) > 0 {
		parts := make([]string, len(values))
		for i, value := range values {
			parts[i] = fmt.Sprint(value)
		}
		return strings.Join(parts, " | ")
	}
	if property["type"] == "array" {
		items, _ := property["items"].(map[string]any)
		return fmt.Sprintf("array<%v>", items["type"])
	}
	return fmt.Sprint(property["type"])
}

func safetyText(lang locale, safety mcp.ReferenceSafety) string {
	labels := []string{}
	add := func(enabled bool, en, zh string) {
		if enabled {
			if lang == localeZH {
				labels = append(labels, zh)
			} else {
				labels = append(labels, en)
			}
		}
	}
	add(safety.ReadOnly, "read-only", "只读")
	add(safety.Destructive, "destructive", "可覆盖/修改")
	add(safety.Idempotent, "idempotent", "幂等")
	add(safety.Network, "network", "网络")
	add(safety.ExternalProcess, "external process", "外部进程")
	add(safety.SupportsDryRun, "dry-run", "dry-run")
	if len(labels) == 0 {
		if lang == localeZH {
			return "写操作（无额外 hint）"
		}
		return "write operation (no additional hint)"
	}
	return strings.Join(labels, ", ")
}

func codeList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "`" + value + "`"
	}
	return strings.Join(quoted, ", ")
}

func escapeTable(value string) string {
	if value == "<nil>" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(escapeMDX(value), "|", "\\|"), "\n", " ")
}

func escapeMDX(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "<", "&lt;"), ">", "&gt;")
}

// FindRoot finds the repository root from a starting directory.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}

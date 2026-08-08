# ncgo add rpc/bff 支持 --template/--template-dir + micro 工作区级模版消费 — Design Spec

- **Date**: 2026-08-09
- **Workflow**: wf-2026-08-09-057（gf-workflow full mode）
- **Status**: Approved（brainstorming 阶段用户批准）
- **Related specs**: `docs/superpowers/specs/2026-08-08-template-registry-closed-loop-design.md`
- **Issue**: #57

## 1. Background

PR #55（wf-2026-08-08-001）落地了 mono 路径的模版包消费闭环：`ncgo new --template <name>` / `--template-dir <dir>` 可消费官方 registry 的 `base-kitex`/`base-hertz` 模版包。设计文档 §11 明确两项后续迭代：

1. `ncgo add rpc` / `ncgo add bff` 模版支持（接口预留）
2. micro 工作区级别的模版

Issue #57 实现这两项：让微服务场景（`ncgo new --mode micro` + `ncgo add rpc/bff`）可消费官方模版包。

### 当前状态

| 路径 | 模版消费支持 |
|---|---|
| `ncgo new`（mono，hertz + kitex） | ✅ `--template` / `--template-dir`（PR #55） |
| `ncgo new --mode micro` | ❌ 工作区骨架由代码硬编码 |
| `ncgo add rpc` | ❌ 无 `--template`/`--template-dir`；有 `--preset` |
| `ncgo add bff` | ❌ 无 `--template`/`--template-dir`/`--preset` |

底层 `mono.Generate` 已支持 `TemplateDir`；`bff.Add`/`rpc.Add` 委托 `mono.Generate` 但未透传 `TemplateDir`。

## 2. 用户模型与关键澄清

延续 PR #55 的**"换名复制"**模型：模版 = 成熟项目快照，消费 = 原样沿用 + module/服务名替换。

Brainstorming 阶段关键澄清：

| 决策点 | 选择 | 理由 |
|---|---|---|
| micro 工作区骨架是否模版化 | ✅ 工作区级也模版化 | 用户明确选择；compose/scripts/README 也可定制 |
| 模版包结构 | 单一 micro 包 | 一个包包含工作区 + kitex + hertz 服务模板；用户视角简单，一致性由单包保证 |
| `kind` 字段 | `kind: micro` | 语义清晰：micro 包 = 微服务工作区包，与 mono 的 kitex/hertz 包对称 |
| add rpc/bff 接受哪些 kind | 两者都允许 | `add rpc` 接受 `kind: kitex` 或 `kind: micro`（提取 kitex 部分）；同理 bff |
| add bff 是否补 `--preset` | ✅ 补 | 与 add rpc 对称；三者互斥约束一致 |
| 实现方案 | 扩展 LoadPackage | 最小新增代码；复用现有 mono 管线；对调用方透明 |

## 3. Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│ CLI Layer                                                     │
│                                                               │
│  ncgo new --mode micro --template <name> | --template-dir <d> │
│  ncgo add rpc <name> --template <name> | --template-dir <d>   │
│  ncgo add bff <name> --template <name> | --template-dir <d>   │
│                    ↓                                          │
│  resolveTemplateDir(name, dir) → absolute dir path            │
│  (--template → registry cache; --template-dir → direct path)  │
└──────────────────────────────────────────────────────────────┘
                            ↓
┌──────────────────────────────────────────────────────────────┐
│ scaffoldtemplate.LoadPackage(dir, expectedKind)              │
│                                                               │
│  Read <dir>/template.yaml                                     │
│  Switch on (pkg.kind, expectedKind):                          │
│    (kitex,  kitex)  → load <dir>/kitex-template/  + idl/     │
│    (hertz,  hertz)  → load <dir>/hertz-template/  + idl/     │
│    (micro,  kitex)  → load <dir>/kitex-template/  + idl/kitex│
│    (micro,  hertz)  → load <dir>/hertz-template/  + idl/hertz│
│    (micro,  micro)  → load <dir>/workspace/                  │
│    mismatch       → clear error                               │
└──────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────┬────────────────────────────────────────┐
│ mono.Generate       │ micro.Generate                         │
│ (add rpc/bff 委托)   │ (ncgo new --mode micro)                │
│                     │                                        │
│ TemplateDir != ""   │ TemplateDir != ""                      │
│ → LoadPackage       │ → LoadPackage                          │
│   with expected     │   with expected                        │
│   kind=kitex/hertz  │   kind=micro                           │
│ → 替换 embedded     │ → overlay workspace/ templates         │
│   <kind>-template   │   onto built-in skeleton               │
│   + IDL             │                                        │
└─────────────────────┴────────────────────────────────────────┘
```

## 4. Template Package Spec

### Mono package (existing, unchanged)

```yaml
# template.yaml
name: base-kitex
kind: kitex           # kitex | hertz
description: ...
version: 1
skip_default_templates: []
```

```
base-kitex/
├── template.yaml
├── kitex-template/*.yaml
└── idl/*.proto
```

### Micro package (new)

```yaml
# template.yaml
name: my-micro
kind: micro           # NEW kind
description: Official micro workspace template
version: 1
```

```
my-micro/
├── template.yaml
├── workspace/              # workspace skeleton templates
│   ├── compose.yaml.tpl
│   ├── README.md.tpl
│   └── scripts/
│       └── *.sh.tpl
├── kitex-template/         # RPC service templates (for add rpc)
│   └── *.yaml
├── hertz-template/         # BFF service templates (for add bff)
│   └── *.yaml
└── idl/
    ├── kitex/*.proto       # kitex IDL templates
    └── hertz/              # hertz IDL templates
        └── app/*.proto
```

### Kind validation matrix

| CLI command | expectedKind | Accepts pkg.kind | Extracts |
|---|---|---|---|
| `ncgo new --kind kitex --template X` | kitex | kitex, micro | `kitex-template/` + `idl/` (or `idl/kitex/`) |
| `ncgo new --kind hertz --template X` | hertz | hertz, micro | `hertz-template/` + `idl/` (or `idl/hertz/`) |
| `ncgo new --mode micro --template X` | micro | micro only | `workspace/` |
| `ncgo add rpc --template X` | kitex | kitex, micro | `kitex-template/` + `idl/` (or `idl/kitex/`) |
| `ncgo add bff --template X` | hertz | hertz, micro | `hertz-template/` + `idl/` (or `idl/hertz/`) |

### Mutual exclusion

`--preset` ↔ `--template` ↔ `--template-dir`: at most one can be specified. Same constraint for `ncgo new`, `ncgo add rpc`, `ncgo add bff`.

## 5. CLI Changes

### `ncgo add rpc` — new flags

```
--template <name>       Template package name from registry
--template-dir <dir>    Template package local directory path
```

(`--preset` already exists)

### `ncgo add bff` — new flags

```
--preset <name>         Preset template name (e.g., rule-center) [NEW]
--template <name>       Template package name from registry
--template-dir <dir>    Template package local directory path
```

### `ncgo new --mode micro` — new flags

```
--template <name>       Template package name from registry (kind: micro)
--template-dir <dir>    Template package local directory path (kind: micro)
```

(`--template`/`--template-dir` already exist for mono mode)

### Resolution helper (shared)

```go
// resolveTemplateDir converts --template/--template-dir to an absolute directory path.
// Returns ("", nil) when neither flag is set (default embedded templates).
func resolveTemplateDir(template, templateDir string) (string, error) {
    if template != "" {
        cacheDir, err := registry.CachePath(template)
        if err != nil {
            return "", fmt.Errorf("template %q not found in cache; run 'ncgo template pull %s' first", template, template)
        }
        return cacheDir, nil
    }
    if templateDir != "" {
        abs, err := filepath.Abs(templateDir)
        if err != nil {
            return "", err
        }
        return abs, nil
    }
    return "", nil
}
```

### JSON output

`ncgo add rpc/bff --output json` adds `template` field (resolved template dir or name) when a template was used.

## 6. LoadPackage Extension

### Current signature

```go
func LoadPackage(dir string, kind string) (*Package, error)
```

### Extended resolution logic

```go
func resolveTemplateSubDirs(pkgKind, expectedKind, dir string) (templateDir, idlDir string, err error) {
    switch {
    case pkgKind == expectedKind && expectedKind == "micro":
        return "workspace", "", nil
    case pkgKind == expectedKind:
        // mono: (kitex, kitex) or (hertz, hertz)
        return "<kind>-template", "idl", nil
    case pkgKind == "micro" && expectedKind == "kitex":
        return "kitex-template", "idl/kitex", nil
    case pkgKind == "micro" && expectedKind == "hertz":
        return "hertz-template", "idl/hertz", nil
    default:
        return "", "", fmt.Errorf("template package kind %q does not match expected kind %q", pkgKind, expectedKind)
    }
}
```

### Backward compatibility

- Mono packages (`kind: kitex/hertz`) 不受影响 — `pkgKind == expectedKind` 路径
- Micro 包只在 expectedKind 是 micro 或从 micro 包提取服务模板时激活

### IDL fallback

从 micro 包提取 kitex/hertz 服务模板时，若 `idl/kitex/` 或 `idl/hertz/` 不存在但 `idl/` 存在（扁平结构），回退到 `idl/` 并发 deprecation warning。

## 7. Micro Workspace Template Consumption

### Consumption flow

```go
func Generate(opts Options) (*Result, error) {
    // 1. Write built-in skeleton as baseline (existing behavior)
    // ... writeWorkspace, writeReadme, WriteWorkspaceCompose, etc. ...

    // 2. If TemplateDir is set, overlay workspace templates
    if opts.TemplateDir != "" {
        pkg, err := scaffoldtemplate.LoadPackage(opts.TemplateDir, "micro")
        if err != nil { return nil, err }
        if err := overlayWorkspaceTemplates(dir, pkg, opts); err != nil {
            return nil, err
        }
    }

    return &Result{Dir: dir, NextSteps: nextSteps(opts)}, nil
}
```

### `overlayWorkspaceTemplates` behavior

```go
func overlayWorkspaceTemplates(dir string, pkg *scaffoldtemplate.Package, opts Options) error {
    // Walk <pkg.Dir>/workspace/ recursively
    // For each *.tpl file:
    //   1. Strip .tpl suffix to get target relative path
    //   2. Render template body with RenderData{Module, Name, NCGOVersion, AssetsVersion}
    //   3. Write to <dir>/<relative path> (overwriting built-in skeleton)
    // For each non-.tpl file:
    //   Copy as-is (overwriting built-in skeleton)
}
```

### Key decisions

- **Overlay, not replace**: 内置骨架先写入，模板 overlay。若模板包不含 `scripts/`，内置 scripts 保留。比要求每个 micro 模板 100% 完整更安全。
- **Variable substitution**: 与 mono 共用 `RenderData`（Module, Name 等）。workspace 模板用 `{{.Module}}`、`{{.Name}}`。
- **`.tpl` 后缀约定**: 标记需要变量替换的文件。无 `.tpl` 后缀的文件原样复制。避免对二进制/text 文件误替换。
- **IDL 不适用于工作区级**: micro 工作区无自身 IDL — IDL 属于服务层（add rpc/bff 创建）。

## 8. MCP Changes

### Existing tools — extend input schemas

**`ncgo_add_rpc`** — add optional parameters:
```json
{
  "template": { "type": "string", "description": "Template package name from registry" },
  "template_dir": { "type": "string", "description": "Template package local directory path" },
  "preset": { "type": "string", "description": "Preset template name" }
}
```

**`ncgo_add_bff`** — add optional parameters:
```json
{
  "template": { "type": "string", "description": "Template package name from registry" },
  "template_dir": { "type": "string", "description": "Template package local directory path" },
  "preset": { "type": "string", "description": "Preset template name" }
}
```

**`ncgo_new`** — micro mode 通过已有的 `mode` + `template`/`template_dir` 参数组合支持，schema 无需变化。

### Output contract

- `content[0].text` for human-readable summary; top-level structured fields for agents
- Add `template` field to structured output when a template was used

### Validation

- MCP handlers enforce same mutual exclusion as CLI: `template` ↔ `template_dir` ↔ `preset`
- Error messages follow existing pattern

## 9. Testing Strategy

### Unit tests

- **LoadPackage extension**:
  - `(kitex, kitex)` → root `kitex-template/`（regression）
  - `(micro, micro)` → `workspace/`
  - `(micro, kitex)` → `kitex-template/` + `idl/kitex/`
  - `(micro, hertz)` → `hertz-template/` + `idl/hertz/`
  - Kind mismatch → clear error
  - IDL fallback: `idl/kitex/` missing → `idl/` with warning

- **Micro workspace overlay**:
  - `overlayWorkspaceTemplates` with fixture → verify variable substitution, file overwrite, `.tpl` stripping
  - Missing `workspace/` → clear error
  - Empty `workspace/` → no-op

- **add rpc/bff with template**:
  - `rpc.Add` + kitex package → uses kitex templates
  - `rpc.Add` + micro package → extracts kitex templates
  - Same for `bff.Add` + hertz/micro
  - Kind mismatch → clear error

### CLI integration tests

- `ncgo add rpc --template X` / `--template-dir <d>`: success + error
- `ncgo add bff --template X` / `--template-dir <d>` / `--preset X`: success + error
- `ncgo new --mode micro --template X` / `--template-dir <d>`: success + error
- Mutual exclusion: `--preset` + `--template` → error

### MCP integration tests

- `ncgo_add_rpc` with `template`/`template_dir` parameters
- `ncgo_add_bff` with `template`/`template_dir`/`preset` parameters
- Mutual exclusion via MCP
- Output shape: `content[0].text` + structured fields

### Golden tests

- Add golden test for `ncgo new --mode micro --template-dir <micro-fixture>`
- Add golden test for `ncgo add rpc --template-dir <micro-fixture>`

### Smoke tests

- `scripts/smoke.sh`: `ncgo add rpc --template-dir` (with `--no-generate`)
- `scripts/smoke.sh`: `ncgo new --mode micro --template-dir`

## 10. Documentation Changes (EN + ZH aligned)

### README.md / README.zh-CN.md

- Command table: `ncgo add rpc` / `ncgo add bff` flag list adds `--preset`, `--template`, `--template-dir`
- `ncgo new --mode micro` flag list adds `--template`, `--template-dir`
- Brief note about micro template package kind

### docs/examples.md / docs/examples.zh-CN.md

- New section: "Micro workspace template consumption"
  - `ncgo new my-workspace --mode micro --template my-micro`
  - `ncgo add rpc user-rpc --template my-micro` (reuse same micro package)
  - `ncgo add rpc user-rpc --template base-kitex` (standalone mono package)
  - Micro template package structure explanation
- Update existing template registry section to mention micro packages

### Template package spec docs

- Add `kind: micro` specification
- Directory structure for micro packages
- Variable substitution reference for workspace templates

## 11. Scope Boundaries (本次不做)

- ❌ ServiceInfo loop/variable substitution for micro service templates（"换名复制"模型不需要）
- ❌ Template version locking / tag / commit-level references
- ❌ Registry repository creation / CI / automation（基础设施，非代码）
- ❌ Template package dependency management（如 micro pkg depends on base-kitex）
- ❌ Interactive template selection during `ncgo new --mode micro`（手动 flag only）

## 12. Risks

| 风险 | 缓解 |
|---|---|
| LoadPackage 职责变重 | `resolveTemplateSubDirs` 作为独立 helper，逻辑集中可测 |
| micro workspace overlay 覆盖内置文件不当 | overlay 只写模板包中存在的文件；内置骨架作为 fallback 保留 |
| IDL fallback 路径歧义 | 优先 `idl/kitex/`、`idl/hertz/`；fallback 到 `idl/` 时发 deprecation warning |
| micro 包缺少 `kitex-template/` 或 `hertz-template/` | LoadPackage 检查子目录存在；缺失时清晰报错 |
| 既有 add rpc/bff 用户无感知变化 | 不传 template flag 时行为完全不变（缺省路径不进入 LoadPackage） |

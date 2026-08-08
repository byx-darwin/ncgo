# rule-center 模版包自包含设计 — Design Spec

- **Date**: 2026-08-08
- **Workflow**: wf-2026-08-08-056（gf-workflow full mode）
- **Status**: Draft（brainstorming 阶段，待用户审批）
- **Related Issues**: #56
- **Related specs**: `2026-08-08-template-registry-closed-loop-design.md`

## 1. Background

官方模版仓库 `byx-darwin/ncgo-templates` 已发布 `rule-center` 模版包（13 个 ratelimit 模板 + `idl/rulecenter.proto`）。PR #55 落地了 `--template`/`--template-dir` 消费机制，但当前实现只负责"替换 `<kind>-template/` 模板 + 渲染 IDL"，缺少 preset 专属的附加逻辑。

**当前 `--preset rule-center` 在 `writeKitexTemplate` 中的附加行为：**

1. 跳过默认 `handler/server/usecase/repository.yaml`（防生成重复脚手架）
2. 额外写 `internal/db/schema/000002_rate_limit_rules.sql`
3. 复制 shared ratelimit fragments 为 `ratelimit_shared_*.yaml`
4. 写 `layout-rulecenter.yaml` 作为 `layout.yaml`
5. 特殊 IDL 处理（从 `ratelimit_proto.yaml` 写完整 proto）

**问题：** `overlayTemplatePackage` 不知道这些 preset 专属逻辑。当用户使用 `ncgo new --kind kitex --template rule-center` 时，产物与 `--preset rule-center` 不等价。

## 2. Goal

让 `ncgo new --kind kitex --template rule-center`（或 `--template-dir` 指向 rule-center 包）与 `ncgo new --kind kitex --preset rule-center` 消费体验等价——**模版包携带 preset 语义，自包含所有产物**。

## 3. Design Decisions

| 决策点 | 选择 | 备选（否决原因） |
|--------|------|------------------|
| 模型 | A：自包含模版包 | B：消费端按名称识别（脆弱、不可扩展）；C：template.yaml 声明 preset 字段（仍依赖 preset 逻辑） |
| 跳过默认模板 | template.yaml 显式声明 `skip_default_templates` | 同名覆盖即跳过（隐式）；按名称检测（不灵活） |
| 变量渲染 | `{{.Module}}` + `{{.ServiceName}}`（完整 `RenderData`） | 仅 `{{.Module}}`（不一致，未来扩展受限） |

**核心理念：** 模版包是成熟项目的完整快照（"换名复制"模型），应自包含所有产物，消费端不做特殊 case 逻辑。

## 4. 模版包结构扩展

```
rule-center/
├── template.yaml                # 元信息（扩展）
├── kitex-template/              # 模板 YAML（现有 + 扩展）
│   ├── ratelimit_handler.yaml
│   ├── ratelimit_server.yaml
│   ├── ratelimit_usecase.yaml
│   ├── ratelimit_repository.yaml
│   ├── ratelimit_proto.yaml
│   ├── ratelimit_shared_resolver.yaml      # 新增
│   ├── ratelimit_shared_resolver_test.yaml # 新增
│   ├── ratelimit_shared_store.yaml         # 新增
│   ├── ratelimit_shared_store_test.yaml    # 新增
│   └── ratelimit_shared_rule_center_client.yaml # 新增
├── idl/                         # IDL（现有）
│   └── rulecenter.proto
├── schema/                      # 新增：SQL schema 文件
│   └── 000002_rate_limit_rules.sql
└── layout.yaml                  # 新增：自定义 layout（可选，替换默认）
```

## 5. template.yaml 扩展

```yaml
name: rule-center
kind: kitex
description: 官方 Rule Center 模版（13 个 ratelimit 模板 + IDL + schema）
version: 1

# 新增字段：显式声明跳过哪些默认模板
skip_default_templates:
  - handler.yaml
  - server.yaml
  - usecase.yaml
  - repository.yaml
```

**语义：**
- `skip_default_templates` 列表中的文件名，在 `writeKitexTemplate` 阶段不写入
- 模版包提供的 `ratelimit_*.yaml` 替代了这些默认模板的功能

## 6. 消费端逻辑增强

### 6.1 `PackageMeta` 扩展

```go
type PackageMeta struct {
    Name                 string   `yaml:"name"`
    Kind                 string   `yaml:"kind"`
    Description          string   `yaml:"description"`
    Version              string   `yaml:"version"`
    SkipDefaultTemplates []string `yaml:"skip_default_templates"` // 新增
}
```

### 6.2 `overlayTemplatePackage` 增强

当前逻辑只做两件事：
1. 替换 `<kind>-template/*.yaml`
2. 渲染 `idl/*.proto`

增强后增加：

| 步骤 | 逻辑 |
|------|------|
| 1 | 读取 `template.yaml` 的 `skip_default_templates` 列表 |
| 2 | 通过 `Options.SkipDefaultTemplates` 传递给 `writeKitexTemplate` |
| 3 | 如果模版包有 `schema/` 目录，渲染并复制到 `internal/db/schema/` |
| 4 | 如果模版包有 `layout.yaml`，复制到 `template/layout.yaml`（替换默认） |
| 5 | `ratelimit_shared_*.yaml` 已在 `kitex-template/` 里，随步骤 1 复制 |

### 6.3 `Options` 扩展

```go
type Options struct {
    // ... existing fields ...
    SkipDefaultTemplates []string // 新增：从 template package 传递
}
```

### 6.4 `writeKitexTemplate` 修改

在复制默认模板时，检查 `opts.SkipDefaultTemplates`，跳过列表中的文件：

```go
if preset == "rule-center" && (name == "handler.yaml" || name == "server.yaml" || name == "usecase.yaml" || name == "repository.yaml") {
    continue
}
// 新增：同时检查 opts.SkipDefaultTemplates
if slices.Contains(opts.SkipDefaultTemplates, name) {
    continue
}
```

### 6.5 schema 和 layout 复制

```go
// schema 文件
if pkg.SchemaDir != "" {
    for _, src := range pkg.Schemas {
        b, err := os.ReadFile(src)
        // ... render with scaffoldtemplate.Render ...
        // ... write to internal/db/schema/ ...
    }
}

// layout 文件
if pkg.LayoutFile != "" {
    b, err := os.ReadFile(pkg.LayoutFile)
    // ... write to template/layout.yaml ...
}
```

## 7. 变量渲染

模版包里的 `schema/` 和 `kitex-template/` 文件用 `scaffoldtemplate.Render` 渲染：

```go
rendered, err := scaffoldtemplate.Render(string(b), scaffoldtemplate.RenderData{
    Module:      opts.Module,
    ServiceName: opts.Name,
})
```

**一致性：** 与 IDL 渲染、模板 YAML 渲染用同一个渲染器。

## 8. 向后兼容

| 场景 | 行为 |
|------|------|
| 模版包没有 `skip_default_templates` 字段 | 不跳过任何默认模板（现有行为） |
| 模版包没有 `schema/` 目录 | 不写额外 schema（现有行为） |
| 模版包没有 `layout.yaml` | 使用默认 layout（现有行为） |
| 模版包没有 `template.yaml` | 沿用现有逻辑（`HasMeta=false`） |

## 9. 测试策略

| 测试类型 | 覆盖 |
|----------|------|
| 单元测试 | `LoadPackage` 解析 `skip_default_templates`；schema/layout 渲染 |
| 集成测试 | `ncgo new --template rule-center` 完整生成树与 `--preset rule-center` 对比 |
| Golden 测试 | 沿用 mono golden 测试框架，新增 rule-center 模版包场景 |
| 向后兼容测试 | 无 `skip_default_templates` 的模版包行为不变 |

**验收标准（来自 Issue #56）：**

- [ ] `ncgo new rule-center --kind kitex --template rule-center`（no-generate）产物包含 `internal/db/schema/000002_rate_limit_rules.sql`
- [ ] 不生成重复脚手架：不出现默认 `handler/server/usecase/repository` 的规则服务目录
- [ ] `template/kitex-template/` 含 `ratelimit_shared_*.yaml` 全部 5 个共享片段
- [ ] `idl/rulecenter.proto` 渲染为完整 preset proto（RuleService + 5 RPC）
- [ ] 与 `--preset rule-center` 的生成树 diff 无实质差异（或文档明确剩余差异）
- [ ] mono golden 测试覆盖上述等价性
- [ ] 文档同步（README/examples EN+ZH）：`--template rule-center` 的 preset 等价用法

## 10. 文档更新

- **README EN/ZH**：`template.yaml` 新字段 `skip_default_templates` 说明
- **docs/examples EN/ZH**：模版包规范章节，增加 `schema/`、`layout.yaml`、`skip_default_templates` 说明

## 11. 范围边界（本次不做）

- `ncgo add rpc` / `ncgo add bff` 模版支持（Issue #57，后续迭代）
- `ncgo new` 自动 `ai sync`（Issue #58，后续迭代）
- 模版版本锁定 / tag / commit 级引用
- registry 仓库的创建、CI、审核自动化

## 12. Risks

| 风险 | 缓解 |
|------|------|
| `skip_default_templates` 字段拼写错误导致跳过失败 | 单元测试覆盖；消费端校验文件名合法性 |
| schema 渲染变量失败导致 SQL 语法错误 | 集成测试验证渲染后 SQL 可执行 |
| layout.yaml 覆盖默认 layout 后 hz 行为变化 | Golden 测试锁定生成树 |
| 向后兼容被破坏 | 所有新字段可选，缺省行为与现有完全一致 |

## 13. 实现计划概要

1. **扩展 `PackageMeta`**：增加 `SkipDefaultTemplates []string`
2. **扩展 `Package`**：增加 `SchemaDir`、`Schemas`、`LayoutFile` 字段
3. **修改 `LoadPackage`**：解析 `schema/` 目录和 `layout.yaml`
4. **扩展 `Options`**：增加 `SkipDefaultTemplates []string`
5. **修改 `writeKitexTemplate`**：检查 `opts.SkipDefaultTemplates` 跳过文件
6. **增强 `overlayTemplatePackage`**：复制 schema、layout，传递 skip 列表
7. **测试**：单元测试 + 集成测试 + golden 测试
8. **文档**：README + examples EN/ZH 同步

## 14. 关联工作

- Issue #57：`ncgo add rpc`/`add bff` 支持 `--template`/`--template-dir`（预留接口，本次不实现）
- Issue #58：`ncgo new` 生成后自动 `ai sync`（独立特性，本次不实现）

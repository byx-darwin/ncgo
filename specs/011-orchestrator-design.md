# orchestrator 共享服务层设计

- 状态：Draft
- 关联：`specs/010-v1.0-plan.md` Phase 1 / P0-1

## 1. 问题

`internal/cli/` 和 `internal/mcp/` 各自编排同一套 scaffold 调用：

```
cli/root.go:187-238   runNewMono()          ← 52 行编排
mcp/tool_new.go:101-131 callNew()           ← 31 行编排（同一逻辑）
cli/add.go:68-95       runAddInfra()        ← 28 行编排
mcp/tool_add.go:22-46  callAddInfra()       ← 25 行编排
cli/add_rulecenter.go:45-83 runAddRuleCenter() ← 39 行编排
mcp/tool_rulecenter.go:11-46 callAddRuleCenter() ← 36 行编排
```

加一个新 flag 或改错误处理要改两个地方，已出现不一致（CLI `runAddInfra` 返回 cobra error，MCP 版本返回 `textResult(err.Error(), true)`）。

## 2. 方案

新建 `internal/orchestrator/` 包，每个顶层操作一个文件：

```
internal/orchestrator/
  new.go        - NewService(ctx, opts) (*NewResult, error)
  add.go        - AddInfra / AddDomain / AddMethod / AddRPC / AddBFF / AddRuleCenter
  doctor.go     - RunDoctor(ctx, opts) (*DoctorResult, error)
  protolint.go  - RunProtolint(ctx, opts) (*ProtolintResult, error)
  ai.go         - InitClaude / SyncAI
  i18n.go       - I18nReport / I18nCheck
  export.go     - ExportTemplates
  upgrade.go    - UpgradeManifest
  extract.go    - ExtractDomain
```

### 2.1 接口设计

```go
// orchestrator/new.go
type NewOptions struct {
    Name           string
    Module         string
    Mode           string   // mono | micro
    Kind           string   // hertz | kitex
    Dir            string
    WithDatabase   bool
    Infra          []string
    Preset         string
    RuleCenterAddr string
    NoGenerate     bool
}

type NewResult struct {
    Dir         string
    Manifest    string
    NextSteps   []string
    WrittenPaths []string
}

func NewService(ctx context.Context, opts NewOptions) (*NewResult, error)
```

```go
// orchestrator/add.go
type AddInfraOptions struct {
    Root     string
    Kind     string   // redis | kafka | es | clickhouse | otel | logging | canary | registry_etcd
    Force    bool
    Wire     bool
    DryRun   bool
}

type AddInfraResult struct {
    DryRun      bool
    Updated     bool
    WrittenPaths []string
}

func AddInfra(ctx context.Context, opts AddInfraOptions) (*AddInfraResult, error)
```

### 2.2 CLI 变为薄包装

```go
// internal/cli/add.go (after)
func runAddInfra(cmd *cobra.Command, args []string) error {
    opts := orchestrator.AddInfraOptions{
        Root:   viper.GetString("root"),
        Kind:   args[0],
        Force:  viper.GetBool("force"),
        Wire:   viper.GetBool("wire"),
        DryRun: viper.GetBool("dry-run"),
    }
    result, err := orchestrator.AddInfra(cmd.Context(), opts)
    if err != nil {
        return err
    }
    // 只负责格式化输出
    switch viper.GetString("output") {
    case "json":
        enc := json.NewEncoder(cmd.OutOrStdout())
        return enc.Encode(result)
    default:
        printAddInfraText(cmd.OutOrStdout(), result)
    }
    return nil
}
```

### 2.3 MCP 变为薄包装

```go
// internal/mcp/tool_add.go (after)
func (s *Server) callAddInfra(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
    opts := orchestrator.AddInfraOptions{
        Root:   getString(args, "root"),
        Kind:   getString(args, "kind"),
        Force:  getBool(args, "force"),
        Wire:   getBool(args, "wire"),
        DryRun: getBool(args, "dryRun"),
    }
    result, err := orchestrator.AddInfra(ctx, opts)
    if err != nil {
        return textResult(err.Error(), true), nil
    }
    return s.formatAddInfraResult(result), nil
}
```

## 3. 迁移顺序

按风险从低到高：

1. `doctor` — 只读操作，无副作用，最适合验证模式
2. `protolint` — 只读，同上
3. `i18n report / check` — 只读
4. `ai init / sync` — 只读 + 写文件，逻辑简单
5. `export templates` — 只读
6. `upgrade --plan` — 只读模式先迁移
7. `add infra / domain / method / rule-center` — 写操作
8. `add rpc / bff` — 写操作，依赖 mono
9. `new` — 最复杂的写操作，最后迁移

## 4. 测试策略

- orchestrator 层用 mock `exec.Runner` + `manifest` + 临时目录做单元测试
- CLI 层只测 flag → opts 映射 + 输出格式（不重复测业务逻辑）
- MCP 层只测参数解析 + schema 生成 + 结果格式化
- 现有 CLI/MCP 集成测试保持通过，逐步迁移覆盖

## 5. 不做的

- 不改 scaffold 包本身（mono/micro/infra/domain 接口不变）
- 不改 manifest/exec/doctor/protolint 内部实现
- 不改 MCP 协议帧处理（server.go 的 readFrame/respond 不变）

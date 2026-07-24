# PR5 实施计划 — 生成项目适配 go-tools v0.1.0：observability + registry 适配 go-framework

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把生成项目的 observability 统一到 go-framework OTLP（kitex 基础保持、hertz 基础补接线、移除 LoongSuite `otel` add-on），并把 kitex registry 后端从 etcd（`kitex-contrib/registry-etcd`）切换到 Polaris（`kitex-contrib/polaris`，新 `registry_polaris` kind + 基础 wire 接线、移除 `registry_etcd`）。

**Architecture:** 以 ncgo 模板/资产改动为主（`internal/assets/_data/` + `internal/scaffold/infra/` + `internal/scaffold/shared/`），不改 go-tools。observability 采用运行时 OTLP（`go-framework/{hertz,kitex}/observability.NewProvider` + `ServerMiddleware/ServerSuite`）；registry 采用 `kitex-contrib/polaris` 的 `NewPolarisRegistry`/`NewPolarisResolver`，经 `ncgo:wire` 锚点幂等接入 kitex 基础 server/client。错误码沿用 PR1 的 goerror + `registry_config_invalid` 段。

**Tech Stack:** Go 1.25（ncgo 构建）/ 生成项目 go 1.26.5 · go-framework v0.1.0（`{hertz,kitex}/observability`、`config.JaegerOption`、`config.ObservabilityConfig`）· kitex-contrib/polaris（registry/discovery）· kitex v0.16.1 · golden 测试 + e2e 编译测试。

## Global Constraints

- **设计文档为准**：`specs/017-go-tools-v0.1.0-adaptation-pr5.md`（决策见 §3）。本计划修正设计文档 §4.B 一处：hertz 的 Jaeger 配置**不需新增** Config 字段——`hertzconfig.ServerConfig` 已内嵌 `Jaeger *config.JaegerOption`，故 hertz observability 读 `cfg.Server.Jaeger`、ServiceName 取 `cfg.Server.Registry.Name`。
- **kitex-contrib/polaris API（实测对 kitex v0.16.1 可编译）**：
  - `polaris.NewPolarisRegistry(so polaris.ServerOptions, configFile ...string) (polaris.Registry, error)`，`ServerOptions{ Metadata map[string]string }`，返回类型扩展 kitex `registry.Registry`。
  - `polaris.NewPolarisResolver(o polaris.ClientOptions, configFile ...string) (polaris.Resolver, error)`，`ClientOptions{ DstMetadata, SrcNamespace, SrcService, SrcMetadata }`，返回类型扩展 kitex `discovery.Resolver`。
  - `configFile` 指向 `polaris.yaml`（polaris-go 标准配置）；缺省（不传）时 SDK 读工作目录 `polaris.yaml`。
- **goerror 错误**：registry 配置校验用 `goerror.In("kitex.registry").Code("registry_config_invalid").Public("registry configuration is invalid")`（沿用 etcd add-on 既有段，见 `registry_etcd.go`）。
- **CLI 契约变更**：移除 `otel`/`observability_otel` 与 `registry_etcd` kind；新增 `registry_polaris`。`ncgo add infra otel`/`registry_etcd` 将返回 invalid kind。MCP `ncgo_add_infra` 的 kind enum 由 `enumField("kind", infra.SupportedKinds())` **自动同步**，无需手改枚举，但依赖具体 kind 的测试需更新。
- **golden 纪律**：模板/资产改动后用**精确包路径** `-update-golden` 重生成（不传 `./internal/scaffold/...` 全树），**逐 diff 审查**避免误 bless。
- **验证链**：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。
- **不做（YAGNI）**：不改 go-tools；不在 CI 运行真实 Polaris/注册运行期测试（仅编译级验证，见设计文档 §9）；不重写既有 logging/canary wire。

## File Structure

| 文件 | 动作 | 责任 |
|------|------|------|
| `internal/scaffold/infra/infra.go` | Modify | kind 常量/SupportedKinds/kitexOnlyKinds/goGetDeps/outputRelPaths/setupSteps/commonAssetKinds/normalizeKind/assetFiles（registry_polaris 双文件） |
| `internal/assets/_data/kitex/optional/registry_polaris.go` | Create | Polaris registry/resolver 包装（`package registry`） |
| `internal/assets/_data/kitex/optional/registry_polaris.yaml` | Create | `polaris.yaml` 模板（输出到项目根 `polaris.yaml`） |
| `internal/assets/_data/kitex/optional/registry_etcd.go` | Delete | 旧 etcd add-on |
| `internal/assets/_data/optional/observability_otel.go` | Delete | LoongSuite add-on |
| `internal/scaffold/infra/wire.go` | Modify | registry_polaris wire（server/client）+ 新 marker + wireSupportedKind + unsupportedWireError |
| `internal/assets/_data/kitex/kitex-template/server.yaml` | Modify | 加 `// ncgo:wire:registry:server` 锚点 |
| `internal/assets/_data/kitex/kitex-template/client.yaml` | Modify | 加 `// ncgo:wire:registry:client` 锚点 |
| `internal/assets/_data/hertz/hertz-template/server_go.yaml` | Modify | hertz observability 接线（NewProvider + ServerMiddleware） |
| `internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml` | Modify | hertz dev conf 增 `server.jaeger` 样例 |
| `internal/assets/_data/hertz/hertz-template/conf_go.yaml` | Modify | hertz conf `Default()` 增 Jaeger 默认（视现状） |
| `internal/scaffold/shared/container.go` | Modify | etcd→polaris compose 特性映射（移除 etcd 特性） |
| `internal/scaffold/infra/infra_test.go` | Modify | registry/otel/wire 测试改写为 polaris |
| `internal/scaffold/infra/testdata/**` | Regen | infra golden |
| `internal/scaffold/mono/testdata/**` | Regen | mono golden（hertz observability、kitex 锚点） |
| `internal/assets/assets_test.go` | Modify | 资产清单（去 otel/etcd、增 polaris） |
| `internal/mcp/server_test.go` | Modify | `kind:"otel"` 用例改为有效 kind |
| `internal/orchestrator/add_test.go` | Modify | `observability_otel` 期望改为有效 kind |
| `internal/scaffold/shared/container_test.go` | Modify | etcd→polaris compose 断言 |
| design-doc / README / examples（中英） | Modify | 文档对齐 |

---

## Task 1: infra.go kind 元数据（移除 otel、registry_etcd→registry_polaris）

**Files:**
- Modify: `internal/scaffold/infra/infra.go`
- Test: `internal/scaffold/infra/infra_test.go`

**Interfaces:**
- Produces: `KindRegistryPolaris = "registry_polaris"`；`SupportedKinds()` 含 `registry_polaris`、不含 `otel`/`observability_otel`/`registry_etcd`；`goGetDeps[KindRegistryPolaris] = {"github.com/kitex-contrib/polaris", "github.com/byx-darwin/go-tools/go-common"}`；`outputRelPaths[KindRegistryPolaris] = internal/base/registry/polaris.go`。

- [ ] **Step 1: 写失败测试（polaris kind 存在、otel/etcd 移除）**

在 `infra_test.go` 追加：

```go
func TestRegistryPolarisKindRegistered(t *testing.T) {
	found := false
	for _, k := range SupportedKinds() {
		if k == KindRegistryPolaris {
			found = true
		}
		if k == "registry_etcd" || k == "observability_otel" || k == "otel" {
			t.Errorf("SupportedKinds should not contain removed kind %q", k)
		}
	}
	if !found {
		t.Errorf("SupportedKinds missing registry_polaris: %v", SupportedKinds())
	}
	if got := goGetDeps[KindRegistryPolaris]; len(got) == 0 || got[0] != "github.com/kitex-contrib/polaris" {
		t.Errorf("goGetDeps[registry_polaris] = %v, want kitex-contrib/polaris first", got)
	}
}

func TestNormalizeKindRejectsRemovedKinds(t *testing.T) {
	for _, kind := range []string{"otel", "observability_otel", "registry_etcd"} {
		if _, err := normalizeKind(kind); err == nil {
			t.Errorf("normalizeKind(%q) should error (removed kind)", kind)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/scaffold/infra/ -run 'TestRegistryPolarisKindRegistered|TestNormalizeKindRejectsRemovedKinds' -count=1`
Expected: FAIL（`KindRegistryPolaris` 未定义，编译错误）

- [ ] **Step 3: 修改 infra.go kind 元数据**

`internal/scaffold/infra/infra.go` 常量块：删除 `KindRegistryEtcd`、`KindObservabilityOtel`、`KindOtelAlias`，新增 `KindRegistryPolaris = "registry_polaris"`：

```go
const (
	KindRedis            = "redis"
	KindKafka            = "kafka"
	KindES               = "es"
	KindClickHouse       = "clickhouse"
	KindRegistryPolaris  = "registry_polaris"
	KindObservabilityLog = "observability_logging"
	KindLoggingAlias     = "logging"
	KindReleaseCanary    = "release_canary"
	KindCanaryAlias      = "canary"
)
```

`SupportedKinds()`：

```go
func SupportedKinds() []string {
	return []string{KindRedis, KindKafka, KindES, KindClickHouse, KindObservabilityLog, KindLoggingAlias, KindReleaseCanary, KindCanaryAlias, KindRegistryPolaris}
}
```

`kitexOnlyKinds()`：

```go
func kitexOnlyKinds() []string {
	return []string{KindRegistryPolaris}
}
```

`goGetDeps`：删除 `KindRegistryEtcd` 与（若存在）otel 条目，新增：

```go
	KindRegistryPolaris: {"github.com/kitex-contrib/polaris", "github.com/byx-darwin/go-tools/go-common"},
```

`outputRelPaths`：删除 `KindRegistryEtcd`、`KindObservabilityOtel` 条目，新增：

```go
	KindRegistryPolaris: filepath.Join("internal", "base", "registry", "polaris.go"),
```

`setupSteps`：删除 `KindObservabilityOtel` 条目（LoongSuite install/otel go build/OTEL_* env）。

`commonAssetKinds`：删除 `KindObservabilityOtel: true`（保留 log/canary）。

`normalizeKind`：删除 `KindOtelAlias` 分支（`if kind == KindOtelAlias {...}`）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/scaffold/infra/ -run 'TestRegistryPolarisKindRegistered|TestNormalizeKindRejectsRemovedKinds' -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/scaffold/infra/infra.go internal/scaffold/infra/infra_test.go
git commit -m "feat(infra): kind 元数据移除 otel、registry_etcd→registry_polaris（PR5）"
```

---

## Task 2: registry_polaris add-on 资产（.go + polaris.yaml）+ assetFiles 双文件 + 删 registry_etcd.go

**Files:**
- Create: `internal/assets/_data/kitex/optional/registry_polaris.go`
- Create: `internal/assets/_data/kitex/optional/registry_polaris.yaml`
- Delete: `internal/assets/_data/kitex/optional/registry_etcd.go`
- Modify: `internal/scaffold/infra/infra.go`（assetFiles）
- Test: `internal/scaffold/infra/infra_test.go`

**Interfaces:**
- Consumes: Task 1 的 `KindRegistryPolaris`、`outputRelPaths`。
- Produces: `Add(Options{Kind: KindRegistryPolaris})` 写出 `internal/base/registry/polaris.go` 与项目根 `polaris.yaml`；`registry_polaris.go` 暴露 `NewRegistry(PolarisConfig)`/`NewResolver(PolarisConfig)`/`PolarisConfig.Validate()`。

- [ ] **Step 1: 创建 registry_polaris.go 资产**

创建 `internal/assets/_data/kitex/optional/registry_polaris.go`：

```go
// Optional Polaris registry/discovery add-on for Kitex RPC services.
//
// To enable: copy this file to internal/base/registry/polaris.go and the
// accompanying polaris.yaml to your project root, run
// `go get github.com/kitex-contrib/polaris`, then wire it in bootstrap:
//
//	reg, err := registry.NewRegistry(registry.PolarisConfig{
//	    ServiceName: cfg.Server.Registry.Name,
//	    ConfigFile:  "polaris.yaml",
//	})
//	if err != nil { log.Fatal(err) }
//	server.Run(kitexserver.WithRegistry(reg))
//
// Client-side discovery:
//
//	res, err := registry.NewResolver(registry.PolarisConfig{
//	    ServiceName: cfg.ServiceName,
//	    ConfigFile:  "polaris.yaml",
//	})
//	if err != nil { log.Fatal(err) }
//	cli, err := echoclient.New(ctx, cfg, kitexclient.WithResolver(res))
//
// Required dependencies:
//
//	go get github.com/kitex-contrib/polaris
//	go get github.com/byx-darwin/go-tools/go-common

package registry

import (
	"github.com/cloudwego/kitex/pkg/discovery"
	kitexregistry "github.com/cloudwego/kitex/pkg/registry"
	polaris "github.com/kitex-contrib/polaris"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// PolarisConfig configures Polaris-backed service registry/discovery.
type PolarisConfig struct {
	ServiceName string            `json:"service_name" yaml:"service_name"`
	ConfigFile  string            `json:"config_file" yaml:"config_file"`
	Metadata    map[string]string `json:"metadata" yaml:"metadata"`
}

// NewRegistry creates a Polaris-backed kitex server registry.
func NewRegistry(cfg PolarisConfig) (kitexregistry.Registry, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return polaris.NewPolarisRegistry(polaris.ServerOptions{Metadata: cfg.Metadata}, cfg.configFiles()...)
}

// NewResolver creates a Polaris-backed kitex client resolver.
func NewResolver(cfg PolarisConfig) (discovery.Resolver, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return polaris.NewPolarisResolver(polaris.ClientOptions{SrcService: cfg.ServiceName}, cfg.configFiles()...)
}

// Validate checks required fields.
func (c PolarisConfig) Validate() error {
	if c.ServiceName == "" {
		return goerror.In("kitex.registry").Code("registry_config_invalid").Public("registry configuration is invalid").New("service_name is empty")
	}
	return nil
}

func (c PolarisConfig) configFiles() []string {
	if c.ConfigFile == "" {
		return nil
	}
	return []string{c.ConfigFile}
}
```

- [ ] **Step 2: 创建 registry_polaris.yaml 资产（polaris.yaml 模板）**

创建 `internal/assets/_data/kitex/optional/registry_polaris.yaml`：

```yaml
# Polaris SDK configuration for service registry/discovery.
# Consumed by github.com/kitex-contrib/polaris (polaris-go).
# See https://github.com/polarismesh/polaris-go for the full option set.
global:
  serverConnector:
    # Polaris server address(es). For local dev, `docker compose --profile config-center-polaris up`.
    addresses:
      - 127.0.0.1:8091
```

- [ ] **Step 3: 删除 registry_etcd.go 资产**

Run: `git rm internal/assets/_data/kitex/optional/registry_etcd.go`

- [ ] **Step 4: assetFiles 支持 registry_polaris 双文件**

在 `internal/scaffold/infra/infra.go` 的 `assetFiles` 开头（`if infraKind == KindObservabilityLog || ...` 分支之前）插入：

```go
	if infraKind == KindRegistryPolaris {
		if serviceKind != manifest.KindKitex {
			return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", infraKind)
		}
		return []addOnFile{
			{SourcePath: "kitex/optional/registry_polaris.go", OutputRelPath: outputRelPaths[KindRegistryPolaris]},
			{SourcePath: "kitex/optional/registry_polaris.yaml", OutputRelPath: "polaris.yaml"},
		}, nil
	}
```

- [ ] **Step 5: 写失败测试（改写 TestAddKitexOnlyRegistryEtcd → TestAddKitexOnlyRegistryPolaris）**

在 `infra_test.go` 中**删除** `TestAddKitexOnlyRegistryEtcd`，替换为：

```go
func TestAddKitexOnlyRegistryPolaris(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindRegistryPolaris})
	if err != nil {
		t.Fatalf("Add registry_polaris: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "registry", "polaris.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read registry file: %v", err)
	}
	for _, want := range []string{"package registry", "func NewRegistry(", "func NewResolver(", "polaris.NewPolarisRegistry"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("registry_polaris missing %q", want)
		}
	}
	polarisYAML := filepath.Join(root, "polaris.yaml")
	if _, err := os.Stat(polarisYAML); err != nil {
		t.Errorf("polaris.yaml not written: %v", err)
	}
	joined := strings.Join(res.NextSteps, "\n")
	if !strings.Contains(joined, "go get github.com/kitex-contrib/polaris") {
		t.Errorf("next steps missing polaris dep:\n%s", joined)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindRegistryPolaris {
		t.Errorf("manifest.Infra = %v, want [registry_polaris]", m.Infra)
	}
}

func TestAddRegistryPolarisRejectedForHertz(t *testing.T) {
	root := seedProject(t, nil)
	if _, err := Add(Options{Root: root, Kind: KindRegistryPolaris}); err == nil {
		t.Fatalf("registry_polaris should be rejected for hertz services")
	}
}
```

- [ ] **Step 6: 运行确认通过**

Run: `go test ./internal/scaffold/infra/ -run 'TestAddKitexOnlyRegistryPolaris|TestAddRegistryPolarisRejectedForHertz' -count=1`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/assets/_data/kitex/optional/registry_polaris.go internal/assets/_data/kitex/optional/registry_polaris.yaml internal/scaffold/infra/infra.go internal/scaffold/infra/infra_test.go
git commit -m "feat(infra): registry_polaris add-on（kitex-contrib/polaris + polaris.yaml），移除 registry_etcd（PR5）"
```

---

## Task 3: 移除 observability_otel 资产 + 更新 assets_test

**Files:**
- Delete: `internal/assets/_data/optional/observability_otel.go`
- Modify: `internal/assets/assets_test.go`
- Modify: `internal/scaffold/infra/infra_test.go`（删除 otel 测试）

- [ ] **Step 1: 删除资产**

Run: `git rm internal/assets/_data/optional/observability_otel.go`

- [ ] **Step 2: 删除 infra_test.go 中 otel 测试**

删除函数：`TestAddObservabilityOtelForKitex`、`TestAddObservabilityOtelForHertz`、`TestAddOtelAliasRecordsCanonicalKind`。

- [ ] **Step 3: 更新 assets_test.go 资产清单**

在 `internal/assets/assets_test.go` 的期望资产列表中：删除 `"kitex/optional/registry_etcd.go"` 与 `"optional/observability_otel.go"`，新增 `"kitex/optional/registry_polaris.go"` 与 `"kitex/optional/registry_polaris.yaml"`。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/assets/ ./internal/scaffold/infra/ -count=1`
Expected: PASS（assets 清单与嵌入 FS 一致；infra otel 测试已删）

- [ ] **Step 5: 提交**

```bash
git add -A internal/assets internal/scaffold/infra/infra_test.go
git commit -m "feat(infra): 移除 LoongSuite observability_otel add-on 资产与测试（PR5）"
```

---

## Task 4: registry_polaris wire（接入 kitex 基础 server/client）

**Files:**
- Modify: `internal/scaffold/infra/wire.go`
- Modify: `internal/assets/_data/kitex/kitex-template/server.yaml`
- Modify: `internal/assets/_data/kitex/kitex-template/client.yaml`
- Test: `internal/scaffold/infra/infra_test.go`

**Interfaces:**
- Consumes: Task 2 的 `registry.NewRegistry`/`registry.NewResolver`。
- Produces: `Wire(root, module, "kitex", KindRegistryPolaris)` 在 server.go 插入 `kitexserver.WithRegistry(...)`、在 `pkg/client/*/client.go` 插入 `kitexclient.WithResolver(...)`。

- [ ] **Step 1: wire.go 增加 marker 常量与 wireSupportedKind**

在 `wire.go` marker 常量块新增：

```go
	markerRegistryServer = "// ncgo:wire:registry:server"
	markerRegistryClient = "// ncgo:wire:registry:client"
```

`wireSupportedKind`：

```go
func wireSupportedKind(kind string) bool {
	return kind == KindObservabilityLog || kind == KindReleaseCanary || kind == KindRegistryPolaris
}
```

`unsupportedWireError`：

```go
func unsupportedWireError() error {
	return fmt.Errorf("infra: --wire is only supported for %s/%s/%s", KindObservabilityLog, KindReleaseCanary, KindRegistryPolaris)
}
```

- [ ] **Step 2: wireKitex server 增加 registry case + block helper**

在 `wireKitex` 的 `switch kind` 中追加 case（`serverPath` 上下文）：

```go
	case KindRegistryPolaris:
		s, err = addGoImportWithPlan(s, module+"/internal/base/registry", serverPath, &serverPlan)
		if err != nil {
			return nil, err
		}
		s, err = insertOnceMarkerOrAnchorWithPlan(s, "kitexserver.WithRegistry(", markerRegistryServer, "\topts = append(opts, extraOptions...)\n", kitexRegistryServer(), serverPath, &serverPlan, "insert_registry_server", "registry.NewRegistry")
		if err != nil {
			return nil, err
		}
```

文件底部新增 helper：

```go
func kitexRegistryServer() string {
	return "\tif reg, regErr := registry.NewRegistry(registry.PolarisConfig{ServiceName: cfg.Server.Registry.Name, ConfigFile: \"polaris.yaml\"}); regErr != nil {\n" +
		"\t\tlog.Fatalf(\"polaris registry: %v\", regErr)\n" +
		"\t} else {\n" +
		"\t\topts = append(opts, kitexserver.WithRegistry(reg))\n" +
		"\t}\n"
}

func kitexRegistryClient() string {
	return "\tif res, resErr := registry.NewResolver(registry.PolarisConfig{ServiceName: cfg.ServiceName, ConfigFile: \"polaris.yaml\"}); resErr == nil {\n" +
		"\t\toptions = append(options, kitexclient.WithResolver(res))\n" +
		"\t}\n"
}
```

- [ ] **Step 3: wireKitexClient 增加 registry case**

在 `wireKitexClient` 的 `switch kind` 中追加：

```go
	case KindRegistryPolaris:
		s, err = addGoImportWithPlan(s, module+"/internal/base/registry", path, &plan)
		if err != nil {
			return nil, err
		}
		s, err = insertOnceMarkerOrAnchorWithPlan(s, "kitexclient.WithResolver(", markerRegistryClient, anchor, kitexRegistryClient(), path, &plan, "insert_registry_client", "registry.NewResolver")
		if err != nil {
			return nil, err
		}
```

（`anchor` 为函数内既有 `"\tif cfg.EnableMetaInfo {...}"` 兜底锚点；优先匹配 `markerRegistryClient`。）

- [ ] **Step 4: server.yaml 加锚点**

`internal/assets/_data/kitex/kitex-template/server.yaml` 中，在 `opts = append(opts, extraOptions...)` 行**之前**插入一行：

```
      // ncgo:wire:registry:server
```

- [ ] **Step 5: client.yaml 加锚点**

`internal/assets/_data/kitex/kitex-template/client.yaml` 中，在 `if cfg.EnableMetaInfo {` 块**之前**插入一行（位于 options 构造区）：

```
      // ncgo:wire:registry:client
```

- [ ] **Step 6: 写失败测试（wire 接入）**

在 `infra_test.go` 追加（仿 `TestAddLoggingWireForKitexServerAndClient` 的 seed + 预置 server.go/client.go 方式；若现有测试用 golden 种子项目，则照该模式）：

```go
func TestAddRegistryPolarisWireForKitexServerAndClient(t *testing.T) {
	root := seedKitexProject(t, nil)
	if _, err := Add(Options{Root: root, Kind: KindRegistryPolaris}); err != nil {
		t.Fatalf("Add registry_polaris: %v", err)
	}
	// 预置含锚点的 server.go / client.go（使用 testdata 种子或现场写入）。
	writeKitexServerWithRegistryAnchor(t, root)
	writeKitexClientWithRegistryAnchor(t, root)

	res, err := Add(Options{Root: root, Kind: KindRegistryPolaris, Wire: true})
	if err != nil {
		t.Fatalf("Add registry_polaris --wire: %v", err)
	}
	serverBody := readFile(t, filepath.Join(root, "internal", "base", "server", "server.go"))
	if !strings.Contains(serverBody, "kitexserver.WithRegistry(") || !strings.Contains(serverBody, "registry.NewRegistry(") {
		t.Errorf("server.go missing registry wiring:\n%s", serverBody)
	}
	clientFiles, _ := filepath.Glob(filepath.Join(root, "pkg", "client", "*", "client.go"))
	if len(clientFiles) == 0 {
		t.Fatalf("no client.go seeded")
	}
	clientBody := readFile(t, clientFiles[0])
	if !strings.Contains(clientBody, "kitexclient.WithResolver(") || !strings.Contains(clientBody, "registry.NewResolver(") {
		t.Errorf("client.go missing resolver wiring:\n%s", clientBody)
	}
	_ = res
}
```

> 说明：`writeKitexServerWithRegistryAnchor`/`writeKitexClientWithRegistryAnchor`/`readFile` 为本测试新增的小 helper，写入含 `// ncgo:wire:registry:server` / `// ncgo:wire:registry:client` 锚点与 `opts = append(opts, extraOptions...)` / `if cfg.EnableMetaInfo {...}` 兜底片段的最小 Go 源。实现时参照同文件既有 wire 测试的种子写法（如 `TestAddLoggingWireForKitexServerAndClient`），保持包导入块含 `import (`。

- [ ] **Step 7: 运行确认通过**

Run: `go test ./internal/scaffold/infra/ -run 'TestAddRegistryPolarisWireForKitexServerAndClient' -count=1`
Expected: PASS

- [ ] **Step 8: 更新 TestAddWireRejectsUnsupportedKind**

`TestAddWireRejectsUnsupportedKind`（约 line 1092）若以 `registry_etcd`/`otel` 为不支持样例，改用 `KindRedis`（不支持 wire）：

```go
	if _, err := Add(Options{Root: root, Kind: KindRedis, Wire: true}); err == nil {
		t.Fatalf("wire should be rejected for redis")
	}
```

Run: `go test ./internal/scaffold/infra/ -run 'TestAddWireRejectsUnsupportedKind' -count=1`
Expected: PASS

- [ ] **Step 9: 提交**

```bash
git add internal/scaffold/infra/wire.go internal/scaffold/infra/infra_test.go internal/assets/_data/kitex/kitex-template/server.yaml internal/assets/_data/kitex/kitex-template/client.yaml
git commit -m "feat(infra): registry_polaris 经 ncgo:wire 锚点接入 kitex server/client（PR5）"
```

---

## Task 5: hertz 基础 observability 接线（go-framework OTLP）

**Files:**
- Modify: `internal/assets/_data/hertz/hertz-template/server_go.yaml`
- Modify: `internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml`
- Modify: `internal/assets/_data/hertz/hertz-template/conf_go.yaml`（Default，视现状）

**Interfaces:**
- Consumes: `hertzconfig.ServerConfig.Jaeger`（已存在）、`go-framework/hertz/observability.NewProvider`/`ServerMiddleware`/`Shutdown`、`config.ObservabilityConfig`。
- Produces: hertz 生成 server.go 在 `cfg.Server.Jaeger.Enable` 时初始化 OTLP provider 并 `h.Use(provider.ServerMiddleware())`。

- [ ] **Step 1: server_go.yaml 增加 import**

在 `server_go.yaml` 的 import 块中，紧邻 `hertzframework "...go-framework/hertz"` 处新增两行（无条件导入）：

```
      gfconfig "github.com/byx-darwin/go-tools/go-framework/config"
      hertzobs "github.com/byx-darwin/go-tools/go-framework/hertz/observability"
```

- [ ] **Step 2: server_go.yaml 增加 observability 接线**

在 `h.Use(middleware.AccessLog())` 之后、logging 注释块之前插入：

```
      // OTel tracing — enabled when server.jaeger config is present
      if cfg.Server.Jaeger != nil && cfg.Server.Jaeger.Enable {
          provider, obsErr := hertzobs.NewProvider(ctx, gfconfig.ObservabilityConfig{
              Enabled:     true,
              Endpoint:    cfg.Server.Jaeger.Endpoint,
              ServiceName: cfg.Server.Registry.Name,
          })
          if obsErr != nil {
              log.Fatalf("init observability: %v", obsErr)
          }
          defer func() {
              if shutdownErr := provider.Shutdown(); shutdownErr != nil {
                  log.Printf("shutdown observability: %v", shutdownErr)
              }
          }()
          h.Use(provider.ServerMiddleware())
      }
```

（`ctx` 已在 `Run()` 中定义为 `context.Background()`。）

- [ ] **Step 3: conf_dev_yaml.yaml 增加 jaeger 样例**

在 hertz dev conf 的 `server:` 段内新增：

```yaml
    # 链路追踪（go-framework OTLP gRPC → Jaeger/collector）
    jaeger:
      # 默认关闭；置 true 并填写 collector 地址以启用
      enable: false
      endpoint: "127.0.0.1:4317"
```

- [ ] **Step 4: conf_go.yaml Default() 增 Jaeger 默认（视现状）**

检查 `conf_go.yaml` 的 `Default()`：若其中以字面量构造 `Server: hertzconfig.ServerConfig{...}`，在 `Registry` 默认旁补：

```go
			Jaeger: &config.JaegerOption{Enable: false, Endpoint: "127.0.0.1:4317"},
```

若 `Default()` 未显式构造 `Server.Jaeger`（保持 nil），则 server 端 `cfg.Server.Jaeger != nil` 判断已能兜底，**可跳过此步**并在提交说明中注明。

- [ ] **Step 5: 本地快速校验模板可渲染**

Run: `go test ./internal/scaffold/mono/ -run TestGenerateGoldenDefault -count=1`
Expected: FAIL（golden 不匹配，因 hertz server 变更）——这是预期的，golden 在 Task 8 统一重生成。此处仅确认渲染不报错（非渲染错误）。若报渲染/语法错误，先修模板。

- [ ] **Step 6: 提交**

```bash
git add internal/assets/_data/hertz/hertz-template/server_go.yaml internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml internal/assets/_data/hertz/hertz-template/conf_go.yaml
git commit -m "feat(scaffold): hertz 基础模板接入 go-framework OTLP observability（PR5）"
```

---

## Task 6: container.go compose 特性（etcd→polaris）

**Files:**
- Modify: `internal/scaffold/shared/container.go`
- Modify: `internal/scaffold/shared/container_test.go`

**Interfaces:**
- Consumes: Task 1 的 `registry_polaris` kind。
- Produces: `composeFeaturesForApp` 将 `registry_polaris` 映射到 `features.polaris`（复用既有 polaris-server-standalone compose 服务）；移除 etcd 特性。

- [ ] **Step 1: 替换 kind 常量与映射**

`container.go`：
- 常量 `infraRegistryEtcd = "registry_etcd"` → `infraRegistryPolaris = "registry_polaris"`。
- `composeFeaturesForApp` 中 `case infraRegistryEtcd: features.etcd = true` → `case infraRegistryPolaris: features.polaris = true`。

- [ ] **Step 2: 移除 etcd 特性死代码**

删除：`composeFeatures.etcd` 字段、`renderEtcdCompose` 调用与函数、`features.etcd` 相关 deps/volumes 追加、merge 中 `f.etcd = ...`。保留 polaris（既有，profile `config-center-polaris`，镜像 `polarismesh/polaris-server-standalone`，可同时服务 config-center 与 registry）。

- [ ] **Step 3: 更新 container_test.go**

将测试中 `registry_etcd`→`registry_polaris`，断言 compose 含 polaris 服务（而非 etcd）。运行：

Run: `go test ./internal/scaffold/shared/ -count=1`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/scaffold/shared/container.go internal/scaffold/shared/container_test.go
git commit -m "feat(shared): compose 特性 etcd→polaris（registry_polaris 复用 polaris-server，PR5）"
```

---

## Task 7: 修正依赖具体 kind 的测试（mcp / orchestrator）

**Files:**
- Modify: `internal/mcp/server_test.go`
- Modify: `internal/orchestrator/add_test.go`

- [ ] **Step 1: mcp/server_test.go 将 `otel` 用例改为有效 kind**

把 `server_test.go` 中 `ncgo_add_infra` 用 `"kind": "otel"` 的调用（约 line 429、973、1021、1048）改为 `"kind": "redis"`；把断言 `m.Infra[0] != "observability_otel"`（约 468-469）改为期望 `"redis"`；相应 `WrittenPath`/输出断言对齐 redis（`internal/base/data/redis.go`）。逐处运行：

Run: `go test ./internal/mcp/ -count=1`
Expected: PASS

- [ ] **Step 2: orchestrator/add_test.go 修正期望**

`add_test.go` 约 line 54：`want [observability_otel]` 改为测试实际使用的有效 kind（若该用例 Add 的是 otel，改为 `redis` 并同步断言 `[redis]`）。

Run: `go test ./internal/orchestrator/ -count=1`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/mcp/server_test.go internal/orchestrator/add_test.go
git commit -m "test: add_infra 用例 otel→redis（otel kind 已移除，PR5）"
```

---

## Task 8: golden 重生成（infra + mono）+ 逐 diff 审查

**Files:**
- Regen: `internal/scaffold/infra/testdata/**`
- Regen: `internal/scaffold/mono/testdata/**`

- [ ] **Step 1: 重生成 infra golden（精确包路径）**

Run: `go test ./internal/scaffold/infra/ -update-golden -count=1`
Expected: PASS（写出新快照）

- [ ] **Step 2: 重生成 mono golden（精确包路径）**

Run: `go test ./internal/scaffold/mono/ -update-golden -count=1`
Expected: PASS

- [ ] **Step 3: 逐 diff 审查（人工）**

Run: `git diff --stat internal/scaffold/infra/testdata internal/scaffold/mono/testdata` 后逐个 `git diff <file>` 审查：
- infra：registry_polaris 快照出现、registry_etcd/observability_otel 快照删除。
- mono：hertz server.go 出现 observability 块、kitex server.go/client.go 出现 registry 锚点行；无意外大面积 diff。
确认无误后再继续；若有误 bless 回退对应文件。

- [ ] **Step 4: 复跑 golden 测试确认稳定**

Run: `go test ./internal/scaffold/infra/ ./internal/scaffold/mono/ -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/scaffold/infra/testdata internal/scaffold/mono/testdata
git commit -m "test(scaffold): 重生成 infra/mono golden（registry_polaris + hertz observability + kitex 锚点，PR5）"
```

---

## Task 9: e2e 编译验证（生成项目可编译）

**Files:**
- 无新增（运行既有 e2e 编译测试）

- [ ] **Step 1: hertz e2e 编译**

Run: `go test ./internal/scaffold/mono/ -run TestGenerateHertzCompiles -count=1`
Expected: PASS（若环境缺 `hz`/`make` 则 SKIP，记录于提交说明）。验证 hertz observability 接线可编译。

- [ ] **Step 2: kitex e2e 编译（含 registry_polaris）**

定位 kitex 编译测试（`internal/scaffold/mono/` 或 `internal/scaffold/rpc/` 中 `TestGenerate*Compiles`）并运行；若存在「生成 kitex 项目 + `ncgo add infra registry_polaris` + `go build`」路径则覆盖之。
Expected: PASS 或 SKIP（缺 kitex 工具链/protoc 时）。

- [ ] **Step 3: 手动补强（可选，若 e2e 未覆盖 polaris 编译）**

在临时目录生成 kitex 项目、`ncgo add infra registry_polaris --wire`、`go mod tidy && go build ./...`，确认 `kitex-contrib/polaris` 对 kitex v0.16.1 可编译（设计文档 §2.3 已独立验证过）。记录结果。

---

## Task 10: 文档对齐（中英）

**Files:**
- Modify: `internal/assets/_data/docs/hertz/design-doc.zh-CN.md` / `.en.md`
- Modify: `internal/assets/_data/docs/kitex/design-doc.zh-CN.md` / `.en.md`
- Modify: `README.md` / `README.zh-CN.md`
- Modify: `docs/examples.md` / `docs/examples.zh-CN.md`

- [ ] **Step 1: design-doc（kitex）**

kitex design-doc 中：
- registry 小节（原「Etcd 注册/发现」）改为「Polaris 注册/发现」：`registry_polaris` kind、`registry/polaris.go` 暴露 `NewRegistry/NewResolver`、依赖 `kitex-contrib/polaris`、`polaris.yaml`、wire 锚点接线、错误码 `registry_config_invalid`。
- observability 小节：说明 kitex 基础已用 go-framework OTLP（`cfg.Jaeger`），LoongSuite add-on 已移除。

- [ ] **Step 2: design-doc（hertz）**

hertz design-doc 中：新增/更新 observability 小节——hertz 基础模板接入 go-framework OTLP（`cfg.Server.Jaeger` → `hertz/observability.NewProvider` → `h.Use(provider.ServerMiddleware())`）；移除 LoongSuite `observability_otel` 相关描述。

- [ ] **Step 3: README + examples（中英）**

`add infra` kind 列表与示例：移除 `otel`/`observability_otel`、`registry_etcd`；新增 `registry_polaris`（示例 `ncgo add infra registry_polaris --wire`）。中英对齐。

- [ ] **Step 4: markdown 诊断**

Run: 仓库 markdown lint（如 `pre-commit run --files <改动的 md>` 或既有 markdown 检查）
Expected: 无错误

- [ ] **Step 5: 提交**

```bash
git add internal/assets/_data/docs README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs: PR5 observability 统一 OTLP + registry 切换 Polaris（中英对齐）"
```

---

## Task 11: 最终验证链

- [ ] **Step 1: 全量构建与静态检查**

Run: `go build ./... && go build . && go vet ./...`
Expected: PASS

- [ ] **Step 2: 全量测试**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: 冒烟**

Run: `./scripts/smoke.sh`
Expected: PASS（`ncgo_add_infra` 枚举含 `registry_polaris`、不含 `otel`/`registry_etcd`）

- [ ] **Step 4: gofmt 检查**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`
Expected: 无输出

---

## Self-Review

- **Spec 覆盖**：设计文档 §4.A→Task 1/3；§4.B→Task 5；§4.C→Task 1/2；§4.D→Task 4；§4.E→Task 2（错误码）；§6 契约面→Task 1/3/7（CLI/MCP/资产）+ Task 8（golden）；§7 测试→Task 1/2/4/8/9；§8 文档→Task 10；§9 风险（golden 审查、e2e 编译、CLI 契约）→Task 8/9/7。container（compose）为 §4 隐含的生成布局面→Task 6。✅
- **类型一致**：`registry.NewRegistry/NewResolver(PolarisConfig)`、`polaris.NewPolarisRegistry/NewPolarisResolver`、`config.ObservabilityConfig{Enabled,Endpoint,ServiceName}`、`cfg.Server.Jaeger`/`cfg.Server.Registry.Name`、marker 常量名在 Task 间一致。✅
- **placeholder 扫描**：无 TBD/TODO；Task 5 Step 4 的「视现状可跳过」是条件分支说明（含明确判定），非占位；Task 9 Step 2 的 kitex 编译测试定位给出明确查找路径。✅

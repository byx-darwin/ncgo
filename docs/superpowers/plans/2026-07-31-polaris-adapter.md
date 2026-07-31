# Polaris Canary Adapter (opt-in) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an OPT-IN `ncgo add infra polaris_adapter` that wires the real `polaris-go` SDK into ncgo's existing SDK-neutral canary seams (`PolarisInstanceLister`/`PolarisRuleLoader` → `release.Discoverer`/`release.RuleProvider`), so a generated Kitex project can load real canary rules from Polaris and discover instances carrying `release.track` metadata — without ncgo or the default scaffold importing any Polaris SDK.

**Architecture:** A new embedded template asset (`kitex/optional/polaris_canary_adapter.go`, `package release`) is written ONLY when the user runs `ncgo add infra polaris_adapter`. It implements the existing callback seams using `polaris-go`, with all SDK calls isolated behind a small ncgo-defined `polarisAPI` interface (`sdkClient`), so mapping/rule-parse/credential/selector logic is pure and testable and only `sdkClient` couples to the SDK. A new infra kind (kitex-only, in `goGetDeps`) writes the asset, updates the manifest, and emits `go get` next-steps; the user owns the SDK version pin. The SDK-neutral core (`optional/release_canary.go`) is untouched and stays dependency-free.

**Tech Stack:** Go 1.26.5 · polaris-go (github.com/polarismesh/polaris-go, opt-in in generated projects) · gopkg.in/yaml.v3 · go-tools/go-common error (`goerror`) · ncgo `internal/scaffold/infra` machinery · golden tests · compile-only testmod for SDK verification.

**Spec:** `docs/superpowers/specs/2026-07-31-real-sdk-adapter-multirole-analysis.md` (§6) · **Issue:** #32 · **Workflow:** wf-2026-07-31-001

## Global Constraints

- Go 版本锁定 `go 1.26.5`；ncgo 本体**禁止** import `polaris-go`（SDK 依赖只进入用户生成项目）。
- SDK-neutral 核心 `internal/assets/_data/optional/release_canary.go` **逐字节不变**；其 golden 不允许重录。
- 默认 scaffold（mono/rpc/bff）golden **逐字节不变**：adapter asset 仅在 `add infra polaris_adapter` 时写出，不进默认模板树。
- 仅 **kitex** 服务支持本 kind（进 `kitexOnlyKinds()`）；hertz 调用必须返回明确错误（follow-up 单独 Issue）。
- 凭证仅从环境变量读取（`POLARIS_TOKEN` / `POLARIS_NAMESPACE` 等）；禁止硬编码；错误与日志禁止包含凭证值。
- 错误语义：**构造期**缺凭证/地址 → 立即报错（fail-fast，暴露误配）；**运行期** list/load 失败 → `Discover`/`Rules` 返回 error，canary LB 经既有 `fallback` fail-open 到 Kitex 默认加权 LB。
- CI 对 SDK 仅做**编译级**验证（无活 Polaris）：独立 testmod 钉住 polaris-go 版本 `go build`；seam 继续用 `Static*` 单测。
- 模板占位符用 `{{.Module}}`，由 `renderAssetBody` 替换为模块路径。
- commit 前缀遵循 conventional commits（`feat:`/`test:`/`docs:`）。
- 每个任务结束运行 `gofmt -l .` 为空、`go build ./...` 成功。

## Resolved Decisions & Assumptions (from Issue #32 review)

1. **错误路径（verifiable AC）**：`newSDKClient` 在 `len(cfg.Addresses)==0` 或（需要鉴权时）`POLARIS_TOKEN` 为空时返回 error，且 error 文本不含 token 值；运行期 `listInstances`/`loadRuleBytes` 失败时 `PolarisDiscoverer.Discover`/`PolarisRuleProvider.Rules` 透传 error（已有 seam 行为），路由层 fail-open。
2. **polaris-go 版本**：ncgo 不 pin；编译验证 testmod 钉一个可编译的 polaris-go v1 版本（实施时解析最新可编译稳定版并记录），asset 头注释标 `tested with polaris-go vX.Y.Z`；升级列为 follow-up。现有骨架经 `kitex-contrib/polaris` 间接用 `polaris-go v1.2.0-beta`（2022）——本 adapter 直连 polaris-go，版本独立选择。
3. **适用边界**：kitex-only MVP（进 `kitexOnlyKinds`）。
4. **Resolver 可见性（Top 风险，假设）**：MVP 保留 `KitexResultDiscoverer`（实例来自 Kitex resolver；当项目用 `registry_polaris` 的 `NewResolver` 时，Polaris 实例经 `kitexInstanceTags` 暴露 `release.track`）；adapter 提供真实 **RuleProvider** 接入 `KitexCanaryLoadBalancer.RuleProvider`。**假设**：kitex-contrib/polaris resolver 返回全量（stable+canary）实例、不做路由过滤。该假设无法在编译级 CI 验证 → 记入文档 troubleshooting，并列为 Phase-B 运行时 harness 验证项（本 MVP 不阻塞）。

---

## File Structure

| 文件 | 责任 |
|---|---|
| `internal/assets/_data/kitex/optional/polaris_canary_adapter.go` ★新 | 嵌入式模板：`package release`，实现 `PolarisInstanceLister`/`PolarisRuleLoader`（真 polaris-go），`polarisAPI` 接口 + `sdkClient`，映射/规则解析/凭证校验/`NewPolarisSelector` |
| `internal/scaffold/infra/infra.go` | 新 kind 常量+alias、`SupportedKinds`、`kitexOnlyKinds`、`goGetDeps`、`outputRelPaths`、`assetFiles` case、`setupSteps` next-steps |
| `internal/scaffold/infra/infra_test.go` | 新 kind 全路径测试（kitex 写文件/manifest/next-steps、hertz 拒绝、dry-run/plan） |
| `internal/scaffold/infra/golden_test.go` | `TestGenerateGoldenInfraPolarisAdapter`（新增 testdata，锁生成 adapter 字节） |
| `internal/scaffold/infra/testdata/infra-polaris-adapter/` ★新 | 新 golden 快照 |
| `internal/assets/_data/kitex/optional/polaris_adapter_compiletest/` ★新（或 `tools/verifyexamples/polaris-adapter/`） | 编译级验证 testmod：`go.mod`（钉 polaris-go）+ 引用 adapter 的 `main`/`doc.go` |
| `scripts/verify-polaris-adapter.sh` ★新 | 在 compiletest testmod 内 `go build`（CI 编译级闸门） |
| `internal/assets/assets_test.go`（或就近 asset sanity 测试）★新/改 | 断言 adapter asset 可被 `go/parser` 解析、gofmt-clean、含关键符号、用 env 凭证、无硬编码 token |
| `internal/mcp/server_test.go`（或 tools 测试） | 断言 `ncgo_add_infra` 的 `kind` enum 含 `polaris_adapter`（enum 由 `SupportedKinds()` 派生） |
| `internal/assets/_data/docs/kitex/*` 或 `docs/examples.md` + `.zh-CN.md` | 用法 + troubleshooting（EN+ZH） |
| `README.md` / `README.zh-CN.md` | 命令清单增 `ncgo add infra polaris_adapter` |

---

## Task 1: 嵌入式 Polaris adapter asset + sanity 测试

**Files:**
- Create: `internal/assets/_data/kitex/optional/polaris_canary_adapter.go`
- Create/Test: `internal/assets/polaris_adapter_asset_test.go`

**Interfaces:**
- Produces（生成到用户项目 `internal/base/release/polaris_adapter.go`，`package release`）：
  - `func NewPolarisInstanceLister(cfg PolarisDiscoveryConfig) (PolarisInstanceLister, error)` —— 构造期校验（fail-fast）；返回的 lister 用 polaris-go 拉实例并映射为 `[]PolarisInstance`（含 `release.track` metadata）。
  - `func NewPolarisRuleLoader(cfg PolarisRuleConfig) (PolarisRuleLoader, error)` —— 构造期校验；返回的 loader 从 Polaris 配置拉 YAML 并 `yaml.Unmarshal` 为 `RuleSet`。
  - `func NewPolarisSelector(discoveryCfg PolarisDiscoveryConfig, ruleCfg PolarisRuleConfig) (Selector, error)` —— 组装 `PolarisDiscoverer{ListInstances}` + `PolarisRuleProvider{LoadRules}`。
  - 内部 `type polarisAPI interface { listInstances(ctx, cfg) ([]PolarisInstance, error); loadRuleBytes(ctx, cfg) ([]byte, error) }` + `type sdkClient struct{...}`（唯一 SDK 耦合点）。
- Consumes: 既有 `PolarisDiscoveryConfig`/`PolarisRuleConfig`/`PolarisInstance`/`PolarisInstanceLister`/`PolarisRuleLoader`/`PolarisDiscoverer`/`PolarisRuleProvider`/`Selector`/`RuleSet`/`MetadataReleaseTrack`/`ProviderPolaris`（`optional/release_canary.go`）。

- [ ] **Step 1: 写 asset sanity 失败测试**

`internal/assets/polaris_adapter_asset_test.go`:

```go
package assets

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

func TestPolarisAdapterAssetSanity(t *testing.T) {
	b, err := fs.ReadFile(FS(), "kitex/optional/polaris_canary_adapter.go")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	src := string(b)

	// Must be valid Go (parses; imports need not resolve for parser).
	if _, err := parser.ParseFile(token.NewFileSet(), "polaris_canary_adapter.go", src, parser.ImportsOnly); err != nil {
		t.Fatalf("asset does not parse: %v", err)
	}
	for _, want := range []string{
		"package release",
		"func NewPolarisInstanceLister(",
		"func NewPolarisRuleLoader(",
		"func NewPolarisSelector(",
		"polarisAPI",
		"os.Getenv", // credentials via env only
	} {
		if !strings.Contains(src, want) {
			t.Errorf("asset missing %q", want)
		}
	}
	// No hardcoded credentials patterns.
	for _, bad := range []string{`"POLARIS_TOKEN_SECRET"`, "password = ", "token = \""} {
		if strings.Contains(src, bad) {
			t.Errorf("asset appears to hardcode credentials: %q", bad)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/assets/ -run TestPolarisAdapterAssetSanity -v`
Expected: FAIL — read asset: file does not exist

- [ ] **Step 3: 创建 adapter asset**

Create `internal/assets/_data/kitex/optional/polaris_canary_adapter.go`（`{{.Module}}` 占位；注意此文件与 canary.go 同属生成项目的 `package release`，故直接引用 seam 类型，不 import release）:

```go
// Optional Polaris canary adapter for Kitex services (OPT-IN).
//
// Generated by `ncgo add infra polaris_adapter`. It wires the real polaris-go
// SDK into the SDK-neutral canary seams in canary.go (same package). ncgo never
// imports polaris-go; this file is the only SDK-coupled surface.
//
// Enable:
//
//	go get github.com/polarismesh/polaris-go
//	go get gopkg.in/yaml.v3
//
// Credentials are read from the environment only — never hardcode them:
//
//	POLARIS_TOKEN      Polaris auth token (empty = no auth)
//	POLARIS_NAMESPACE  default namespace when cfg.Namespace is empty
//
// tested with polaris-go vX.Y.Z (see scripts/verify-polaris-adapter.sh)

package release

import (
	"context"
	"os"
	"strings"

	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
	"gopkg.in/yaml.v3"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// polarisAPI isolates every polaris-go call so mapping/parse/credential logic
// stays pure and unit-testable; only sdkClient couples to the SDK.
type polarisAPI interface {
	listInstances(ctx context.Context, cfg PolarisDiscoveryConfig) ([]PolarisInstance, error)
	loadRuleBytes(ctx context.Context, cfg PolarisRuleConfig) ([]byte, error)
}

// NewPolarisInstanceLister returns a PolarisInstanceLister backed by polaris-go.
// Construction fails fast when addresses are missing so misconfiguration is loud.
func NewPolarisInstanceLister(cfg PolarisDiscoveryConfig) (PolarisInstanceLister, error) {
	client, err := newSDKClient(cfg.Addresses)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, c PolarisDiscoveryConfig) ([]PolarisInstance, error) {
		return client.listInstances(ctx, c)
	}, nil
}

// NewPolarisRuleLoader returns a PolarisRuleLoader that reads a canary RuleSet
// (YAML) from Polaris config.
func NewPolarisRuleLoader(cfg PolarisRuleConfig) (PolarisRuleLoader, error) {
	client, err := newSDKClient(cfg.Addresses)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, c PolarisRuleConfig) (RuleSet, error) {
		raw, err := client.loadRuleBytes(ctx, c)
		if err != nil {
			return RuleSet{}, err
		}
		var rs RuleSet
		if err := yaml.Unmarshal(raw, &rs); err != nil {
			return RuleSet{}, goerror.In("release.polaris").Code("rule_parse_failed").Public("canary rules are invalid").Wrap(err)
		}
		return rs, nil
	}, nil
}

// NewPolarisSelector assembles a release.Selector wired to real Polaris
// discovery + rule loading.
func NewPolarisSelector(discoveryCfg PolarisDiscoveryConfig, ruleCfg PolarisRuleConfig) (Selector, error) {
	lister, err := NewPolarisInstanceLister(discoveryCfg)
	if err != nil {
		return Selector{}, err
	}
	loader, err := NewPolarisRuleLoader(ruleCfg)
	if err != nil {
		return Selector{}, err
	}
	return Selector{
		ServiceName:  firstNonEmpty(discoveryCfg.Service, ruleCfg.FileName),
		Discoverer:   PolarisDiscoverer{Config: discoveryCfg, ListInstances: lister},
		RuleProvider: PolarisRuleProvider{Config: ruleCfg, LoadRules: loader},
	}, nil
}

// sdkClient is the ONLY polaris-go-coupled implementation of polarisAPI.
type sdkClient struct {
	api polaris.SDKContext
}

func newSDKClient(addresses []string) (*sdkClient, error) {
	if len(addresses) == 0 {
		return nil, goerror.In("release.polaris").Code("config_invalid").Public("polaris configuration is invalid").New("addresses is empty")
	}
	cfg := polaris.NewDefaultConfiguration()
	cfg.GetGlobal().GetServerConnector().SetAddresses(addresses)
	if token := os.Getenv("POLARIS_TOKEN"); token != "" {
		// polaris-go reads auth from config; token value is never logged.
		cfg.GetGlobal().GetServerConnector().SetAuthToken(token)
	}
	api, err := polaris.NewSDKContextByConfig(cfg)
	if err != nil {
		return nil, goerror.In("release.polaris").Code("sdk_init_failed").Public("polaris client init failed").Wrap(err)
	}
	return &sdkClient{api: api}, nil
}

func (c *sdkClient) listInstances(ctx context.Context, cfg PolarisDiscoveryConfig) ([]PolarisInstance, error) {
	ns := firstNonEmpty(cfg.Namespace, os.Getenv("POLARIS_NAMESPACE"), "default")
	req := &polaris.GetAllInstancesRequest{}
	req.Namespace = ns
	req.Service = cfg.Service
	resp, err := c.api.GetConsumer().GetAllInstances(ctx, req)
	if err != nil {
		return nil, goerror.In("release.polaris").Code("list_instances_failed").Public("polaris discovery failed").Wrap(err)
	}
	out := make([]PolarisInstance, 0, len(resp.GetInstances()))
	for _, ins := range resp.GetInstances() {
		out = append(out, instanceFromPolaris(ins, ns, cfg.Service))
	}
	return out, nil
}

func (c *sdkClient) loadRuleBytes(ctx context.Context, cfg PolarisRuleConfig) ([]byte, error) {
	ns := firstNonEmpty(cfg.Namespace, os.Getenv("POLARIS_NAMESPACE"), "default")
	req := &polaris.GetConfigFileRequest{}
	req.Namespace = ns
	req.FileGroup = cfg.Group
	req.FileName = cfg.FileName
	file, err := c.api.GetConfigAPI().GetConfigFile(ctx, req)
	if err != nil {
		return nil, goerror.In("release.polaris").Code("load_rules_failed").Public("polaris rule load failed").Wrap(err)
	}
	return []byte(file.GetContent()), nil
}

// instanceFromPolaris maps a polaris-go instance to the SDK-neutral model,
// preserving release.track metadata so canary pools resolve correctly.
func instanceFromPolaris(ins model.Instance, namespace, service string) PolarisInstance {
	meta := map[string]string{}
	for k, v := range ins.GetMetadata() {
		meta[k] = v
	}
	return PolarisInstance{
		ID:        ins.GetInstanceKey(),
		Namespace: namespace,
		Service:   service,
		Host:      ins.GetHost(),
		Port:      int(ins.GetPort()),
		Weight:    int(ins.GetWeight()),
		Healthy:   ins.IsHealthy(),
		Isolate:   ins.IsIsolated(),
		Metadata:  meta,
	}
}
```

> ⚠️ **SDK API  reconciliation gate**：`sdkClient` 中的 polaris-go 调用（`NewDefaultConfiguration`/`GetServerConnector().SetAddresses/SetAuthToken`/`GetConsumer().GetAllInstances`/`GetConfigAPI().GetConfigFile`/`model.Instance` getters）按 polaris-go v1 文档编写。**Task 5 的编译验证**为唯一裁决：若钉住的 polaris-go 版本 API 不同，**只调整 `sdkClient` 与 `instanceFromPolaris`**，接口 `polarisAPI`、`New*` 构造函数与所有测试不受影响。`goerror` 的 `Wrap` 若该版本不存在则用 `.New(err.Error())` 替代（保持 Public 文案不变、不泄露 token）。

- [ ] **Step 4: 运行 sanity 测试通过**

Run: `go test ./internal/assets/ -run TestPolarisAdapterAssetSanity -v && go build ./...`
Expected: PASS（asset 是嵌入数据，不参与 ncgo 编译；`go build` 验证 assets 包仍 embed 成功）

- [ ] **Step 5: Commit**

```bash
git add internal/assets/_data/kitex/optional/polaris_canary_adapter.go internal/assets/polaris_adapter_asset_test.go
git commit -m "feat(assets): opt-in polaris canary adapter template (real polaris-go behind seam)"
```

---

## Task 2: infra kind `polaris_adapter`（kitex-only）

**Files:**
- Modify: `internal/scaffold/infra/infra.go`（常量区 :32-44、`SupportedKinds` :48、`kitexOnlyKinds` :56、`goGetDeps` :63、`outputRelPaths` :88、`assetFiles` :279、`setupSteps` :74）
- Test: `internal/scaffold/infra/infra_test.go`

**Interfaces:**
- Produces: `KindPolarisAdapter = "polaris_adapter"`（+ alias `"polaris-adapter"`）；`SupportedKinds()` 含之；`kitexOnlyKinds()` 含之；`goGetDeps[KindPolarisAdapter] = {"github.com/polarismesh/polaris-go", "gopkg.in/yaml.v3", "github.com/byx-darwin/go-tools/go-common"}`；`outputRelPaths[KindPolarisAdapter] = internal/base/release/polaris_adapter.go`；`assetFiles` kitex 分支返回 `kitex/optional/polaris_canary_adapter.go`；`setupSteps` next-steps。
- Consumes: 既有 `addOnFile`/`renderAssetBody`/manifest kitex 校验。

- [ ] **Step 1: 写失败测试**

Append to `internal/scaffold/infra/infra_test.go`（fixture 仿 `TestAddKitexOnlyRegistryPolaris` :981 —— 复用其 kitex manifest + TempDir 搭建写法；先 `sed -n '981,1043p' internal/scaffold/infra/infra_test.go` 抄 fixture helper）:

```go
func TestAddInfraPolarisAdapterKitex(t *testing.T) {
	root := newKitexProjectFixture(t) // 复用既有 kitex fixture helper（与 registry_polaris 用例同款）
	res, err := Add(Options{Root: root, Kind: "polaris-adapter"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertFileContains(t, root, "internal/base/release/polaris_adapter.go", "func NewPolarisSelector(")
	assertFileContains(t, root, "internal/base/release/polaris_adapter.go", "package release")
	if !res.Updated {
		t.Errorf("manifest not updated")
	}
	joined := strings.Join(res.NextSteps, "\n")
	if !strings.Contains(joined, "go get github.com/polarismesh/polaris-go") {
		t.Errorf("next-steps missing polaris-go go get: %v", res.NextSteps)
	}
}

func TestAddInfraPolarisAdapterRejectsHertz(t *testing.T) {
	root := newHertzProjectFixture(t)
	_, err := Add(Options{Root: root, Kind: "polaris_adapter"})
	if err == nil || !strings.Contains(err.Error(), "kitex") {
		t.Fatalf("want kitex-only error, got %v", err)
	}
}

func TestAddInfraPolarisAdapterPlan(t *testing.T) {
	root := newKitexProjectFixture(t)
	res, err := Add(Options{Root: root, Kind: "polaris_adapter", DryRun: true})
	if err != nil {
		t.Fatalf("Add dry-run: %v", err)
	}
	if !res.DryRun {
		t.Errorf("expected DryRun=true")
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "release", "polaris_adapter.go")); err == nil {
		t.Errorf("dry-run must not write adapter file")
	}
	if len(res.Plan) == 0 {
		t.Errorf("expected non-empty plan items")
	}
}
```

> 若 `newKitexProjectFixture`/`newHertzProjectFixture` 名称与现状不符，按 `grep -n "func new.*Fixture\|func TestAddKitexOnlyRegistryPolaris" internal/scaffold/infra/infra_test.go` 的实际 helper 名调整（仅 fixture 名允许按实况对齐）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/scaffold/infra/ -run 'TestAddInfraPolarisAdapter' -v`
Expected: FAIL — `infra: kind "polaris-adapter" is invalid`

- [ ] **Step 3: 实现 kind**

`infra.go` 常量区追加：

```go
	KindPolarisAdapter      = "polaris_adapter"
	KindPolarisAdapterAlias = "polaris-adapter"
```

`SupportedKinds()` 末尾追加 `KindPolarisAdapter, KindPolarisAdapterAlias`。

`kitexOnlyKinds()` 追加 `KindPolarisAdapter`。

`goGetDeps` 追加：

```go
	KindPolarisAdapter: {"github.com/polarismesh/polaris-go", "gopkg.in/yaml.v3", "github.com/byx-darwin/go-tools/go-common"},
```

`outputRelPaths` 追加：

```go
	KindPolarisAdapter: filepath.Join("internal", "base", "release", "polaris_adapter.go"),
```

`setupSteps` 追加：

```go
	KindPolarisAdapter: {
		"go get github.com/polarismesh/polaris-go",
		"set POLARIS_TOKEN / POLARIS_NAMESPACE env vars (never hardcode credentials)",
		"wire release.NewPolarisSelector(...) into KitexCanaryLoadBalancer.RuleProvider",
		"verify kitex resolver returns full stable+canary instance set (see troubleshooting)",
		"go mod tidy",
	},
```

`normalizeKind` 追加 alias 分支（在既有 alias 判断后）：

```go
	if kind == KindPolarisAdapterAlias {
		return KindPolarisAdapter, nil
	}
```

`assetFiles` 在 `KindRegistryPolaris` 分支后追加：

```go
	if infraKind == KindPolarisAdapter {
		if serviceKind != manifest.KindKitex {
			return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", infraKind)
		}
		return []addOnFile{
			{SourcePath: "kitex/optional/polaris_canary_adapter.go", OutputRelPath: outputRelPaths[KindPolarisAdapter]},
		}, nil
	}
```

> `setupSteps` 是否被 `nextSteps()` 消费需核对：`sed -n '572,616p' internal/scaffold/infra/infra.go`。若 `nextSteps` 仅用 `goGetDeps` 生成 `go get` 行，则 `setupSteps` 的额外条目需确认有读取点（仿 `KindRateLimit` 的 setupSteps 用法）；无读取点则把额外指引并入 `goGetDeps` 之上的 next-steps 逻辑或保留 `goGetDeps` 即可（测试仅断言 `go get github.com/polarismesh/polaris-go` 出现）。

- [ ] **Step 4: 运行测试至通过**

Run: `go test ./internal/scaffold/infra/ -run 'TestAddInfraPolarisAdapter' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/infra/infra.go internal/scaffold/infra/infra_test.go
git commit -m "feat(infra): add infra polaris_adapter kind (kitex-only, opt-in polaris-go)"
```

---

## Task 3: golden 锁定生成的 adapter

**Files:**
- Modify: `internal/scaffold/infra/golden_test.go`
- Create: `internal/scaffold/infra/testdata/infra-polaris-adapter/`（由 `-update-golden` 生成）

- [ ] **Step 1: 加 golden 测试**

Append to `golden_test.go`（仿 `TestGenerateGoldenInfraCanary` :54）:

```go
func TestGenerateGoldenInfraPolarisAdapter(t *testing.T) {
	root := newKitexProjectFixture(t)
	if _, err := Add(Options{Root: root, Kind: "polaris_adapter"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	p := filepath.Join(root, "internal", "base", "release", "polaris_adapter.go")
	golden.File(t, filepath.Join("infra-polaris-adapter", filepath.Base(p)), goldenReadFile(t, p))
}
```

- [ ] **Step 2: 生成 golden 并人工审查 diff**

Run: `go test ./internal/scaffold/infra/ -run TestGenerateGoldenInfraPolarisAdapter -update-golden`
Run: `git status internal/scaffold/infra/testdata/ && git diff --stat`
Expected: 仅新增 `testdata/infra-polaris-adapter/polaris_adapter.go`，**无** mono/rpc/canary 既有 golden 变化（若有 → 停止排查，默认 scaffold 不应受影响）。

- [ ] **Step 3: 复跑确认稳定**

Run: `go test ./internal/scaffold/infra/ -run TestGenerateGoldenInfraPolarisAdapter -v`
Expected: PASS（不带 -update-golden）

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/infra/golden_test.go internal/scaffold/infra/testdata/infra-polaris-adapter/
git commit -m "test(infra): golden for polaris_adapter generated output"
```

---

## Task 4: MCP / CLI 契约同步

**Files:**
- Test: `internal/mcp/server_test.go`（或现有 tools schema 测试）
- 检查（一般无需改码）: `internal/cli/add_infra.go`

**Interfaces:**
- `ncgo_add_infra` 的 `kind` enum 由 `enumField("kind", infra.SupportedKinds())` 派生（`internal/mcp/tools.go:30`）→ 新增 kind 自动进入 enum；本任务加测试锁定该契约。

- [ ] **Step 1: 确认 CLI 无硬编码 kind 列表**

Run: `grep -rn "polaris_adapter\|SupportedKinds\|registry_polaris" internal/cli/ | grep -v _test | head`
Expected: CLI 通过字符串参数传给 `infra.Add`，无硬编码枚举；若有硬编码帮助文本列出 kind，追加 `polaris_adapter`。

- [ ] **Step 2: 写 MCP enum 测试**

在既有 MCP tools/schema 测试中追加（按现有断言风格；先 `grep -n "ncgo_add_infra\|enum\|SupportedKinds" internal/mcp/*_test.go`）:

```go
func TestAddInfraToolKindEnumIncludesPolarisAdapter(t *testing.T) {
	tools := toolDefinitions() // 按现有导出名调整
	for _, tl := range tools {
		if tl.Name != "ncgo_add_infra" {
			continue
		}
		raw, _ := json.Marshal(tl.InputSchema)
		if !strings.Contains(string(raw), "polaris_adapter") {
			t.Fatalf("ncgo_add_infra kind enum missing polaris_adapter: %s", raw)
		}
		return
	}
	t.Fatal("ncgo_add_infra tool not found")
}
```

- [ ] **Step 3: 运行**

Run: `go test ./internal/mcp/ -run TestAddInfraToolKindEnumIncludesPolarisAdapter -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/ internal/cli/
git commit -m "test(mcp): assert add_infra kind enum includes polaris_adapter"
```

---

## Task 5: 编译级 SDK 验证闸门（无活 Polaris）

**Files:**
- Create: `tools/verifyexamples/polaris-adapter/go.mod`、`tools/verifyexamples/polaris-adapter/main.go`
- Create: `scripts/verify-polaris-adapter.sh`

**Interfaces:**
- 该 testmod 复制/引用 adapter asset，`go build` 验证其可编译 against 钉住的 polaris-go。这是 §Resolved Decisions #2/#4 的编译级裁决点。

- [ ] **Step 1: 建 testmod**

`tools/verifyexamples/polaris-adapter/go.mod`（版本实施时解析为可编译的 polaris-go 稳定版；先试最新 v1，失败再降）:

```
module verifyexample/polaris-adapter

go 1.26.5

require (
	github.com/polarismesh/polaris-go vX.Y.Z
	github.com/byx-darwin/go-tools/go-common vA.B.C
	gopkg.in/yaml.v3 v3.0.1
)
```

`main.go`：把 `internal/assets/_data/kitex/optional/polaris_canary_adapter.go` 的内容连同最小 `release` seam 类型（`PolarisDiscoveryConfig`/`PolarisRuleConfig`/`PolarisInstance`/`PolarisInstanceLister`/`PolarisRuleLoader`/`PolarisDiscoverer`/`PolarisRuleProvider`/`Selector`/`RuleSet`/`firstNonEmpty`）拷入同包，`package main` 加 `func main(){}` 引用 `NewPolarisSelector` 以触发编译。

> 生成方式（保证与 asset 一致）：脚本从 asset 拷贝并用 `sed 's/{{.Module}}/example/g'`，再拼接从 `optional/release_canary.go` 抽取的最小 seam 子集。若抽取复杂，可改为 `go build` 一个临时模块，其 `release` 包直接 `cp` 两个 asset 文件（canary.go 去掉 hertz/kitex 专有 import 后）——以「能编译」为准，记录最终做法。

- [ ] **Step 2: 写验证脚本**

`scripts/verify-polaris-adapter.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../tools/verifyexamples/polaris-adapter"
# refresh adapter copy from embedded asset (single source of truth)
cp ../../../internal/assets/_data/kitex/optional/polaris_canary_adapter.go ./polaris_canary_adapter.go
sed -i.bak 's#{{.Module}}#example#g' polaris_canary_adapter.go && rm -f polaris_canary_adapter.go.bak
go mod tidy
go build ./...
echo "polaris-adapter compile OK"
```

- [ ] **Step 3: 运行并裁决 SDK API**

Run: `chmod +x scripts/verify-polaris-adapter.sh && ./scripts/verify-polaris-adapter.sh`
Expected: `polaris-adapter compile OK`。若编译失败于 polaris-go API：**仅**调整 asset 的 `sdkClient`/`instanceFromPolaris`（Task 1），复跑 Task 1 sanity 测试 + 本脚本，直到通过；记录最终 pin 版本到 asset 头注释 `tested with polaris-go vX.Y.Z`。

- [ ] **Step 4: 接入 CI（文档/工作流）**

在 `.github/workflows/ci.yml` 增加一步 `run: ./scripts/verify-polaris-adapter.sh`（若 CI 允许网络拉模块；否则记录为本地/定期验证）。Run: `grep -n "smoke.sh\|go test" .github/workflows/ci.yml` 找到插入点。

- [ ] **Step 5: Commit**

```bash
git add tools/verifyexamples/polaris-adapter/ scripts/verify-polaris-adapter.sh .github/workflows/ci.yml
git commit -m "test(infra): compile-only verification gate for polaris adapter SDK usage"
```

---

## Task 6: 文档（EN+ZH）+ troubleshooting

**Files:**
- Modify: `docs/examples.md` + `docs/examples.zh-CN.md`（或 `internal/assets/_data/docs/kitex/*`）
- Modify: `README.md` + `README.zh-CN.md`

- [ ] **Step 1: 用法 + troubleshooting**

新增 `ncgo add infra polaris_adapter` 章节，覆盖：opt-in 语义与 `go get`；env 凭证（`POLARIS_TOKEN`/`POLARIS_NAMESPACE`，禁止硬编码）；如何把 `release.NewPolarisSelector(...)` 接入 `KitexCanaryLoadBalancer.RuleProvider`；troubleshooting：① `addresses is empty`/缺 token → 构造期报错含义；② 发现/规则加载失败 → 路由 fail-open 到默认加权（观察指标）；③ **Kitex resolver 可见性假设**：若 canary pool 为空，确认 resolver 返回全量实例（stable+canary），否则需 LB 层直调 `Discoverer`（Phase-B）；④ `release.track` metadata 未生效 → 检查注册端 metadata。EN/ZH 对齐。

- [ ] **Step 2: README 命令清单**

`ncgo add infra polaris_adapter` 一行 + 链接到上述章节。EN/ZH 对齐。

- [ ] **Step 3: markdown 诊断 + Commit**

Run: `gofmt -l . ; go build ./...`（docs 改动不影响构建；按仓库 markdown 诊断约定执行）

```bash
git add docs/ README.md README.zh-CN.md internal/assets/_data/docs/
git commit -m "docs: opt-in polaris canary adapter usage + troubleshooting (EN+ZH)"
```

---

## Task 7: 全量回归闸门

**Files:** 无（验证）

- [ ] **Step 1: 质量闸门**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*') && go vet ./... && go build ./... && go build . && go test ./... -count=1`
Expected: gofmt 输出为空；vet/build 成功；全部测试 PASS。

- [ ] **Step 2: smoke**

Run: `./scripts/smoke.sh`
Expected: `smoke OK`

- [ ] **Step 3: golden 完整性复核**

Run: `git diff --stat origin/main -- internal/scaffold/mono/testdata internal/scaffold/rpc/testdata internal/scaffold/bff/testdata`
Expected: 无输出（默认 scaffold golden 未被触碰；仅 infra 新增 golden）。

---

## 任务依赖图

```
T1(asset+sanity) → T2(infra kind) → T3(golden) → T4(MCP/CLI)
T1 ───────────────→ T5(编译验证; 可能回修 T1 sdkClient)
T4/T5 → T6(文档) → T7(回归)
```

T5 可能在 SDK API 裁决时回修 T1 的 `sdkClient`（接口与测试不变）。T6 依赖 T4/T5 的最终命令与版本结论。

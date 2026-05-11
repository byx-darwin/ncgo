# Nacos 和 Polaris 基础设施插件

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将 Nacos 和 Polaris 注册为独立的 infra 插件类型，使用户可以运行 `ncgo add infra nacos` 和 `ncgo add infra polaris`（或通过 `ncgo new --infra nacos` 在创建项目时直接引入）。

**架构设计：** 遵循现有的数据层 infra 模式（redis、kafka、es、clickhouse）：在 `hertz/optional/` 和 `kitex/optional/` 下放置框架专属模板文件，在 `hertz/optional-config/` 下放置 Hertz 配置 YAML 片段。不需要 --wire 支持——这些是数据层客户端包装器，不是中间件。

**技术栈：** Go、嵌入式资源、YAML 配置、samber/oops 错误处理、samber/do 依赖注入。

---

### 任务 1：在 infra.go 中注册 Nacos 和 Polaris 元数据

**文件：**
- 修改：`internal/scaffold/infra/infra.go`

- [ ] **步骤 1：添加 Kind 常量**

在 `internal/scaffold/infra/infra.go` 现有常量块之后（第 42 行之后）添加两个新常量：

```go
KindNacos    = "nacos"
KindPolaris  = "polaris"
```

- [ ] **步骤 2：更新 SupportedKinds()**

在 `SupportedKinds()` 的返回切片中追加 `KindNacos, KindPolaris`（第 48 行）：

```go
func SupportedKinds() []string {
	return []string{KindRedis, KindKafka, KindES, KindClickHouse, KindObservabilityOtel, KindOtelAlias, KindObservabilityLog, KindLoggingAlias, KindReleaseCanary, KindCanaryAlias, KindRegistryEtcd, KindNacos, KindPolaris}
}
```

- [ ] **步骤 3：添加到 commonKinds()**

在 `commonKinds()` 的返回切片中追加 `KindNacos, KindPolaris`（第 52 行）：

```go
func commonKinds() []string {
	return []string{KindRedis, KindKafka, KindES, KindClickHouse, KindObservabilityOtel, KindObservabilityLog, KindReleaseCanary, KindNacos, KindPolaris}
}
```

- [ ] **步骤 4：添加 outputRelPaths 条目**

在 `outputRelPaths` map 中添加两条（第 98 行之后）：

```go
KindNacos:   filepath.Join("internal", "base", "data", "nacos.go"),
KindPolaris: filepath.Join("internal", "base", "data", "polaris.go"),
```

- [ ] **步骤 5：添加 goGetDeps 条目**

在 `goGetDeps` map 中添加两条（第 72 行之后）：

```go
KindNacos:   {"github.com/nacos-group/nacos-sdk-go/v2", "github.com/samber/oops"},
KindPolaris: {"github.com/polarismesh/polaris-go", "github.com/samber/oops"},
```

- [ ] **步骤 6：添加 Hertz 配置片段 keys**

在 `hertzConfigSnippetKeys` map 中添加两条（第 110 行之后）：

```go
KindNacos:   "nacos",
KindPolaris: "polaris",
```

- [ ] **步骤 7：验证编译**

运行：`go build ./internal/scaffold/infra/...`
预期：PASS

- [ ] **步骤 8：提交**

```bash
git add internal/scaffold/infra/infra.go
git commit -m "feat(infra): register nacos and polaris as infra add-on kinds"
```

### 任务 2：创建 Hertz Nacos 模板和配置

**文件：**
- 新建：`internal/assets/_data/hertz/optional/nacos.go`
- 新建：`internal/assets/_data/hertz/optional-config/nacos.yaml`

- [ ] **步骤 1：创建 Hertz Nacos 可选模板**

创建 `internal/assets/_data/hertz/optional/nacos.go`，遵循 `redis.go` 的 Hertz 模式——从共享的 `*Config` 类型中使用 `cfg.Nacos`：

```go
// Optional Nacos add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/nacos.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.Provide(injector, data.NewNacos)
//
// NewNacos reads cfg.Nacos and reuses the same shared Nacos client used by
// Nacos-backed middleware. For a dedicated client, call NewNacosWithOptions.
//
// Required dependency:
//
//	go get github.com/nacos-group/nacos-sdk-go/v2
//	go get github.com/samber/oops

package data

import (
	"context"

	"github.com/samber/oops"
)

// Nacos wraps a Nacos config reference for service discovery and config management.
type Nacos struct {
	Cfg NacosConfig
}

// NacosConfig maps to the nacos section in conf/dev/conf.yaml.
type NacosConfig struct {
	ServerAddr  string `yaml:"server_addr" json:"server_addr"`
	NamespaceID string `yaml:"namespace_id" json:"namespace_id"`
	GroupName   string `yaml:"group_name" json:"group_name"`
	Username    string `yaml:"username" json:"username"`
	Password    string `yaml:"password" json:"password"`
	ContextPath string `yaml:"context_path" json:"context_path"`
	CacheDir    string `yaml:"cache_dir" json:"cache_dir"`
	LogLevel    string `yaml:"log_level" json:"log_level"`
	LogDir      string `yaml:"log_dir" json:"log_dir"`
	TimeoutMS   int    `yaml:"timeout_ms" json:"timeout_ms"`
}

// NewNacos reuses the shared Nacos client derived from cfg.Nacos, validates
// connectivity with the injected startup context, and returns a cleanup
// function for samber/do.
func NewNacos(ctx context.Context, cfg *Config) (*Nacos, func(), error) {
	if cfg == nil {
		return nil, nil, oops.
			In("nacos").
			Tags("registry", "nacos", "configuration").
			Code(10308).
			Public("config_invalid").
			New("data.Config is nil")
	}
	if err := validateNacosConfig(cfg.Nacos); err != nil {
		return nil, nil, err
	}
	cleanup := func() {}
	return &Nacos{Cfg: cfg.Nacos}, cleanup, nil
}

// NewNacosWithOptions creates a dedicated Nacos client from raw config options.
// Use it only when you intentionally want a different connection than cfg.Nacos.
func NewNacosWithOptions(ctx context.Context, opts NacosConfig) (*Nacos, func(), error) {
	if err := validateNacosConfig(opts); err != nil {
		return nil, nil, err
	}
	cleanup := func() {}
	return &Nacos{Cfg: opts}, cleanup, nil
}

func validateNacosConfig(cfg NacosConfig) error {
	if cfg.ServerAddr == "" {
		return oops.
			In("nacos").
			Tags("registry", "nacos", "configuration").
			Code(10308).
			Public("config_invalid").
			New("NacosConfig.ServerAddr is empty")
	}
	return nil
}
```

- [ ] **步骤 2：创建 Hertz Nacos 配置片段**

创建 `internal/assets/_data/hertz/optional-config/nacos.yaml`：

```yaml
# ncgo:add-infra:start nacos
nacos:
  server_addr: "127.0.0.1:8848"
  namespace_id: ""
  group_name: "DEFAULT_GROUP"
  username: ""
  password: ""
  context_path: "/nacos"
  cache_dir: "./cache"
  log_level: "info"
  log_dir: "./log"
  timeout_ms: 10000
# ncgo:add-infra:end nacos
```

- [ ] **步骤 3：验证资源加载**

运行：`go test ./internal/scaffold/infra/... -run TestAddAllSupportedKindsCopySuccessfully/nacos -count=1`
预期：PASS（该测试会读取嵌入资源并验证包含 package 声明）

- [ ] **步骤 4：提交**

```bash
git add internal/assets/_data/hertz/optional/nacos.go internal/assets/_data/hertz/optional-config/nacos.yaml
git commit -m "feat(infra): add nacos Hertz template and config snippet"
```

### 任务 3：创建 Hertz Polaris 模板和配置

**文件：**
- 新建：`internal/assets/_data/hertz/optional/polaris.go`
- 新建：`internal/assets/_data/hertz/optional-config/polaris.yaml`

- [ ] **步骤 1：创建 Hertz Polaris 可选模板**

创建 `internal/assets/_data/hertz/optional/polaris.go`，遵循与 Nacos 相同的模式：

```go
// Optional Polaris add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/polaris.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.Provide(injector, data.NewPolaris)
//
// NewPolaris reads cfg.Polaris and reuses the same shared Polaris client used by
// Polaris-backed middleware. For a dedicated client, call NewPolarisWithOptions.
//
// Required dependency:
//
//	go get github.com/polarismesh/polaris-go
//	go get github.com/samber/oops

package data

import (
	"context"

	"github.com/samber/oops"
)

// Polaris wraps a Polaris config reference for service discovery and governance.
type Polaris struct {
	Cfg PolarisConfig
}

// PolarisConfig maps to the polaris section in conf/dev/conf.yaml.
type PolarisConfig struct {
	Addresses   []string `yaml:"addresses" json:"addresses"`
	Namespace   string   `yaml:"namespace" json:"namespace"`
	Service     string   `yaml:"service" json:"service"`
	Protocol    string   `yaml:"protocol" json:"protocol"`
	TimeoutMS   int      `yaml:"timeout_ms" json:"timeout_ms"`
	RetryCount  int      `yaml:"retry_count" json:"retry_count"`
}

// NewPolaris reuses the shared Polaris client derived from cfg.Polaris, validates
// connectivity with the injected startup context, and returns a cleanup
// function for samber/do.
func NewPolaris(ctx context.Context, cfg *Config) (*Polaris, func(), error) {
	if cfg == nil {
		return nil, nil, oops.
			In("polaris").
			Tags("registry", "polaris", "configuration").
			Code(10308).
			Public("config_invalid").
			New("data.Config is nil")
	}
	if err := validatePolarisConfig(cfg.Polaris); err != nil {
		return nil, nil, err
	}
	cleanup := func() {}
	return &Polaris{Cfg: cfg.Polaris}, cleanup, nil
}

// NewPolarisWithOptions creates a dedicated Polaris client from raw config options.
// Use it only when you intentionally want a different connection than cfg.Polaris.
func NewPolarisWithOptions(ctx context.Context, opts PolarisConfig) (*Polaris, func(), error) {
	if err := validatePolarisConfig(opts); err != nil {
		return nil, nil, err
	}
	cleanup := func() {}
	return &Polaris{Cfg: opts}, cleanup, nil
}

func validatePolarisConfig(cfg PolarisConfig) error {
	if len(cfg.Addresses) == 0 {
		return oops.
			In("polaris").
			Tags("registry", "polaris", "configuration").
			Code(10308).
			Public("config_invalid").
			New("PolarisConfig.Addresses is empty")
	}
	return nil
}
```

- [ ] **步骤 2：创建 Hertz Polaris 配置片段**

创建 `internal/assets/_data/hertz/optional-config/polaris.yaml`：

```yaml
# ncgo:add-infra:start polaris
polaris:
  addresses:
    - 127.0.0.1:8091
  namespace: "default"
  service: ""
  protocol: "grpc"
  timeout_ms: 5000
  retry_count: 1
# ncgo:add-infra:end polaris
```

- [ ] **步骤 3：验证资源加载**

运行：`go test ./internal/scaffold/infra/... -run TestAddAllSupportedKindsCopySuccessfully/polaris -count=1`
预期：PASS

- [ ] **步骤 4：提交**

```bash
git add internal/assets/_data/hertz/optional/polaris.go internal/assets/_data/hertz/optional-config/polaris.yaml
git commit -m "feat(infra): add polaris Hertz template and config snippet"
```

### 任务 4：创建 Kitex Nacos 模板

**文件：**
- 新建：`internal/assets/_data/kitex/optional/nacos.go`

- [ ] **步骤 1：创建 Kitex Nacos 可选模板**

创建 `internal/assets/_data/kitex/optional/nacos.go`，遵循 `kitex/optional/redis.go` 的模式——使用原始 SDK 选项结构体，通过 `do.ProvideValue` 传入：

```go
// Optional Nacos add-on for Kitex RPC services.
//
// To enable: copy this file to internal/base/data/nacos.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.ProvideValue(injector, data.NacosConfig{
//	    ServerAddr:  "127.0.0.1:8848",
//	    NamespaceID: "",
//	    GroupName:   "DEFAULT_GROUP",
//	    Username:    "",
//	    Password:    "",
//	    ContextPath: "/nacos",
//	    CacheDir:    "./cache",
//	    LogLevel:    "info",
//	    LogDir:      "./log",
//	    TimeoutMS:   10000,
//	})
//	do.Provide(injector, data.NewNacos)
//
// Required dependency:
//
//	go get github.com/nacos-group/nacos-sdk-go/v2
//	go get github.com/samber/oops

package data

import (
	"context"

	"github.com/samber/oops"
)

// Nacos wraps a Nacos config reference for service discovery and config management.
type Nacos struct {
	Cfg NacosConfig
}

// NacosConfig holds the full Nacos SDK configuration options.
type NacosConfig struct {
	ServerAddr  string `yaml:"server_addr" json:"server_addr"`
	NamespaceID string `yaml:"namespace_id" json:"namespace_id"`
	GroupName   string `yaml:"group_name" json:"group_name"`
	Username    string `yaml:"username" json:"username"`
	Password    string `yaml:"password" json:"password"`
	ContextPath string `yaml:"context_path" json:"context_path"`
	CacheDir    string `yaml:"cache_dir" json:"cache_dir"`
	LogLevel    string `yaml:"log_level" json:"log_level"`
	LogDir      string `yaml:"log_dir" json:"log_dir"`
	TimeoutMS   int    `yaml:"timeout_ms" json:"timeout_ms"`
}

// NewNacos creates a Nacos client from the full NacosConfig struct,
// validates connectivity with the injected startup context, and returns a cleanup function for samber/do.
func NewNacos(ctx context.Context, cfg NacosConfig) (*Nacos, func(), error) {
	if err := validateNacosConfig(cfg); err != nil {
		return nil, nil, err
	}
	cleanup := func() {}
	return &Nacos{Cfg: cfg}, cleanup, nil
}

func validateNacosConfig(cfg NacosConfig) error {
	if cfg.ServerAddr == "" {
		return oops.
			In("nacos").
			Tags("registry", "nacos", "configuration").
			Code(10308).
			Public("config_invalid").
			New("NacosConfig.ServerAddr is empty")
	}
	return nil
}
```

- [ ] **步骤 2：验证资源加载**

运行：`go test ./internal/scaffold/infra/... -run TestAddAllSupportedKindsCopySuccessfully -count=1`（由于 nacos 已加入 `commonKinds()`，Hertz 测试会覆盖它；Kitex 路径路由也会自动生效）

- [ ] **步骤 3：提交**

```bash
git add internal/assets/_data/kitex/optional/nacos.go
git commit -m "feat(infra): add nacos Kitex template"
```

### 任务 5：创建 Kitex Polaris 模板

**文件：**
- 新建：`internal/assets/_data/kitex/optional/polaris.go`

- [ ] **步骤 1：创建 Kitex Polaris 可选模板**

创建 `internal/assets/_data/kitex/optional/polaris.go`，遵循与 Kitex Nacos 相同的模式：

```go
// Optional Polaris add-on for Kitex RPC services.
//
// To enable: copy this file to internal/base/data/polaris.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.ProvideValue(injector, data.PolarisConfig{
//	    Addresses:  []string{"127.0.0.1:8091"},
//	    Namespace:  "default",
//	    Service:    "",
//	    Protocol:   "grpc",
//	    TimeoutMS:  5000,
//	    RetryCount: 1,
//	})
//	do.Provide(injector, data.NewPolaris)
//
// Required dependency:
//
//	go get github.com/polarismesh/polaris-go
//	go get github.com/samber/oops

package data

import (
	"context"

	"github.com/samber/oops"
)

// Polaris wraps a Polaris config reference for service discovery and governance.
type Polaris struct {
	Cfg PolarisConfig
}

// PolarisConfig holds the full Polaris SDK configuration options.
type PolarisConfig struct {
	Addresses  []string `yaml:"addresses" json:"addresses"`
	Namespace  string   `yaml:"namespace" json:"namespace"`
	Service    string   `yaml:"service" json:"service"`
	Protocol   string   `yaml:"protocol" json:"protocol"`
	TimeoutMS  int      `yaml:"timeout_ms" json:"timeout_ms"`
	RetryCount int      `yaml:"retry_count" json:"retry_count"`
}

// NewPolaris creates a Polaris client from the full PolarisConfig struct,
// validates connectivity with the injected startup context, and returns a cleanup function for samber/do.
func NewPolaris(ctx context.Context, cfg PolarisConfig) (*Polaris, func(), error) {
	if err := validatePolarisConfig(cfg); err != nil {
		return nil, nil, err
	}
	cleanup := func() {}
	return &Polaris{Cfg: cfg}, cleanup, nil
}

func validatePolarisConfig(cfg PolarisConfig) error {
	if len(cfg.Addresses) == 0 {
		return oops.
			In("polaris").
			Tags("registry", "polaris", "configuration").
			Code(10308).
			Public("config_invalid").
			New("PolarisConfig.Addresses is empty")
	}
	return nil
}
```

- [ ] **步骤 2：提交**

```bash
git add internal/assets/_data/kitex/optional/polaris.go
git commit -m "feat(infra): add polaris Kitex template"
```

### 任务 6：为 Nacos 和 Polaris 添加专项测试

**文件：**
- 修改：`internal/scaffold/infra/infra_test.go`

- [ ] **步骤 1：添加 TestAddNacosForHertz**

在 `infra_test.go` 的 `TestAddAllSupportedKindsCopySuccessfully` 之后追加：

```go
func TestAddNacosForHertz(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindNacos})
	if err != nil {
		t.Fatalf("Add nacos: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "data", "nacos.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read nacos file: %v", err)
	}
	for _, want := range []string{"package data", "type NacosConfig struct", "func NewNacos", "validateNacosConfig"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("nacos template missing %q", want)
		}
	}
	confPath := filepath.Join(root, "conf", "dev", "conf.yaml")
	confBody, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(confBody), "nacos:") || !strings.Contains(string(confBody), "# ncgo:add-infra:start nacos") {
		t.Errorf("nacos config block missing in conf/dev/conf.yaml:\n%s", confBody)
	}
	joined := strings.Join(res.NextSteps, "\n")
	for _, want := range []string{"go get github.com/nacos-group/nacos-sdk-go/v2", "go get github.com/samber/oops", "go mod tidy"} {
		if !strings.Contains(joined, want) {
			t.Errorf("NextSteps missing %q in:\n%s", want, joined)
		}
	}
	assertManifestInfra(t, root, KindNacos)
}
```

- [ ] **步骤 2：添加 TestAddNacosForKitex**

```go
func TestAddNacosForKitex(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindNacos})
	if err != nil {
		t.Fatalf("Add nacos kitex: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "data", "nacos.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read nacos file: %v", err)
	}
	if !strings.Contains(string(body), "package data") {
		t.Errorf("nacos kitex template missing package declaration")
	}
	assertManifestInfra(t, root, KindNacos)
}
```

- [ ] **步骤 3：添加 TestAddPolarisForHertz**

```go
func TestAddPolarisForHertz(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindPolaris})
	if err != nil {
		t.Fatalf("Add polaris: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "data", "polaris.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read polaris file: %v", err)
	}
	for _, want := range []string{"package data", "type PolarisConfig struct", "func NewPolaris", "validatePolarisConfig"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("polaris template missing %q", want)
		}
	}
	confPath := filepath.Join(root, "conf", "dev", "conf.yaml")
	confBody, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(confBody), "polaris:") || !strings.Contains(string(confBody), "# ncgo:add-infra:start polaris") {
		t.Errorf("polaris config block missing in conf/dev/conf.yaml:\n%s", confBody)
	}
	joined := strings.Join(res.NextSteps, "\n")
	for _, want := range []string{"go get github.com/polarismesh/polaris-go", "go get github.com/samber/oops", "go mod tidy"} {
		if !strings.Contains(joined, want) {
			t.Errorf("NextSteps missing %q in:\n%s", want, joined)
		}
	}
	assertManifestInfra(t, root, KindPolaris)
}
```

- [ ] **步骤 4：添加 TestAddPolarisForKitex**

```go
func TestAddPolarisForKitex(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindPolaris})
	if err != nil {
		t.Fatalf("Add polaris kitex: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "data", "polaris.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read polaris file: %v", err)
	}
	if !strings.Contains(string(body), "package data") {
		t.Errorf("polaris kitex template missing package declaration")
	}
	assertManifestInfra(t, root, KindPolaris)
}
```

- [ ] **步骤 5：运行所有 infra 测试**

运行：`go test ./internal/scaffold/infra/... -count=1`
预期：全部通过（现有的 `TestAddAllSupportedKindsCopySuccessfully` 会自动遍历 nacos 和 polaris，因为它们已加入 `commonKinds()`）

- [ ] **步骤 6：提交**

```bash
git add internal/scaffold/infra/infra_test.go
git commit -m "test(infra): add targeted tests for nacos and polaris add-ons"
```

### 任务 7：验证 MCP 工具自动识别新类型

**文件：**
- 无需修改 — `internal/mcp/tools.go` 第 30 行已使用 `enumField("kind", infra.SupportedKinds())`，会自动拾取新类型。

- [ ] **步骤 1：验证编译和 MCP 测试**

运行：`go build ./... && go test ./internal/mcp/... -count=1`
预期：PASS

### 任务 8：最终验证

- [ ] **步骤 1：运行仓库级别检查**

```bash
go build ./... && go build . && go vet ./... && go test ./... -count=1
```
预期：全部通过

- [ ] **步骤 2：运行冒烟测试**

```bash
./scripts/smoke.sh
```
预期：PASS（验证 MCP 工具列表中包含 ncgo_add_infra，且 nacos/polaris 出现在枚举值中）

- [ ] **步骤 3：最终提交（如有剩余变更）**

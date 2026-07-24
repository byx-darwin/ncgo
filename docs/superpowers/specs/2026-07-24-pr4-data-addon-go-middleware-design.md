# PR4 — 数据类 add-on 接入 go-middleware（kafka/es/clickhouse）设计文档

- 状态：approved
- 日期：2026-07-24
- 关联：Issue #9；设计总纲 `specs/017-go-tools-v0.1.0-adaptation.md`
- 前置：PR1（go.mod 基础 + 错误机制，Issue #6）、PR3（logging + redis，Issue #8，commit aa786c5）

## 1. 背景

ncgo 的 kafka/es/clickhouse optional add-on 模板当前直接引用第三方库（`segmentio/kafka-go`、`elastic/go-elasticsearch/v8`、`ClickHouse/clickhouse-go/v2`），手写连接构建、验证和清理逻辑。go-tools v0.1.0 的 `go-middleware` 模块已提供这三个中间件的标准化 Config + 工厂方法。PR4 将模板从手写实现迁移到 go-middleware 工厂委托，与 PR3 redis 迁移模式对齐。

## 2. 已确认的决策

| 维度 | 决策 |
|------|------|
| 包装结构 | **保留**现有包装结构体（KafkaWriter/KafkaReader/ES/ClickHouse）+ samber/do DI 模式，内部委托 go-middleware 工厂 |
| Config YAML 对齐 | optional-config snippet 字段名对齐 go-middleware Config 的 yaml tag，生成项目可直接 unmarshal |
| Kitex 错误码 | kitex 模板错误码对齐 hertz 风格（frameworkerror.CodeConfigInvalid + 项目段/go-middleware 码），不再使用字符串码 |
| ClickHouse 错误码 | 使用 go-middleware/clickhouse 预定义码（CodeConnect=20401、CodeQuery=20402），删除项目段 CodeDatabaseUnavailable(40503) 及 init() 注册 |
| ES/Kafka 错误码 | go-middleware 无预定义码，继续使用项目段码（ES: CodeSearchUnavailable=40506）；kafka 配置错误用 frameworkerror.CodeConfigInvalid |

## 3. 变更范围

### 3.1 Kafka（hertz + kitex）

**当前**：模板接收 raw `*kafka.Writer` / `kafka.ReaderConfig`，直接包装。

**迁移后**：

- `KafkaWriter` 内部从 `*kafka.Writer` 改为 `*mwkafka.Writer`
- `KafkaReader` 内部从 `*kafka.Reader` 改为 `*mwkafka.Consumer`
- `NewKafkaWriter(cfg mwkafka.WriterConfig)` — 验证 `cfg.Broker` 非空、`cfg.Topic` 非空，调用 `mwkafka.NewWriter(cfg)`
- `NewKafkaReader(cfg mwkafka.ReaderConfig)` — 验证 `cfg.Broker` 非空、`cfg.Topic` 非空，调用 `mwkafka.NewConsumer(cfg)`
- 清理函数调用 `w.Close()` / `c.Close()`
- 文件头注释：用法示例改为传递 `mwkafka.WriterConfig`/`mwkafka.ReaderConfig`，依赖改为 `go get go-middleware`
- 错误码：hertz 保持 `frameworkerror.CodeConfigInvalid`；kitex 从字符串码对齐为 `frameworkerror.CodeConfigInvalid`

**import 变更**：
```go
// before
import "github.com/segmentio/kafka-go"

// after
import mwkafka "github.com/byx-darwin/go-tools/go-middleware/kafka"
```

### 3.2 Elasticsearch（hertz + kitex）

**当前**：模板接收 raw `elasticsearch.Config`，调用 `elasticsearch.NewClient(cfg)`，Ping 验证。

**迁移后**：

- `ES` 包装结构体保持 `Client *elasticsearch.Client`（go-middleware/es 返回原生客户端）
- `NewES(ctx context.Context, cfg mwes.Config)` — 验证 `cfg.Addresses` 非空，调用 `mwes.NewClient(cfg)`，Ping 验证连通性
- 保留项目段码 `CodeSearchUnavailable = 40506` + `init()` 注册 HTTP 503 映射（go-middleware/es 无预定义码）
- 配置错误使用 `frameworkerror.CodeConfigInvalid`；连接错误使用 `CodeSearchUnavailable`
- kitex 模板从字符串码对齐为相同的数值码
- 文件头注释：用法示例改为传递 `mwes.Config`，依赖改为 `go get go-middleware`

**import 变更**：
```go
// before
import "github.com/elastic/go-elasticsearch/v8"

// after
import (
    "github.com/elastic/go-elasticsearch/v8" // 仅用于 ES.Client 字段类型
    mwes "github.com/byx-darwin/go-tools/go-middleware/es"
)
```

> 注意：ES 包装结构体的 `Client` 字段类型仍是 `*elasticsearch.Client`（go-middleware/es 的返回类型），因此 `go-elasticsearch/v8` 仍作为 import 出现，但不再直接调用其构造函数。

### 3.3 ClickHouse（hertz + kitex）

**当前**：模板接收 raw `*clickhouse.Options`，调用 `clickhouse.Open(opts)`，Ping 验证。使用项目段码 `CodeDatabaseUnavailable = 40503`。

**迁移后**：

- `ClickHouse` 包装结构体保持 `Conn clickhouse.Conn`（go-middleware/clickhouse 返回 `clickhouse.Conn` 接口）
- `NewClickHouse(ctx context.Context, cfg mwclickhouse.Config)` — 验证 `cfg.Addrs` 非空（或 `cfg.DSN` 非空），调用 `mwclickhouse.NewClient(cfg)`，Ping 验证连通性
- 错误码改用 go-middleware 预定义码：配置错误用 `frameworkerror.CodeConfigInvalid`，连接错误用 `mwclickhouse.CodeConnect`（20401，已注册 HTTP 503）
- 删除项目段码 `CodeDatabaseUnavailable = 40503` 及其 `init()` 注册
- kitex 模板从字符串码对齐为相同的数值码
- 文件头注释：用法示例改为传递 `mwclickhouse.Config`，依赖改为 `go get go-middleware`

**import 变更**：
```go
// before
import (
    "github.com/ClickHouse/clickhouse-go/v2"
    "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// after
import (
    "github.com/ClickHouse/clickhouse-go/v2" // 仅用于 ClickHouse.Conn 字段类型（clickhouse.Conn 接口）
    mwclickhouse "github.com/byx-darwin/go-tools/go-middleware/clickhouse"
)
```

> 注意：`clickhouse.Conn` 接口类型来自 `clickhouse-go/v2`，因此该依赖仍出现在 import 中，但不再直接调用 `clickhouse.Open`。`driver` 包不再需要。

### 3.4 optional-config YAML snippet 对齐

将 `hertz/optional-config/{kafka,es,clickhouse}.yaml` 的字段名对齐 go-middleware Config 的 yaml tag：

**kafka.yaml**：
```yaml
kafka:
  producer:
    enabled: false
    broker:          # was: brokers
      - 127.0.0.1:9092
    topic: demo-events
    allow_auto_topic_creation: false
    tls:
      enable: false
      insecure_skip_verify: false
    sasl:
      enable: false
      user: ""
      password: ""
  consumer:
    enabled: false
    broker:          # was: brokers
      - 127.0.0.1:9092
    group_id: demo
    topic: demo-events
    min_bytes: 10000
    max_bytes: 10485760
    max_wait: 500ms
    read_batch_timeout: 0s
    tls:
      enable: false
      insecure_skip_verify: false
    sasl:
      enable: false
      user: ""
      password: ""
```

移除当前 snippet 中 go-middleware WriterConfig/ReaderConfig 不支持的字段（balancer、required_acks、async、batch_size、batch_bytes、batch_timeout_milliseconds、queue_capacity、start_offset）。

**es.yaml**：
```yaml
es:
  enabled: false
  addresses:
    - http://127.0.0.1:9200
  username: ""
  password: ""
  api_key: ""
  cloud_id: ""
  max_retries: 3
  max_idle_conns_per_host: 0
  tls:
    enable: false
    insecure_skip_verify: false
```

移除 go-middleware es.Config 不支持的字段（service_token、compress_request_body、discover_nodes_on_start、enable_metrics、enable_debug_logger）。新增 cloud_id、max_idle_conns_per_host、tls。

**clickhouse.yaml**：
```yaml
clickhouse:
  enabled: false
  dsn: ""            # 新增：DSN 连接串（与独立字段二选一，优先 DSN）
  addrs:             # was: addr
    - 127.0.0.1:9000
  database: default
  username: default
  password: ""
  dial_timeout: 5    # was: dial_timeout_seconds（秒，int）
  max_open_conns: 5
  max_idle_conns: 5
  conn_max_lifetime: 300  # was: conn_max_lifetime_seconds（秒，int）
  compress: true     # was: compress: lz4（改为 bool）
  tls:
    enable: false
    insecure_skip_verify: false
```

移除 go-middleware clickhouse.Config 不支持的字段（protocol、block_buffer_size）。compress 从枚举字符串改为布尔值。

### 3.5 infra.go goGetDeps 更新

```go
// before
KindKafka:      {"github.com/segmentio/kafka-go", "github.com/byx-darwin/go-tools/go-common"},
KindES:         {"github.com/elastic/go-elasticsearch/v8", "github.com/byx-darwin/go-tools/go-common"},
KindClickHouse: {"github.com/ClickHouse/clickhouse-go/v2", "github.com/byx-darwin/go-tools/go-common"},

// after
KindKafka:      {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"},
KindES:         {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"},
KindClickHouse: {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"},
```

### 3.6 Golden 测试更新

受影响的 golden 快照（infra testdata）：
- 当前无 kafka/es/clickhouse 专用 golden testdata 目录（仅 redis 和 logging 有）
- 如 infra_test.go 中有 kafka/es/clickhouse 的文本断言（goGetDeps next-steps），需同步更新
- mono golden 测试不受影响（kafka/es/clickhouse 是 optional add-on，不在默认生成中）

### 3.7 文档更新

- `README.md` / `README.zh-CN.md`：kafka/es/clickhouse add-on 依赖说明从第三方库改为 go-middleware
- `docs/examples.md` / `docs/examples.zh-CN.md`：如有 `ncgo add infra kafka/es/clickhouse` 示例，更新 next-steps 输出

## 4. 受影响文件清单

| 文件 | 变更类型 |
|------|----------|
| `internal/assets/_data/hertz/optional/kafka.go` | 重写 |
| `internal/assets/_data/hertz/optional/es.go` | 重写 |
| `internal/assets/_data/hertz/optional/clickhouse.go` | 重写 |
| `internal/assets/_data/kitex/optional/kafka.go` | 重写 |
| `internal/assets/_data/kitex/optional/es.go` | 重写 |
| `internal/assets/_data/kitex/optional/clickhouse.go` | 重写 |
| `internal/assets/_data/hertz/optional-config/kafka.yaml` | 字段对齐 |
| `internal/assets/_data/hertz/optional-config/es.yaml` | 字段对齐 |
| `internal/assets/_data/hertz/optional-config/clickhouse.yaml` | 字段对齐 |
| `internal/scaffold/infra/infra.go` | goGetDeps 更新 |
| `internal/scaffold/infra/infra_test.go` | 断言更新（如有） |
| `README.md` | 文档对齐 |
| `README.zh-CN.md` | 文档对齐 |
| `docs/examples.md` | 文档对齐（如涉及） |
| `docs/examples.zh-CN.md` | 文档对齐（如涉及） |

## 5. 验证策略

1. `go build ./...` — 编译通过
2. `go vet ./...` — 静态检查通过
3. `go test ./internal/scaffold/infra/... -count=1` — infra 单元测试
4. `go test ./... -count=1` — 全量测试
5. `./scripts/smoke.sh` — 端到端 smoke
6. 手动审查 `ncgo add infra kafka/es/clickhouse` 的 next-steps 输出

## 6. 风险

- **go-elasticsearch/v8 仍为 import 依赖**：ES 包装结构体的 `Client` 字段类型是 `*elasticsearch.Client`（go-middleware 返回类型），所以 `go-elasticsearch/v8` 仍出现在 import 中。`go get go-middleware` 会传递引入它，但 goGetDeps next-steps 不再显式列出它。这是预期行为。
- **clickhouse-go/v2 仍为 import 依赖**：同理，`clickhouse.Conn` 接口来自该包。
- **optional-config snippet breaking change**：字段名变更（brokers→broker、addr→addrs 等）对已生成的项目是 breaking 的，但 ncgo 模板追踪 go-tools（干净切换决策），旧项目自行迁移。
- **kafka API 表面变化**：KafkaWriter/KafkaReader 的内部类型变化，生成项目的 DI 注入方式从传递 raw kafka-go 结构体改为传递 go-middleware Config。这是 PR4 的核心意图。

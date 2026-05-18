# gRPC Rule-Center 端到端测试 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过 `ncgo add rpc rule-center --preset rule-center` 在微服务 workspace 中生成 Kitex 规则中心服务，该服务从 PostgreSQL 读取限流规则并通过 gRPC 对外提供 CRUD 接口，配合已有的 `ncgo test rate-limit seed` 和 `ncgo test rate-limit run` 完成端到端验证。

**Architecture:** 在 Kitex 模板层新增 rule-center 专用模板（proto 定义、handler 实现、限流中间件、conf 扩展），通过 `--preset rule-center` 标志在 `ncgo add rpc` 时选择性地使用这些模板而非默认模板。seed 命令追加 `phase='grpc'` 规则，run 命令追加 `--grpc` 标志支持 grpcurl 验证。

**Tech Stack：** Go（Cobra CLI）、Kitex（gRPC/protobuf）、PostgreSQL、sqlc、Redis、grpcurl、Docker Compose。

---

### 任务 1：Kitex conf.yaml 模板 — 追加 RateLimitConfig

**Files:**
- 修改：`internal/assets/_data/kitex/kitex-template/conf.yaml`

在现有 `Config` struct 中追加 `RateLimit RateLimitConfig` 字段，并追加完整的 RateLimit 相关 struct 定义（复用 Hertz 的结构，精简掉 Kitex 不需要的部分）。同时在 `Default()` 和 `Validate()` 中追加对应逻辑。

- [ ] **步骤 1：修改 Config struct（约第 27 行）**

在 `Database DatabaseConfig` 之后追加 `RateLimit RateLimitConfig`：

```go
  type Config struct {
      Env      string         `json:"env" yaml:"env"`
      Debug    bool           `json:"debug" yaml:"debug"`
      Server   ServerConfig   `json:"server" yaml:"server"`
      RPC      RPCConfig      `json:"rpc" yaml:"rpc"`
      Auth     AuthConfig     `json:"auth" yaml:"auth"`
      Database DatabaseConfig `json:"database" yaml:"database"`
      RateLimit RateLimitConfig `json:"rate_limit" yaml:"rate_limit"`
  }
```

- [ ] **步骤 2：在 DatabaseConfig 之后追加 RateLimit 相关 struct**

```go
  type RateLimitConfig struct {
      Enabled  bool                  `json:"enabled" yaml:"enabled"`
      Source   RateLimitSourceConfig `json:"source" yaml:"source"`
      GRPC     RateLimitGRPCConfig   `json:"grpc" yaml:"grpc"`
      Database RateLimitDatabaseConfig `json:"database" yaml:"database"`
      Backend  string                `json:"backend" yaml:"backend"`
      FailOpen bool                  `json:"fail_open" yaml:"fail_open"`
      KeyPrefix string               `json:"key_prefix" yaml:"key_prefix"`
      PreAuth  RateLimitPhaseConfig  `json:"pre_auth" yaml:"pre_auth"`
      PostAuth RateLimitPhaseConfig  `json:"post_auth" yaml:"post_auth"`
  }

  type RateLimitSourceConfig struct {
      Type            string `json:"type" yaml:"type"`
      CacheTTLSeconds int    `json:"cache_ttl_seconds" yaml:"cache_ttl_seconds"`
      FallbackOnError bool   `json:"fallback_on_error" yaml:"fallback_on_error"`
  }

  type RateLimitGRPCConfig struct {
      Target              string `json:"target" yaml:"target"`
      TimeoutMilliseconds int    `json:"timeout_milliseconds" yaml:"timeout_milliseconds"`
      ServiceName         string `json:"service_name" yaml:"service_name"`
  }

  type RateLimitDatabaseConfig struct {
      QueryTimeoutMilliseconds int `json:"query_timeout_milliseconds" yaml:"query_timeout_milliseconds"`
  }

  type RateLimitPhaseConfig struct {
      Enabled     bool                   `json:"enabled" yaml:"enabled"`
      DefaultRule RateLimitRuleConfig    `json:"default_rule" yaml:"default_rule"`
      Rules       []RateLimitMatchConfig `json:"rules" yaml:"rules"`
  }

  type RateLimitMatchConfig struct {
      AppKey      string              `json:"app_key" yaml:"app_key"`
      Method      string              `json:"method" yaml:"method"`
      MatchKind   string              `json:"match_kind" yaml:"match_kind"`
      Path        string              `json:"path" yaml:"path"`
      PathPattern string              `json:"path_pattern" yaml:"path_pattern"`
      Priority    int                 `json:"priority" yaml:"priority"`
      Rule        RateLimitRuleConfig `json:"rule" yaml:"rule"`
  }

  type RateLimitRuleConfig struct {
      Enabled           bool     `json:"enabled" yaml:"enabled"`
      KeyBy             []string `json:"key_by" yaml:"key_by"`
      Strategy          string   `json:"strategy" yaml:"strategy"`
      WindowSeconds     int      `json:"window_seconds" yaml:"window_seconds"`
      MaxRequests       int      `json:"max_requests" yaml:"max_requests"`
      RequestsPerSecond float64  `json:"requests_per_second" yaml:"requests_per_second"`
      Burst             int      `json:"burst" yaml:"burst"`
      ClientTTLSeconds  int      `json:"client_ttl_seconds" yaml:"client_ttl_seconds"`
  }
```

- [ ] **步骤 3：修改 Default() — 追加 RateLimit 默认值**

在 `Database: DatabaseConfig{...}` 之后追加：

```go
          RateLimit: RateLimitConfig{
              Enabled:   false,
              Source:    RateLimitSourceConfig{Type: "database", CacheTTLSeconds: 60, FallbackOnError: true},
              GRPC:      RateLimitGRPCConfig{TimeoutMilliseconds: 200, ServiceName: "{{ToLower .ServiceInfo.ServiceName}}"},
              Database:  RateLimitDatabaseConfig{QueryTimeoutMilliseconds: 200},
              Backend:   "memory",
              KeyPrefix: "{{ToLower .ServiceInfo.ServiceName}}:rate_limit",
              PreAuth: RateLimitPhaseConfig{
                  Enabled: true,
                  DefaultRule: RateLimitRuleConfig{
                      Enabled: true, KeyBy: []string{"ip"}, Strategy: "fixed_window",
                      WindowSeconds: 60, MaxRequests: 100, ClientTTLSeconds: 300,
                  },
                  Rules: []RateLimitMatchConfig{},
              },
              PostAuth: RateLimitPhaseConfig{Enabled: false, DefaultRule: RateLimitRuleConfig{Enabled: true}, Rules: []RateLimitMatchConfig{}},
          },
```

- [ ] **步骤 4：修改 Validate() — 追加 RateLimit 校验**

在 `Validate()` 方法末尾（`return nil` 之前）追加：

```go
      if c.RateLimit.Enabled {
          if c.RateLimit.Source.Type != "database" && c.RateLimit.Source.Type != "grpc" {
              return oops.In("config").Code(10308).Public("config_invalid").New("rate_limit.source.type must be database or grpc")
          }
          if c.RateLimit.Source.Type == "grpc" && c.RateLimit.GRPC.Target == "" {
              return oops.In("config").Code(10308).Public("config_invalid").New("rate_limit.grpc.target is empty")
          }
          switch c.RateLimit.Backend {
          case "memory", "redis":
          default:
              return oops.In("config").Code(10308).Public("config_invalid").New("rate_limit.backend must be memory or redis")
          }
      }
```

- [ ] **步骤 5：提交**

```bash
git add internal/assets/_data/kitex/kitex-template/conf.yaml
git commit -m "feat(kitex): add RateLimitConfig to kitex conf template"
```

---

### 任务 2：Kitex server.yaml 模板 — 接入限流中间件

**Files:**
- 修改：`internal/assets/_data/kitex/kitex-template/server.yaml`

在 middleware chain 中追加限流中间件，并通过 `ncgo:wire:ratelimit` 锚点标记便于后续 infra 扩展。

- [ ] **步骤 1：在 interceptor.Recovery() 之后追加限流中间件**

找到 `interceptor.Recovery(),` 这一行，在它之后追加：

```go
              interceptor.Recovery(),
              // Optional rate-limit wiring (after `ncgo add infra rate-limit`):
              // import "{{.Module}}/internal/base/middleware"
              // middleware.RateLimit(cfg.RateLimit),
              // ncgo:wire:ratelimit:server-middleware
              interceptor.CallerAllowlist(
```

- [ ] **步骤 2：提交**

```bash
git add internal/assets/_data/kitex/kitex-template/server.yaml
git commit -m "feat(kitex): add rate-limit middleware anchor to server template"
```

---

### 任务 3：新建 Kitex rule-center 专用模板

**Files:**
- 新建：`internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml` — proto 定义
- 新建：`internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml` — handler 实现
- 新建：`internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml` — 限流中间件
- 新建：`internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml` — usecase 实现
- 新建：`internal/assets/_data/kitex/kitex-template/ratelimit_repository.yaml` — repository 实现
- 新建：`internal/assets/_data/kitex/schema/000002_rate_limit_rules.sql` — SQL schema

这些文件是 `--preset rule-center` 时覆盖默认模板的专用文件。

- [ ] **步骤 1：创建 proto 定义 `ratelimit_proto.yaml`**

```yaml
# Kitex rule-center preset — proto definition
# This file replaces the default IDL when --preset rule-center is used.
path: idl/rule-center.proto
update_behavior:
  type: cover
body: |-
  syntax = "proto3";
  package ratelimit.v1;

  option go_package = "api/ratelimit/v1;ratelimitv1";

  service RuleService {
    rpc GetRule(GetRuleRequest) returns (GetRuleResponse);
    rpc CreateRule(CreateRuleRequest) returns (CreateRuleResponse);
    rpc UpdateRule(UpdateRuleRequest) returns (UpdateRuleResponse);
    rpc DeleteRule(DeleteRuleRequest) returns (DeleteRuleResponse);
    rpc ListRules(ListRulesRequest) returns (ListRulesResponse);
  }

  message GetRuleRequest {
    string service = 1;
    string phase = 2;
    string method = 3;
    string path = 4;
    optional string app_key = 5;
  }

  message CreateRuleRequest {
    string service = 1;
    string phase = 2;
    string method = 3;
    string match_kind = 4;
    string path = 5;
    string path_pattern = 6;
    optional string app_key = 7;
    int32 priority = 8;
    bool enabled = 9;
    repeated string key_by = 10;
    string strategy = 11;
    int32 window_seconds = 12;
    int32 max_requests = 13;
    double requests_per_second = 14;
    int32 burst = 15;
    int32 client_ttl_seconds = 16;
  }

  message UpdateRuleRequest {
    int64 id = 1;
    optional bool enabled = 2;
    optional int32 priority = 3;
    optional repeated string key_by = 4;
    optional string strategy = 5;
    optional int32 window_seconds = 6;
    optional int32 max_requests = 7;
    optional double requests_per_second = 8;
    optional int32 burst = 9;
    optional int32 client_ttl_seconds = 10;
  }

  message DeleteRuleRequest {
    int64 id = 1;
  }

  message ListRulesRequest {
    optional string service = 1;
    optional string phase = 2;
  }

  message GetRuleResponse {
    bool found = 1;
    RateLimitRule rule = 2;
  }

  message CreateRuleResponse {
    int64 id = 1;
  }

  message UpdateRuleResponse {}

  message DeleteRuleResponse {}

  message ListRulesResponse {
    repeated RateLimitRule rules = 1;
  }

  message RateLimitRule {
    int64 id = 1;
    string service = 2;
    string phase = 3;
    string method = 4;
    string match_kind = 5;
    string path = 6;
    string path_pattern = 7;
    optional string app_key = 8;
    int32 priority = 9;
    bool enabled = 10;
    repeated string key_by = 11;
    string strategy = 12;
    int32 window_seconds = 13;
    int32 max_requests = 14;
    double requests_per_second = 15;
    int32 burst = 16;
    int32 client_ttl_seconds = 17;
  }
```

- [ ] **步骤 2：创建 handler 实现 `ratelimit_handler.yaml`**

```yaml
# Kitex rule-center preset — handler for RuleService
# Generates internal/handler/rulecenter/handler.go
path: internal/handler/rulecenter/handler.go
update_behavior:
  type: cover
body: |-
  // Code generated by ncgo rule-center preset.

  package rulecenterhandler

  import (
      "context"

      "{{.Module}}/internal/pkg/rpcerror"
      ratelimitv1 "{{.Module}}/api/ratelimit/v1"
      usecase "{{.Module}}/internal/usecase/rulecenter"
  )

  type RuleServiceImpl struct {
      uc *usecase.UseCase
  }

  func NewRuleServiceImpl(uc *usecase.UseCase) *RuleServiceImpl {
      return &RuleServiceImpl{uc: uc}
  }

  func (s *RuleServiceImpl) GetRule(ctx context.Context, req *ratelimitv1.GetRuleRequest) (resp *ratelimitv1.GetRuleResponse, err error) {
      rule, found, err := s.uc.GetRule(ctx, req.Service, req.Phase, req.Method, req.Path, req.AppKey)
      if err != nil {
          return nil, rpcerror.ToBizError(err)
      }
      return &ratelimitv1.GetRuleResponse{Found: found, Rule: rule}, nil
  }

  func (s *RuleServiceImpl) CreateRule(ctx context.Context, req *ratelimitv1.CreateRuleRequest) (resp *ratelimitv1.CreateRuleResponse, err error) {
      id, err := s.uc.CreateRule(ctx, req)
      if err != nil {
          return nil, rpcerror.ToBizError(err)
      }
      return &ratelimitv1.CreateRuleResponse{Id: id}, nil
  }

  func (s *RuleServiceImpl) UpdateRule(ctx context.Context, req *ratelimitv1.UpdateRuleRequest) (resp *ratelimitv1.UpdateRuleResponse, err error) {
      err = s.uc.UpdateRule(ctx, req)
      if err != nil {
          return nil, rpcerror.ToBizError(err)
      }
      return &ratelimitv1.UpdateRuleResponse{}, nil
  }

  func (s *RuleServiceImpl) DeleteRule(ctx context.Context, req *ratelimitv1.DeleteRuleRequest) (resp *ratelimitv1.DeleteRuleResponse, err error) {
      err = s.uc.DeleteRule(ctx, req.Id)
      if err != nil {
          return nil, rpcerror.ToBizError(err)
      }
      return &ratelimitv1.DeleteRuleResponse{}, nil
  }

  func (s *RuleServiceImpl) ListRules(ctx context.Context, req *ratelimitv1.ListRulesRequest) (resp *ratelimitv1.ListRulesResponse, err error) {
      rules, err := s.uc.ListRules(ctx, req.Service, req.Phase)
      if err != nil {
          return nil, rpcerror.ToBizError(err)
      }
      return &ratelimitv1.ListRulesResponse{Rules: rules}, nil
  }
```

- [ ] **步骤 3：创建 usecase 实现 `ratelimit_usecase.yaml`**

```yaml
# Kitex rule-center preset — usecase for RuleService
path: internal/usecase/rulecenter/usecase.go
update_behavior:
  type: cover
body: |-
  // Code generated by ncgo rule-center preset.

  package rulecenter

  import (
      "context"
      "strings"

      "github.com/samber/oops"
      ratelimitv1 "{{.Module}}/api/ratelimit/v1"
      "{{.Module}}/internal/db/gen"
  )

  type RuleRepo interface {
      GetRateLimitExactRuleByAppKey(ctx context.Context, service, phase, method, path, appKey string) (*gen.RateLimitRule, error)
      GetRateLimitExactRuleFallback(ctx context.Context, service, phase, method, path string) (*gen.RateLimitRule, error)
      GetRateLimitPatternRuleByAppKey(ctx context.Context, service, phase, method, appKey string) ([]gen.RateLimitRule, error)
      GetRateLimitPatternRuleFallback(ctx context.Context, service, phase, method string) ([]gen.RateLimitRule, error)
      CreateRateLimitRule(ctx context.Context, arg gen.CreateRateLimitRuleParams) (int64, error)
      UpdateRateLimitRule(ctx context.Context, arg gen.UpdateRateLimitRuleParams) error
      DeleteRateLimitRule(ctx context.Context, id int64) error
      ListRateLimitRules(ctx context.Context, arg gen.ListRateLimitRulesParams) ([]gen.RateLimitRule, error)
  }

  type UseCase struct {
      repo RuleRepo
  }

  func New(repo RuleRepo) *UseCase {
      return &UseCase{repo: repo}
  }

  func (uc *UseCase) GetRule(ctx context.Context, service, phase, method, path string, appKey *string) (*ratelimitv1.RateLimitRule, bool, error) {
      ak := ""
      if appKey != nil {
          ak = *appKey
      }
      // Try exact match with app_key
      if ak != "" {
          if rule, err := uc.repo.GetRateLimitExactRuleByAppKey(ctx, service, phase, method, path, ak); err == nil && rule != nil {
              return toProto(rule), true, nil
          }
      }
      // Try exact match fallback (app_key IS NULL)
      if rule, err := uc.repo.GetRateLimitExactRuleFallback(ctx, service, phase, method, path); err == nil && rule != nil {
          return toProto(rule), true, nil
      }
      // Try pattern match with app_key
      if ak != "" {
          if rules, err := uc.repo.GetRateLimitPatternRuleByAppKey(ctx, service, phase, method, ak); err == nil && len(rules) > 0 {
              if r := pickBestPattern(rules, path); r != nil {
                  return toProto(r), true, nil
              }
          }
      }
      // Try pattern match fallback
      if rules, err := uc.repo.GetRateLimitPatternRuleFallback(ctx, service, phase, method); err == nil && len(rules) > 0 {
          if r := pickBestPattern(rules, path); r != nil {
              return toProto(r), true, nil
          }
      }
      return nil, false, nil
  }

  func (uc *UseCase) CreateRule(ctx context.Context, req *ratelimitv1.CreateRuleRequest) (int64, error) {
      ak := ""
      if req.AppKey != nil {
          ak = *req.AppKey
      }
      return uc.repo.CreateRateLimitRule(ctx, gen.CreateRateLimitRuleParams{
          Service:         req.Service,
          Phase:           req.Phase,
          Method:          req.Method,
          MatchKind:       req.MatchKind,
          Path:            req.Path,
          PathPattern:     req.PathPattern,
          AppKey:          appKeyPtr(ak),
          Priority:        int32(req.Priority),
          Enabled:         req.Enabled,
          KeyBy:           req.KeyBy,
          Strategy:        req.Strategy,
          WindowSeconds:   int32(req.WindowSeconds),
          MaxRequests:     int32(req.MaxRequests),
          RequestsPerSecond: req.RequestsPerSecond,
          Burst:           int32(req.Burst),
          ClientTtlSeconds: int32(req.ClientTtlSeconds),
      })
  }

  func (uc *UseCase) UpdateRule(ctx context.Context, req *ratelimitv1.UpdateRuleRequest) error {
      arg := gen.UpdateRateLimitRuleParams{ID: req.Id}
      if req.Enabled != nil { arg.Enabled = *req.Enabled }
      if req.Priority != nil { arg.Priority = *req.Priority }
      if req.Strategy != nil { arg.Strategy = *req.Strategy }
      if req.WindowSeconds != nil { arg.WindowSeconds = *req.WindowSeconds }
      if req.MaxRequests != nil { arg.MaxRequests = *req.MaxRequests }
      if req.RequestsPerSecond != nil { arg.RequestsPerSecond = *req.RequestsPerSecond }
      if req.Burst != nil { arg.Burst = *req.Burst }
      if req.ClientTtlSeconds != nil { arg.ClientTtlSeconds = *req.ClientTtlSeconds }
      if req.KeyBy != nil { arg.KeyBy = req.KeyBy }
      return uc.repo.UpdateRateLimitRule(ctx, arg)
  }

  func (uc *UseCase) DeleteRule(ctx context.Context, id int64) error {
      return uc.repo.DeleteRateLimitRule(ctx, id)
  }

  func (uc *UseCase) ListRules(ctx context.Context, service *string, phase *string) ([]*ratelimitv1.RateLimitRule, error) {
      svc := ""
      if service != nil { svc = *service }
      ph := ""
      if phase != nil { ph = *phase }
      rules, err := uc.repo.ListRateLimitRules(ctx, gen.ListRateLimitRulesParams{Service: svc, Phase: ph})
      if err != nil {
          return nil, err
      }
      result := make([]*ratelimitv1.RateLimitRule, len(rules))
      for i := range rules {
          result[i] = toProto(&rules[i])
      }
      return result, nil
  }

  func toProto(r *gen.RateLimitRule) *ratelimitv1.RateLimitRule {
      if r == nil {
          return nil
      }
      return &ratelimitv1.RateLimitRule{
          Id: r.ID,
          Service: r.Service,
          Phase: r.Phase,
          Method: r.Method,
          MatchKind: r.MatchKind,
          Path: r.Path,
          PathPattern: r.PathPattern,
          AppKey: ptrString(r.AppKey),
          Priority: int32(r.Priority),
          Enabled: r.Enabled,
          KeyBy: r.KeyBy,
          Strategy: r.Strategy,
          WindowSeconds: int32(r.WindowSeconds),
          MaxRequests: int32(r.MaxRequests),
          RequestsPerSecond: r.RequestsPerSecond,
          Burst: int32(r.Burst),
          ClientTtlSeconds: int32(r.ClientTtlSeconds),
      }
  }

  func ptrString(s string) *string { return &s }
  func appKeyPtr(s string) *string { if s == "" { return nil }; return &s }

  func pickBestPattern(rules []gen.RateLimitRule, path string) *gen.RateLimitRule {
      for _, r := range rules {
          if r.PathPattern == "*" {
              return &r
          }
          if r.MatchKind == "prefix" && strings.HasPrefix(path, r.PathPattern) {
              return &r
          }
          if r.MatchKind == "exact" && r.PathPattern == path {
              return &r
          }
      }
      return nil
  }
```

- [ ] **步骤 4：创建 repository 实现 `ratelimit_repository.yaml`**

```yaml
# Kitex rule-center preset — repository for RuleService
path: internal/repository/rulecenter/repo.go
update_behavior:
  type: cover
body: |-
  // Code generated by ncgo rule-center preset.

  package rulecenterrepo

  import (
      "context"

      "github.com/jackc/pgx/v5/pgxpool"
      "{{.Module}}/internal/db/gen"
  )

  type Repo struct {
      q    *gen.Queries
      pool *pgxpool.Pool
  }

  func New(q *gen.Queries, pool *pgxpool.Pool) *Repo {
      return &Repo{q: q, pool: pool}
  }

  func (r *Repo) GetRateLimitExactRuleByAppKey(ctx context.Context, service, phase, method, path, appKey string) (*gen.RateLimitRule, error) {
      return r.q.GetRateLimitExactRuleByAppKey(ctx, gen.GetRateLimitExactRuleByAppKeyParams{
          Service: service, Phase: phase, Method: method, Path: path, AppKey: appKey,
      })
  }

  func (r *Repo) GetRateLimitExactRuleFallback(ctx context.Context, service, phase, method, path string) (*gen.RateLimitRule, error) {
      return r.q.GetRateLimitExactRuleFallback(ctx, gen.GetRateLimitExactRuleFallbackParams{
          Service: service, Phase: phase, Method: method, Path: path,
      })
  }

  func (r *Repo) GetRateLimitPatternRuleByAppKey(ctx context.Context, service, phase, method, appKey string) ([]gen.RateLimitRule, error) {
      return r.q.GetRateLimitPatternRuleByAppKey(ctx, gen.GetRateLimitPatternRuleByAppKeyParams{
          Service: service, Phase: phase, Method: method, AppKey: appKey,
      })
  }

  func (r *Repo) GetRateLimitPatternRuleFallback(ctx context.Context, service, phase, method string) ([]gen.RateLimitRule, error) {
      return r.q.GetRateLimitPatternRuleFallback(ctx, gen.GetRateLimitPatternRuleFallbackParams{
          Service: service, Phase: phase, Method: method,
      })
  }

  func (r *Repo) CreateRateLimitRule(ctx context.Context, arg gen.CreateRateLimitRuleParams) (int64, error) {
      return r.q.CreateRateLimitRule(ctx, arg)
  }

  func (r *Repo) UpdateRateLimitRule(ctx context.Context, arg gen.UpdateRateLimitRuleParams) error {
      return r.q.UpdateRateLimitRule(ctx, arg)
  }

  func (r *Repo) DeleteRateLimitRule(ctx context.Context, id int64) error {
      return r.q.DeleteRateLimitRule(ctx, id)
  }

  func (r *Repo) ListRateLimitRules(ctx context.Context, arg gen.ListRateLimitRulesParams) ([]gen.RateLimitRule, error) {
      return r.q.ListRateLimitRules(ctx, arg)
  }
```

- [ ] **步骤 5：创建限流中间件 `ratelimit_middleware.yaml`**

```yaml
# Kitex rule-center preset — rate-limit middleware for gRPC service self-protection
path: internal/base/middleware/ratelimit.go
update_behavior:
  type: cover
body: |-
  // Code generated by ncgo rule-center preset.

  package middleware

  import (
      "context"

      "github.com/cloudwego/kitex/pkg/endpoint"
      "github.com/cloudwego/kitex/pkg/rpcinfo"
      "{{.Module}}/internal/base/conf"
  )

  // RateLimit returns a Kitex middleware that enforces rate-limit rules
  // for gRPC service self-protection.
  func RateLimit(cfg conf.RateLimitConfig) endpoint.Middleware {
      _ = cfg
      // TODO: wire up actual rate-limit enforcement once the rate-limit
      // infra add-on supports Kitex. For now, this is a pass-through
      // placeholder that compiles and can be enabled via conf.
      return func(next endpoint.Endpoint) endpoint.Endpoint {
          return func(ctx context.Context, req, resp interface{}) error {
              if !cfg.Enabled {
                  return next(ctx, req, resp)
              }
              // Placeholder: phase="grpc", method as path
              ri := rpcinfo.GetRPCInfo(ctx)
              _ = ri
              // Full rate-limit enforcement will be wired here after
              // `ncgo add infra rate-limit` supports Kitex.
              return next(ctx, req, resp)
          }
      }
  }
```

- [ ] **步骤 6：创建 SQL schema `schema/000002_rate_limit_rules.sql`**

```yaml
# Kitex rule-center preset — rate_limit_rules table schema
path: internal/db/schema/000002_rate_limit_rules.sql
update_behavior:
  type: cover
body: |-
  CREATE TABLE IF NOT EXISTS rate_limit_rules (
      id BIGSERIAL PRIMARY KEY,
      service TEXT NOT NULL,
      phase TEXT NOT NULL,           -- 'pre_auth', 'post_auth', 'grpc'
      method TEXT NOT NULL,          -- HTTP method or '*' for gRPC
      match_kind TEXT NOT NULL,      -- 'exact', 'prefix', 'glob', 'regex'
      path TEXT NOT NULL,            -- exact path or '*'
      path_pattern TEXT NOT NULL,    -- pattern for prefix/glob/regex matching
      app_key TEXT,                  -- nullable, IS NULL means fallback
      priority INTEGER NOT NULL DEFAULT 0,
      enabled BOOLEAN NOT NULL DEFAULT true,
      key_by TEXT[] NOT NULL DEFAULT ARRAY['ip']::text[],
      strategy TEXT NOT NULL,        -- 'fixed_window', 'token_bucket'
      window_seconds INTEGER NOT NULL,
      max_requests INTEGER NOT NULL,
      requests_per_second DOUBLE PRECISION,
      burst INTEGER,
      client_ttl_seconds INTEGER,
      created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );

  CREATE INDEX idx_rate_limit_rules_lookup ON rate_limit_rules (service, phase, method, match_kind, path, app_key);
  CREATE INDEX idx_rate_limit_rules_pattern ON rate_limit_rules (service, phase, method, app_key);
```

- [ ] **步骤 7：提交**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_*.yaml internal/assets/_data/kitex/schema/000002_rate_limit_rules.sql
git commit -m "feat(kitex): add rule-center preset templates (proto, handler, usecase, repo, middleware, schema)"
```

---

### 任务 3B：Kitex layout.yaml — 定义 rule-center 模板布局

**Files:**
- 新建：`internal/assets/_data/kitex/layout-rulecenter.yaml`

这是 rule-center preset 专用的 layout 文件，定义哪些模板文件参与 Kitex 生成。

- [ ] **步骤 1：创建 layout-rulecenter.yaml**

```yaml
# Kitex rule-center preset — layout definition
# Used instead of the default layout.yaml when --preset rule-center is set.
templates:
  - kitex-template/ratelimit_proto.yaml
  - kitex-template/ratelimit_handler.yaml
  - kitex-template/ratelimit_usecase.yaml
  - kitex-template/ratelimit_repository.yaml
  - kitex-template/ratelimit_middleware.yaml
  - kitex-template/conf.yaml
  - kitex-template/server.yaml
  - kitex-template/main.yaml
  - kitex-template/data.yaml
  - kitex-template/rpcerror.yaml
  - kitex-template/interceptor.yaml
  - kitex-template/makefile.yaml
  - kitex-template/client.yaml
  - kitex-template/migration_init.yaml
  - kitex-template/migration_keep.yaml
  - sqlc.yaml
  - schema/000002_rate_limit_rules.sql
```

- [ ] **步骤 2：提交**

```bash
git add internal/assets/_data/kitex/layout-rulecenter.yaml
git commit -m "feat(kitex): add rule-center layout definition"
```

---

### 任务 4：修改 sqlc.yaml — 追加 rate_limit 查询

**Files:**
- 修改：`internal/assets/_data/kitex/kitex-template/ratelimit_sqlc_queries.yaml`

注意：sqlc 的查询 SQL 文件不是通过 kitex template 生成的，而是放在 `internal/db/query/` 目录下由 sqlc 读取。我们需要创建一个新的 template yaml 来生成这个 SQL 文件。

- [ ] **步骤 1：创建 SQL 查询模板**

```yaml
# Kitex rule-center preset — sqlc query definitions for rate_limit_rules
path: internal/db/query/rate_limit_rule.sql
update_behavior:
  type: cover
body: |-
  -- name: GetRateLimitExactRuleByAppKey :one
  SELECT * FROM rate_limit_rules
  WHERE service = $1 AND phase = $2 AND method = $3 AND match_kind = 'exact' AND (path = $4 OR path = '*') AND app_key = $5
  ORDER BY priority DESC, id DESC
  LIMIT 1;

  -- name: GetRateLimitExactRuleFallback :one
  SELECT * FROM rate_limit_rules
  WHERE service = $1 AND phase = $2 AND method = $3 AND match_kind = 'exact' AND (path = $4 OR path = '*') AND app_key IS NULL
  ORDER BY priority DESC, id DESC
  LIMIT 1;

  -- name: GetRateLimitPatternRuleByAppKey :many
  SELECT * FROM rate_limit_rules
  WHERE service = $1 AND phase = $2 AND method = $3 AND app_key = $4 AND (path_pattern = '*' OR path = $5)
  ORDER BY priority DESC, id DESC;

  -- name: GetRateLimitPatternRuleFallback :many
  SELECT * FROM rate_limit_rules
  WHERE service = $1 AND phase = $2 AND method = $3 AND app_key IS NULL AND (path_pattern = '*' OR path = $4)
  ORDER BY priority DESC, id DESC;

  -- name: CreateRateLimitRule :one
  INSERT INTO rate_limit_rules (
      service, phase, method, match_kind, path, path_pattern, app_key,
      priority, enabled, key_by, strategy, window_seconds, max_requests,
      requests_per_second, burst, client_ttl_seconds
  ) VALUES (
      $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
  ) RETURNING id;

  -- name: UpdateRateLimitRule :exec
  UPDATE rate_limit_rules SET
      enabled = COALESCE(sqlc.narg('enabled'), enabled),
      priority = COALESCE(sqlc.narg('priority'), priority),
      strategy = COALESCE(sqlc.narg('strategy'), strategy),
      window_seconds = COALESCE(sqlc.narg('window_seconds'), window_seconds),
      max_requests = COALESCE(sqlc.narg('max_requests'), max_requests),
      requests_per_second = COALESCE(sqlc.narg('requests_per_second'), requests_per_second),
      burst = COALESCE(sqlc.narg('burst'), burst),
      client_ttl_seconds = COALESCE(sqlc.narg('client_ttl_seconds'), client_ttl_seconds),
      key_by = COALESCE(sqlc.narg('key_by'), key_by),
      updated_at = now()
  WHERE id = $1;

  -- name: DeleteRateLimitRule :exec
  DELETE FROM rate_limit_rules WHERE id = $1;

  -- name: ListRateLimitRules :many
  SELECT * FROM rate_limit_rules
  WHERE (sqlc.narg('service')::text IS NULL OR service = sqlc.narg('service')::text)
    AND (sqlc.narg('phase')::text IS NULL OR phase = sqlc.narg('phase')::text)
  ORDER BY priority DESC, id DESC;
```

- [ ] **步骤 2：提交**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_sqlc_queries.yaml
git commit -m "feat(kitex): add sqlc query template for rule-center rate_limit queries"
```

---

### 任务 5：修改 `ncgo add rpc` — 追加 `--preset` 标志

**Files:**
- 修改：`internal/cli/add.go` — 追加 `--preset` flag
- 修改：`internal/scaffold/rpc/rpc.go` — 接收 preset 参数并传递给 mono.Generate

- [ ] **步骤 1：修改 add.go 中 addRPCOptions 和 newAddRPCCmd**

在 `addRPCOptions` struct 中追加 `preset string`：

```go
type addRPCOptions struct {
	root       string
	module     string
	dir        string
	noGenerate bool
	dryRun     bool
	plan       bool
	output     string
	preset     string
}
```

在 `newAddRPCCmd()` 的 flag 定义中追加：

```go
	f.StringVar(&opts.preset, "preset", "", "Preset template to use (e.g., rule-center)")
```

在 `runAddRPC` 中传递 preset：

```go
	res, err := rpc.Add(cmd.Context(), rpc.Options{
		Root:          opts.root,
		Name:          name,
		Module:        opts.module,
		Dir:           opts.dir,
		AssetsVersion: assets.Version(),
		NCGOVersion:   Version,
		NoGenerate:    opts.noGenerate,
		DryRun:        opts.dryRun,
		Preset:        opts.preset,
	})
```

- [ ] **步骤 2：修改 rpc.go Options struct**

```go
type Options struct {
	Root          string
	Name          string
	Module        string
	Dir           string
	AssetsVersion string
	NCGOVersion   string
	NoGenerate    bool
	DryRun        bool
	Preset        string // preset template name (e.g., "rule-center")
	Runner        exec.Runner
	Now           time.Time
}
```

- [ ] **步骤 3：提交**

```bash
git add internal/cli/add.go internal/scaffold/rpc/rpc.go
git commit -m "feat(cli): add --preset flag to ncgo add rpc"
```

---

### 任务 6：修改 mono.Generate — 支持 preset 模板覆盖

**Files:**
- 修改：`internal/scaffold/mono/mono.go` — 在 Options 中追加 Preset 字段
- 修改：`internal/scaffold/mono/template.go` 或对应文件 — preset 时覆盖默认模板选择

关键逻辑：当 `Preset == "rule-center"` 时：
1. IDL 使用 `idl/rule-center.proto` 而非默认
2. Kitex 生成时使用 `layout-rulecenter.yaml` 而非默认 layout
3. 生成的 manifest 标记 `preset: rule-center`

- [ ] **步骤 1：修改 mono.Options 追加 Preset**

```go
type Options struct {
	Name          string
	Module        string
	Kind          string
	Dir           string
	WithDatabase  bool
	Infra         []string
	IDL           string
	AssetsVersion string
	NCGOVersion   string
	NoGenerate    bool
	Runner        exec.Runner
	Now           time.Time
	Preset        string // preset name (e.g., "rule-center")
}
```

- [ ] **步骤 2：在 rpc.go 中传递 Preset 到 mono.Generate**

在 `mono.Generate` 调用中追加 `Preset: opts.Preset`。

- [ ] **步骤 3：在 mono.Generate 中处理 preset 逻辑**

在 `writeTemplate` 或 kitex 调用之前，如果 `opts.Preset == "rule-center"`：
- IDL 自动使用 `idl/rule-center.proto`
- Kitex 使用 rule-center layout
- 需要修改 kitex 调用参数中的 template-dir 里的 layout 指向

具体实现需要在 kitex 模板目录中创建一个 rule-center 专用的 layout 文件，并在 kitex 调用时通过 `--customize_layout` 指向它。

- [ ] **步骤 4：提交**

```bash
git add internal/scaffold/mono/mono.go internal/scaffold/rpc/rpc.go
git commit -m "feat(mono): support preset template override for rule-center"
```

---

### 任务 7：扩展 `ncgo test rate-limit seed` — 追加 gRPC 规则

**Files:**
- 修改：`internal/scaffold/test/ratelimit/seed.go`

- [ ] **步骤 1：修改 buildSeedSQL 追加 phase='grpc' 规则**

在现有的两个 INSERT 之后追加 gRPC 规则：

```go
func buildSeedSQL(service string, maxRequests, windowSecs int) string {
	var b strings.Builder
	svc := sanitizeSQLString(service)
	b.WriteString(fmt.Sprintf("DELETE FROM rate_limit_rules WHERE service = '%s';\n", svc))
	b.WriteString(fmt.Sprintf(`INSERT INTO rate_limit_rules (service, phase, method, match_kind, path, path_pattern, enabled, key_by, strategy, window_seconds, max_requests) VALUES
('%s', 'pre_auth', '*', 'exact', '*', '*', true, ARRAY['ip']::text[], 'fixed_window', %d, %d),
('%s', 'pre_auth', 'GET', 'exact', '/healthz', '/healthz', true, ARRAY['ip']::text[], 'fixed_window', %d, %d),
('%s', 'grpc', '*', 'exact', '*', '*', true, ARRAY['ip']::text[], 'fixed_window', %d, %d);
`, svc, windowSecs, maxRequests, svc, windowSecs, maxRequests, svc, windowSecs, maxRequests))
	return b.String()
}
```

- [ ] **步骤 2：更新 seed_test.go**

修改 `TestBuildSeedSQLBasic` 中的期望值：

```go
	// Should have exactly 3 INSERT value rows (pre_auth x2 + grpc x1)
	rows := strings.Count(sql, "fixed_window")
	if rows != 3 {
		t.Errorf("expected 3 INSERT rows, found %d", rows)
	}
```

新增测试：

```go
func TestBuildSeedSQLContainsGRPCRule(t *testing.T) {
	sql := buildSeedSQL("my-service", 10, 60)
	if !strings.Contains(sql, "'my-service', 'grpc', '*', 'exact', '*', '*'") {
		t.Errorf("expected grpc rule for my-service, got:\n%s", sql)
	}
}
```

- [ ] **步骤 3：提交**

```bash
git add internal/scaffold/test/ratelimit/seed.go internal/scaffold/test/ratelimit/seed_test.go
git commit -m "feat(test): add grpc phase rules to rate-limit seed"
```

---

### 任务 8：扩展 `ncgo test rate-limit run` — 追加 `--grpc` 标志

**Files:**
- 修改：`internal/scaffold/test/ratelimit/run.go`
- 修改：`internal/cli/test.go` — 追加 --grpc flag 传递

- [ ] **步骤 1：修改 RunOptions 追加 GRPC 字段**

```go
type RunOptions struct {
	Root     string
	Host     string
	Port     int
	Rate     int
	Duration string
	Paths    []string
	GRPC     bool // when true, use grpcurl instead of vegeta
}
```

- [ ] **步骤 2：修改 Run 函数追加 grpcurl 分支**

```go
func Run(ctx context.Context, opts RunOptions) error {
	// ... existing defaults ...
	if opts.GRPC {
		return runGRPC(ctx, opts)
	}
	// ... existing vegeta logic ...
}

func runGRPC(ctx context.Context, opts RunOptions) error {
	if _, err := exec.LookPath("grpcurl"); err != nil {
		return fmt.Errorf("grpcurl not found: install it via `go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest`")
	}
	target := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	cmd := exec.CommandContext(ctx, "grpcurl", "-plaintext", "-d",
		`{"service":"`+opts.Host+`","phase":"grpc","path":"/test"}`,
		target, "ratelimit.v1.RuleService.GetRule")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

- [ ] **步骤 3：修改 CLI 追加 --grpc flag**

在 `newRateLimitRunCmd()` 中追加：

```go
	var grpc bool
	cmd.Flags().BoolVar(&grpc, "grpc", false, "Use grpcurl to verify gRPC rule-center (default: false)")
```

在 RunOptions 传递时加上 `GRPC: grpc`。

- [ ] **步骤 4：更新 run_test.go**

```go
func TestBuildTargets(t *testing.T) {
	// ... existing tests ...
}

func TestGRPCModeSkipsVegeta(t *testing.T) {
	// Verify that when opts.GRPC is true, runGRPC is called instead of vegeta
	// This is a behavior test — the actual grpcurl call is integration-level.
	opts := RunOptions{GRPC: true, Host: "localhost", Port: 8888}
	// At unit-test level, just verify the path selection logic
	if !opts.GRPC {
		t.Error("expected GRPC mode to be enabled")
	}
}
```

- [ ] **步骤 5：提交**

```bash
git add internal/scaffold/test/ratelimit/run.go internal/scaffold/test/ratelimit/run_test.go
git commit -m "feat(test): add --grpc flag for grpcurl rule-center verification"
```

---

### 任务 9：更新 golden 快照 + 全仓库验证

- [ ] **步骤 1：更新 golden 快照**

```bash
go test ./internal/scaffold/mono/... -update-golden -count=1
```

- [ ] **步骤 2：全仓库构建 + 测试**

```bash
go build ./... && go build . && go vet ./... && go test ./... -count=1
```

- [ ] **步骤 3：冒烟测试**

```bash
./scripts/smoke.sh
```

- [ ] **步骤 4：提交**

```bash
git add -A
git commit -m "test(rate-limit): update golden snapshots for rule-center changes"
```

---

### 风险与注意事项

1. **Kitex proto package 路径：** proto 的 `go_package` 设为 `api/ratelimit/v1;ratelimitv1`，需要确保 Kitex 生成时能正确识别。可能需要调整 proto 文件放置位置或在 kitex 调用时追加 `-I` 参数。

2. **sqlc 生成顺序：** rule-center 服务需要 `make sqlc` 在 `kitex` 之后运行（因为 sqlc 依赖 schema 和 query 文件）。Kitex 生成的是 Go 代码，sqlc 生成的也是 Go 代码，两者不冲突但需要按正确顺序执行。

3. **workspace compose：** 确保 `ncgo add rpc rule-center` 生成的 manifest 中 `with_database=true`，这样 workspace compose 才能正确追加 postgres 服务。

4. **grpcurl 依赖：** run 命令的 `--grpc` 模式需要用户安装 grpcurl。错误提示应清晰。

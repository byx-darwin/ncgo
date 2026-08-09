# Final Whole-Branch Review: Kitex RPC 服务限流拦截 (Issue #30, PR #31)

**Reviewer:** opus whole-branch review · **Range:** main(42572ef)..feat/30-kitex-ratelimit(fa0a1a1) · 18 commits
**Verdict:** **BLOCK** → 2 Critical + 3 Important(修复后复评)

## Critical (Must Fix Before Merge)

1. **`add infra rate-limit --wire` 在真实生成项目上产出不可编译的 server.go**
   - wire `exists` 哨兵被模板注释短路:`insertAfterMarkerOrAnyTracked`(wire.go:534-539)在 `strings.Contains(src, "middleware.RateLimit(")` 时短路 —— 但生成的 server.go 含注释 `// middleware.RateLimit(cfg.RateLimit),`(server.yaml:102)→ 中间件调用插入被跳过
   - `addGoImportTracked`(wire.go:461-462)同样因注释 `// import "..."`(server.yaml:101)含引号路径而跳过真实 import
   - static-limit 插入照常执行(哨兵不在注释中)→ server.go 引用未定义 `middleware` 包 → 编译错误
   - infra 测试漏网:fixture `writeKitexServerWithRateLimitMarkers`(infra_test.go:1301-1339)省略了真实模板的注释行
   - **修复**:exists 哨兵改为匹配真实插入代码(如 `"\t\t\tmiddleware.RateLimit(cfg.RateLimit),\n"`);import 检查跳过注释行;fixture 补注释行

2. **`add infra rate-limit` 默认 conf 块被 kitex `conf.Validate()` 拒绝(启动失败)**
   - `defaultRateLimitConfBlock()`(infra.go:732-746)写 `source.type: config`,但 kitex conf.Validate 仅接受 `database|grpc`(conf.yaml:312-313),`Load()` 启动时执行 → "rate_limit.source.type must be database or grpc"
   - 设计的默认值(config)与 rule_center 路径都被拒
   - **修复**:kitex Validate 白名单扩展为 `config|database|rule_center|grpc`(镜像 hertz conf_go.yaml:765-768)

## Important (Should Fix Before Merge)

3. **e2e RPC attacker 无法从 grpcurl 输出识别 10429**
   - parseBizCode(attack_grpc.go:23-39)找 `"code": N`;但 Kitex gRPC 传输将 BizStatusError 映射为 gRPC code 13 + 消息 `biz error: code=10429, msg=rate limited`(biz code 在 kitex 专有 biz-status trailer,grpcurl 不输出)
   - 解析器返回 13(JSON 路径)或 0(文本路径)→ 所有拒绝归入 StatusOther → 即使限流生效也判 FAIL
   - 单测用合成 `{"code":10429}`,与真实线上格式不符
   - **修复**:消息中匹配 `code[=:]\s*(\d+)`(或 grpcurl -v trailers / kitex 生成客户端);单测改用真实线上格式样本

4. **`add rule-center`(kitex)写入不可解析的 Duration**
   - 复用 hertz 的 `updateConfForRuleCenter`(rulecenter.go:144)插入 `query_timeout_milliseconds: 200` —— 裸整数,kitex `config.Duration.UnmarshalYAML` 报 `time: missing unit in duration "200"` → conf.Load 失败(叠加 Critical #2 的 Validate 拒绝)
   - **修复**:kitex 路径写 `200ms` 带单位形式(或同时修正 hertz 既有裸整数行为)

5. **`mergeKitexRateLimitConfig` 误改嵌套键**
   - 合并循环(infra.go:786-800)翻转 rate_limit 作用域内任意缩进的 `enabled:`/`mode:` 行 → `pre_auth.enabled: false` 会被强制改 true
   - **修复**:按块深度追踪或精确匹配 2 空格缩进的顶层键

## Minor(Follow-up,均不阻塞)

- 24 条累积 Minors 全部可作后续:T10 conf-merge 已升级为 Important #5;T7 retryAfter 零值已解决;T12 四项与 Important #3 叠加;T8 go-redis 非问题(store.go 本身需要)
- 新 Minor:hertz 中间件未读 cfg.Mode(设计 §6.3 声称 hertz 获得 shadow 能力,未实现,无回归);ruleCenterOnce 首次空调用永久禁用客户端

## 干净面(保留)

共享片段管线三路径逐字节一致 ✅ · 拒绝契约(10429 + BizExtra)全链路一致 ✅ · 中间件 shadow/enforce/fail_open 语义正确、sync.Once 无竞态 ✅ · golden 卫生 ✅ · dry-run 安全 + astwire 幂等 ✅

## Merge Recommendation

**BLOCK** —— 修复 2 Critical + 3 Important,对真实路径做实证验证后复评。

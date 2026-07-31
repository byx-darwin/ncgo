# Kitex RPC 服务限流拦截 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 ncgo 生成的所有 Kitex RPC 服务经 `ncgo add infra rate-limit` 获得真实限流拦截(双轨:静态 WithLimit 兜底 + 动态 chain 中间件,默认 shadow 模式),复用 Hertz 限流基建,并放开 `add rule-center` 对 kitex 的支持。

**Architecture:** 从 `hertz/layout.yaml` 抽取框架无关的 resolver/store/rule-center client 为共享 asset 片段(`internal/assets/_data/ratelimit/*.yaml`);hertz 经 include 指令在 scaffold 时缝合(monkey-free,hz 无感知),kitex 经 layout 模板列表直接引用;kitex 中间件模板重写为真实 `endpoint.Middleware`,超限返回 BizStatusError 10429;infra 命令经 astwire 激活 server.go 两个标记完成接线。

**Tech Stack:** Go 1.26.5 · Kitex(endpoint.Middleware / rpcinfo / kerrors / transmeta)· astwire(go/ast 接线)· yaml.v3 · go-redis Lua · expvar · grpcurl(e2e 压测)· golden 测试

**Spec:** `docs/superpowers/specs/2026-07-31-kitex-ratelimit-design.md` · **Issue:** #30 · **Workflow:** wf-2026-07-30-001

## Global Constraints

- Go 版本锁定 `go 1.26.5`(kitex 模板 go.mod 已锁定),不引入新的外部依赖(shadow 计数用标准库 `expvar`)
- 纯搬移任务(Task 2/3/5)必须保证 hertz 生成产物**逐字节不变**(golden 不允许 re-record)
- 行为变化任务(Task 4/6/8/12)才可 `-update-golden` 重录,且必须在 commit 中人工审查 diff
- 模板占位符规范:共享片段一律用 `{{.Module}}`(kitex 原生);hertz 缝合时转换为 `{{.GoModule}}`
- commit 前缀遵循 conventional commits(`feat:`/`test:`/`refactor:`/`docs:`)
- 每个任务结束运行 `gofmt -l .` 必须为空、`go build ./...` 必须成功
- 验收码:kitex 限流拒绝 biz code = **10429**;metainfo retry-after key = **`rl-retry-after`**
- 默认值:kitex `mode: shadow`、hertz `mode: enforce`(现状不变)、`static` 全零(不挂载 WithLimit)

---

## File Structure

### ncgo 仓库改动

| 文件 | 责任 |
|---|---|
| `internal/assets/_data/ratelimit/resolver.yaml` ★新 | 共享片段:规则解析器(Lookup/Resolver/各 source/缓存/匹配/Normalize) |
| `internal/assets/_data/ratelimit/resolver_test.yaml` ★新 | 共享片段:解析器测试 |
| `internal/assets/_data/ratelimit/store.yaml` ★新 | 共享片段:计数器(memory/redis)+ `NewStore` + `BuildKey` |
| `internal/assets/_data/ratelimit/store_test.yaml` ★新 | 共享片段:计数器测试(含 BuildKey 等价性) |
| `internal/assets/_data/ratelimit/rule_center_client.yaml` ★新 | 共享片段:rule-center gRPC 客户端 |
| `internal/scaffold/mono/files.go` | include 指令展开(`expandIncludes`)+ rule-center client 读取改共享源 |
| `internal/scaffold/mono/files_test.go` ★新 | expandIncludes 单测(含占位符转换、缩进、多片段) |
| `internal/scaffold/mono/shared_fragments_test.go` ★新 | 共享片段可解析 + 关键符号存在 |
| `internal/assets/_data/hertz/layout.yaml` | 内联 resolver/test/store 换 include 指令;middleware 瘦身;conf 增 Mode/Static |
| `internal/assets/_data/hertz/optional/rule_center_client.go` 删 | 由共享片段取代 |
| `internal/assets/_data/kitex/kitex-template/conf.yaml` | RateLimitConfig 增 Mode/Static/RuleCenter |
| `internal/assets/_data/kitex/kitex-template/rpcerror.yaml` + `rpcerror_test.yaml` | `RateLimited()` 10429 构造器 |
| `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml` | 占位符 → 真实中间件 + StaticLimitOption |
| `internal/assets/_data/kitex/kitex-template/ratelimit_middleware_test.yaml` ★新 | 中间件单测模板(enforce/shadow/fail_open) |
| `internal/assets/_data/kitex/kitex-template/server.yaml` | 新增 `ncgo:wire:ratelimit:static-limit` 标记 |
| `internal/assets/_data/kitex/layout-rulecenter.yaml` | templates 列表引用共享片段 + 中间件测试 |
| `internal/scaffold/infra/infra.go` | `KindRateLimit`(kitex-only)+ 多文件 assetFiles + kitex conf 合并 |
| `internal/scaffold/infra/wire.go` | `wireKitex` rate-limit case(两个标记)+ marker 常量 |
| `internal/scaffold/infra/infra_test.go` | 新 kind 全路径测试 |
| `internal/scaffold/rulecenter/rulecenter.go` | 放开 kitex(kind 校验 + 跳过 wire) |
| `internal/scaffold/rulecenter/rulecenter_test.go` | `TestAddRejectsKitex` → `TestAddAcceptsKitex` |
| `internal/scaffold/test/ratelimit/attack_grpc.go` ★新 | RPC 压测客户端(grpcurl worker pool)+ 输出解析 |
| `internal/scaffold/test/ratelimit/e2e.go` | kitex 分支(TCP readiness + RPC attacker) |
| `internal/scaffold/test/ratelimit/attack_grpc_test.go` ★新 | 解析器单测 |
| golden: `internal/scaffold/mono/testdata/`、`internal/scaffold/rpc/testdata/`、`internal/scaffold/infra/testdata/` | 按任务重录/新增 |
| `internal/assets/_data/docs/hertz/rate-limit-dynamic-design.zh-CN.md` + `.en.md` | Kitex 章节 |

---

## Task 1: include 指令展开器(hertz 缝合支持)

**Files:**
- Modify: `internal/scaffold/mono/files.go`(writeHertzTemplate 的 layout.yaml 读取处,约 :100-109)
- Create: `internal/scaffold/mono/files_test.go`

**Interfaces:**
- Produces: `expandIncludes(layout []byte, srcFS fs.FS) ([]byte, error)` —— 将形如 `  # {{include: ratelimit/resolver}}` 的行替换为对应共享片段渲染出的 layout 条目;片段占位符 `{{.Module}}` 转 `{{.GoModule}}`;body 行统一 6 空格缩进

- [ ] **Step 1: 写失败测试**

`internal/scaffold/mono/files_test.go`:

```go
package mono

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestExpandIncludes(t *testing.T) {
	fragment := "# shared\npath: internal/pkg/ratelimit/resolver.go\nupdate_behavior:\n  type: cover\nbody: |-\n  package ratelimit\n\n  // module {{.Module}}/internal/base/conf\n"
	fsys := fstest.MapFS{
		"ratelimit/resolver.yaml": &fstest.MapFile{Data: []byte(fragment)},
	}
	layout := "layouts:\n  - path: internal/handler/\n    delims: [\"\", \"\"]\n    body: \"\"\n  # {{include: ratelimit/resolver}}\n  - path: internal/usecase/\n    delims: [\"\", \"\"]\n    body: \"\"\n"

	out, err := expandIncludes([]byte(layout), fsys)
	if err != nil {
		t.Fatalf("expandIncludes: %v", err)
	}
	got := string(out)

	wantEntry := "  - path: internal/pkg/ratelimit/resolver.go\n    delims: [\"{{\", \"}}\"]\n    body: |-\n      package ratelimit\n\n      // module {{.GoModule}}/internal/base/conf\n"
	if !strings.Contains(got, wantEntry) {
		t.Errorf("expanded entry mismatch\ngot:\n%s\nwant substring:\n%s", got, wantEntry)
	}
	if strings.Contains(got, "{{include:") {
		t.Errorf("directive not consumed:\n%s", got)
	}
	if !strings.Contains(got, "  - path: internal/usecase/") {
		t.Errorf("following entries lost:\n%s", got)
	}
}

func TestExpandIncludesMissingFragment(t *testing.T) {
	fsys := fstest.MapFS{}
	_, err := expandIncludes([]byte("layouts:\n  # {{include: ratelimit/missing}}\n"), fsys)
	if err == nil || !strings.Contains(err.Error(), "ratelimit/missing") {
		t.Fatalf("want missing-fragment error, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/scaffold/mono/ -run TestExpandIncludes -v`
Expected: FAIL — `expandIncludes` 未定义

- [ ] **Step 3: 实现 expandIncludes 并接入 writeHertzTemplate**

在 `internal/scaffold/mono/files.go` 添加(imports 增 `io/fs`、`regexp`、`gopkg.in/yaml.v3`):

```go
var includeDirectiveRE = regexp.MustCompile(`^(\s*)#\s*\{\{include:\s*([A-Za-z0-9_/.-]+)\}\}\s*$`)

// expandIncludes replaces "# {{include: <asset>}}" directive lines in a hertz
// layout.yaml with the referenced shared fragment rendered as a layout entry.
// Fragments use the canonical kitex template format (path/body, {{.Module}}
// placeholder, 2-space body indent); hz consumes the expanded result, so
// directives never leave ncgo. Output is deterministic for golden tests.
func expandIncludes(layout []byte, srcFS fs.FS) ([]byte, error) {
	lines := strings.Split(string(layout), "\n")
	var out []string
	for _, line := range lines {
		m := includeDirectiveRE.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			continue
		}
		name := m[2]
		entry, err := renderSharedFragment(srcFS, name)
		if err != nil {
			return nil, err
		}
		out = append(out, entry...)
	}
	return []byte(strings.Join(out, "\n")), nil
}

func renderSharedFragment(srcFS fs.FS, name string) ([]string, error) {
	b, err := fs.ReadFile(srcFS, name+".yaml")
	if err != nil {
		return nil, fmt.Errorf("scaffold: read shared fragment %s: %w", name, err)
	}
	var frag struct {
		Path string `yaml:"path"`
		Body string `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &frag); err != nil {
		return nil, fmt.Errorf("scaffold: parse shared fragment %s: %w", name, err)
	}
	body := strings.ReplaceAll(frag.Body, "{{.Module}}", "{{.GoModule}}")
	entry := []string{
		"  - path: " + frag.Path,
		`    delims: ["{{", "}}"]`,
		"    body: |-",
	}
	for _, bl := range strings.Split(body, "\n") {
		if bl == "" {
			entry = append(entry, "")
			continue
		}
		entry = append(entry, "      "+strings.TrimPrefix(strings.TrimPrefix(bl, "  "), " "))
	}
	return entry, nil
}
```

接入点 —— `writeHertzTemplate` 中 layout.yaml 的读取循环改为:

```go
	for _, name := range []string{"layout.yaml", "package.yaml"} {
		b, err := fs.ReadFile(srcFS, "hertz/"+name)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded hertz/%s: %w", name, err)
		}
		if name == "layout.yaml" {
			b, err = expandIncludes(b, srcFS)
			if err != nil {
				return fmt.Errorf("scaffold: expand hertz/layout.yaml: %w", err)
			}
		}
		if err := os.WriteFile(filepath.Join(tplDir, name), b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", name, err)
		}
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/scaffold/mono/ -run TestExpandIncludes -v && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/mono/files.go internal/scaffold/mono/files_test.go
git commit -m "feat(scaffold): hertz layout include-directive expansion for shared fragments"
```

---

## Task 2: 抽取 resolver 共享片段(机械搬移)

**Files:**
- Create: `internal/assets/_data/ratelimit/resolver.yaml`
- Create: `internal/assets/_data/ratelimit/resolver_test.yaml`
- Create: `internal/scaffold/mono/shared_fragments_test.go`

**Interfaces:**
- Produces: 两个 kitex 模板格式片段(`path:`/`update_behavior:`/`body:`,body 为 2 空格缩进、占位符 `{{.Module}}`),内容 = hertz/layout.yaml 中 `internal/pkg/ratelimit/resolver.go` 与 `resolver_test.go` 两个条目的 body 逐字搬移

- [ ] **Step 1: 用脚本从 layout.yaml 生成片段(保证逐字)**

```bash
mkdir -p internal/assets/_data/ratelimit
cd internal/assets/_data

# resolver 条目:从 "- path: internal/pkg/ratelimit/resolver.go" 到下一个 "  - path:" 之前
awk '/^  - path: internal\/pkg\/ratelimit\/resolver.go$/{f=1} f&&/^  - path: internal\/pkg\/ratelimit\/resolver_test.go$/{f=0} f' hertz/layout.yaml > /tmp/resolver_entry.txt
# resolver_test 条目:从其 path 行到下一个 "  - path:" 之前
awk '/^  - path: internal\/pkg\/ratelimit\/resolver_test.go$/{f=1;print;next} f&&/^  - path: /{f=0} f' hertz/layout.yaml > /tmp/resolver_test_entry.txt

for pair in "resolver:/tmp/resolver_entry.txt:internal/pkg/ratelimit/resolver.go:rate-limit resolver (framework-agnostic)" \
            "resolver_test:/tmp/resolver_test_entry.txt:internal/pkg/ratelimit/resolver_test.go:rate-limit resolver tests"; do
  name="${pair%%:*}"; rest="${pair#*:}"; src="${rest%%:*}"; rest="${rest#*:}"; path="${rest%%:*}"; desc="${rest#*:}"
  {
    echo "# Shared ${desc}"
    echo "path: ${path}"
    echo "update_behavior:"
    echo "  type: cover"
    echo "body: |-"
    # 提取 body 行(原 6 空格缩进),降为 2 空格;{{.GoModule}} → {{.Module}}
    sed -n '/^    body: |-$/,$p' "$src" | tail -n +2 | sed 's/^      /  /' | sed 's/{{\.GoModule}}/{{.Module}}/g'
  } > "ratelimit/${name}.yaml"
done
echo "resolver.yaml: $(wc -l < ratelimit/resolver.yaml) lines; resolver_test.yaml: $(wc -l < ratelimit/resolver_test.yaml) lines"
```

Expected: resolver.yaml ≈ 490 行、resolver_test.yaml ≈ 240 行(以实际为准),且 `head -8 ratelimit/resolver.yaml` 显示 `body: |-` 后为 `  package ratelimit`

- [ ] **Step 2: 写片段可解析测试**

`internal/scaffold/mono/shared_fragments_test.go`:

```go
package mono

import (
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/assets"
)

func TestSharedRateLimitFragmentsParse(t *testing.T) {
	srcFS := assets.FS()
	for _, frag := range []struct {
		name       string
		wantPath   string
		wantSymbol string
	}{
		{"ratelimit/resolver", "internal/pkg/ratelimit/resolver.go", "func NewResolver("},
		{"ratelimit/resolver_test", "internal/pkg/ratelimit/resolver_test.go", "func TestResolver"},
	} {
		b, err := fs.ReadFile(srcFS, frag.name+".yaml")
		if err != nil {
			t.Fatalf("read %s: %v", frag.name, err)
		}
		var doc struct {
			Path string `yaml:"path"`
			Body string `yaml:"body"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			t.Fatalf("parse %s: %v", frag.name, err)
		}
		if doc.Path != frag.wantPath {
			t.Errorf("%s path = %q, want %q", frag.name, doc.Path, frag.wantPath)
		}
		if !strings.Contains(doc.Body, frag.wantSymbol) {
			t.Errorf("%s body missing %q", frag.name, frag.wantSymbol)
		}
		if strings.Contains(doc.Body, "{{.GoModule}}") {
			t.Errorf("%s body must use {{.Module}}, found {{.GoModule}}", frag.name)
		}
	}
}
```

- [ ] **Step 3: 运行测试确认通过**

Run: `go test ./internal/scaffold/mono/ -run TestSharedRateLimitFragmentsParse -v`
Expected: PASS(若 `wantSymbol` 与实际测试函数名不符,按实际测试函数名修正断言常量 —— 仅此一处允许按实况调整)

- [ ] **Step 4: Commit**

```bash
git add internal/assets/_data/ratelimit/ internal/scaffold/mono/shared_fragments_test.go
git commit -m "feat(assets): shared ratelimit resolver fragments extracted verbatim from hertz layout"
```

---

## Task 3: hertz layout 换用 include 指令(golden 逐字节锁定)

**Files:**
- Modify: `internal/assets/_data/hertz/layout.yaml`(resolver 与 resolver_test 两个内联条目)

- [ ] **Step 1: 替换内联条目为指令**

用编辑器将 layout.yaml 中从 `  - path: internal/pkg/ratelimit/resolver.go` 起、到 `resolver_test.go` 条目结束(下一个 `  - path:` 之前)的全部内容替换为两行:

```yaml
  # {{include: ratelimit/resolver}}
  # {{include: ratelimit/resolver_test}}
```

(位置不变,仍在原 resolver 条目处。)

- [ ] **Step 2: 验证缝合输出与抽取前逐字节一致**

Run: `go test ./internal/scaffold/mono/ -run 'TestGenerateGolden' -v`
Expected: **PASS 且不允许使用 -update-golden**。若失败,用 `git diff` 对照 testdata 找出字节差异(通常是缩进/空行),修正 Task 1 的 `renderSharedFragment` 或 Task 2 的抽取脚本后重新生成片段,直到通过。

- [ ] **Step 3: 全量构建 + hertz 相关测试**

Run: `go build ./... && go test ./internal/scaffold/... ./internal/cli/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/assets/_data/hertz/layout.yaml
git commit -m "refactor(assets): hertz layout consumes shared resolver fragments via include (byte-identical)"
```

---

## Task 4: 抽取 store 共享片段 + BuildKey(hertz 行为保持)

**Files:**
- Create: `internal/assets/_data/ratelimit/store.yaml`、`store_test.yaml`
- Modify: `internal/assets/_data/hertz/layout.yaml`(rate_limit.go 条目瘦身 + include store;rate_limit_test.go 同理;conf 条目留待 Task 6)

**Interfaces:**
- Produces(共享包新 API,生成于 `internal/pkg/ratelimit/store.go`):
  - `type Store interface { Allow(ctx context.Context, key string, rule conf.RateLimitRuleConfig) (bool, error) }`
  - `func NewStore(cfg conf.RateLimitConfig, redisClient redis.UniversalClient) Store` —— backend=redis 且 client!=nil → redis store;否则 memory store(maxEntries 取 `cfg.Memory.MaxEntries`,<=0 时默认 10000)
  - `func BuildKey(lookup Lookup, keyBy []string, prefix string) string` —— 与原 hertz `rateLimitKey` 对相同维度产生**完全相同**的 key 串
- Consumes: Task 2 的 resolver 片段(`Lookup` 类型)

- [ ] **Step 1: 记录行为基准(key 格式 + redis helper 语义)**

Run: `sed -n '/func rateLimitKey(/,/^      }$/p' internal/assets/_data/hertz/layout.yaml`
Run: `grep -n "func sharedRedisClient" -A 15 internal/assets/_data/hertz/layout.yaml`
将两段输出贴入 scratch 笔记:BuildKey 必须复刻 rateLimitKey 各维度子串格式;确认 sharedRedisClient 在无 redis 配置时的返回值(返回 nil → Task Step 3 直接传 `sharedRedisClient(cfg.Redis)`;会 panic/必须配置 → 改为 `if cfg.Backend == "redis" { client = sharedRedisClient(cfg.Redis) }` 条件传参)。

- [ ] **Step 2: 生成 store 片段**

从 layout.yaml 的 `internal/pkg/middleware/rate_limit.go` 条目 body 中搬移以下符号到新片段 `ratelimit/store.yaml`(package 改为 `ratelimit`;`rateLimitStore` 改名导出 `Store`;`newRateLimitStore(cfg)` 改为 `NewStore(cfg, redisClient)` 且不再调用 `sharedRedisClient`;`memoryCacheMaxEntries(n)` 内联为 `if n <= 0 { n = 10000 }`):

搬移清单:`rateLimitStore`(→`Store`)接口、`rateBucket`、`fixedWindowCounter`、`memoryRateLimitStore` 及其 `Allow`/`allowFixedWindow` 方法、`newMemoryRateLimitStore`、`redisRateLimitStore`、`redisRateLimitScript`、`newRateLimitStore`(→`NewStore`)。

片段头部:

```yaml
# Shared rate-limit counter store (framework-agnostic)
path: internal/pkg/ratelimit/store.go
update_behavior:
  type: cover
body: |-
  package ratelimit
  ...(搬移内容;redis import 保留;新增 strings 若 BuildKey 需要)
```

新增 `BuildKey`(加入 store.yaml body,复刻 Step 1 记录的格式,输入改为 Lookup 字段):

```go
  // BuildKey renders the counter key for a resolved rule. Dimension tokens
  // mirror the legacy hertz rateLimitKey output byte-for-byte so existing
  // counters keep their identity after the extraction.
  func BuildKey(lookup Lookup, keyBy []string, prefix string) string {
      parts := make([]string, 0, len(keyBy)+1)
      if prefix != "" {
          parts = append(parts, prefix)
      }
      for _, dim := range keyBy {
          var v string
          switch dim {
          case "ip":
              v = lookup.ClientIP
          case "path", "method_path":
              v = lookup.Method + " " + lookup.Path
          case "app_key", "ak_path":
              v = lookup.AppKey + "|" + lookup.Path
          case "user", "user_uuid":
              v = lookup.UserUUID
          default:
              v = lookup.ClientIP
          }
          if v == "" {
              v = "unknown"
          }
          parts = append(parts, dim+"="+v)
      }
      return strings.Join(parts, ":")
  }
```

> ⚠️ switch 分支的维度名与 token 格式**必须**以 Step 1 记录的原 rateLimitKey 为准;上面是示意骨架,实现时逐分支对齐原函数,并补 `store_test.yaml` 中的等价性用例(原 rate_limit_test.go 里 key 相关断言整体搬入,输入改为 Lookup)。

- [ ] **Step 3: 瘦身 hertz rate_limit.go 条目**

layout.yaml 的 `internal/pkg/middleware/rate_limit.go` body 中:
- 删除已搬移的所有符号;
- 闭包内改为:`store := ratelimit.NewStore(cfg, sharedRedisClient(cfg.Redis))`(memory 时 sharedRedisClient 的行为按现有实现;若其在无 redis 配置时 panic,则改为条件传入 nil);
- key 构建改为:`lookup := ratelimit.Lookup{Service: rateLimitService(cfg), Phase: phase, AppKey: rateLimitAppKey(c, cfg), Method: string(c.Method()), Path: rateLimitPath(c), UserUUID: rateLimitUserUUID(c), ClientIP: requestIP(c)}`(复用已构建的 lookup,resolver.Resolve 与 BuildKey 共用),`key := ratelimit.BuildKey(lookup, rule.KeyBy, cfg.KeyPrefix)`;
- 在 store 条目原位置上方加入 `  # {{include: ratelimit/store}}` 与 `  # {{include: ratelimit/store_test}}`(rate_limit_test.go 条目同样拆分搬移)。

- [ ] **Step 4: 重录 hertz golden 并审查 diff**

Run: `go test ./internal/scaffold/mono/ -run TestGenerateGolden -v` → 预期 FAIL(输出结构变化)
Run: `go test ./internal/scaffold/mono/ -run TestGenerateGolden -update-golden`
Run: `git diff internal/scaffold/mono/testdata/ | head -200`
Expected: diff 仅为"middleware 中的 store 代码消失 + 新增 internal/pkg/ratelimit/store.go 产物",无其他意外变化

- [ ] **Step 5: 全量测试 + Commit**

Run: `go build ./... && go test ./internal/scaffold/... ./internal/cli/...`
Expected: PASS

```bash
git add internal/assets/_data/ratelimit/ internal/assets/_data/hertz/layout.yaml internal/scaffold/mono/testdata/
git commit -m "refactor(assets): extract ratelimit store + BuildKey into shared fragment"
```

---

## Task 5: 抽取 rule-center client 共享片段

**Files:**
- Create: `internal/assets/_data/ratelimit/rule_center_client.yaml`
- Delete: `internal/assets/_data/hertz/optional/rule_center_client.go`
- Modify: `internal/scaffold/rulecenter/rulecenter.go`(writeRuleCenterClient :88-115)
- Modify: `internal/scaffold/mono/files.go`(writeHertzTemplate RuleCenterAddr 分支 :129-145)

- [ ] **Step 1: 生成片段**

```bash
cd internal/assets/_data
{
  echo "# Shared rule-center gRPC client (framework-agnostic)"
  echo "path: internal/pkg/middleware/rule_center_client.go"
  echo "update_behavior:"
  echo "  type: cover"
  echo "body: |-"
  sed 's/^/  /' hertz/optional/rule_center_client.go | sed 's/{{\.GoModule}}/{{.Module}}/g'
} > ratelimit/rule_center_client.yaml
git rm hertz/optional/rule_center_client.go
```

- [ ] **Step 2: 改两个读取点**

`rulecenter.go` 新增 helper(替换 writeRuleCenterClient 中 `fs.ReadFile(srcFS, "hertz/optional/rule_center_client.go")` + ReplaceAll 两行):

```go
func readSharedFragmentBody(srcFS fs.FS, name, module string) ([]byte, error) {
	b, err := fs.ReadFile(srcFS, name+".yaml")
	if err != nil {
		return nil, fmt.Errorf("read shared fragment %s: %w", name, err)
	}
	var frag struct {
		Body string `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &frag); err != nil {
		return nil, fmt.Errorf("parse shared fragment %s: %w", name, err)
	}
	return []byte(strings.ReplaceAll(frag.Body, "{{.Module}}", module)), nil
}
```

writeRuleCenterClient 改为读 `readSharedFragmentBody(srcFS, "ratelimit/rule_center_client", m.Module)`;`files.go` 的 RuleCenterAddr 分支同样改读共享片段(替换 `opts.Module`)。

- [ ] **Step 3: 运行 rulecenter + mono 测试**

Run: `go test ./internal/scaffold/rulecenter/ ./internal/scaffold/mono/ -v`
Expected: PASS(输出文件内容不变 → golden 不动)

- [ ] **Step 4: Commit**

```bash
git add internal/assets/_data/ratelimit/rule_center_client.yaml internal/scaffold/rulecenter/rulecenter.go internal/scaffold/mono/files.go
git commit -m "refactor(assets): rule-center client becomes shared fragment"
```

---

## Task 6: conf 新字段(kitex + hertz)

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/conf.yaml`(RateLimitConfig :73-87)
- Modify: `internal/assets/_data/hertz/layout.yaml`(hertz 侧 RateLimitConfig 条目)

- [ ] **Step 1: kitex conf.yaml 增字段**

`RateLimitConfig` 结构体 `Enabled` 行后插入:

```go
    Mode        string                    `json:"mode" yaml:"mode"`               // shadow | enforce
    Static      StaticLimitConfig         `json:"static" yaml:"static"`
    RuleCenter  RateLimitRuleCenterConfig `json:"rule_center" yaml:"rule_center"`
```

结构体区新增:

```go
  type StaticLimitConfig struct {
      MaxQPS         int `json:"max_qps" yaml:"max_qps"`
      MaxConnections int `json:"max_connections" yaml:"max_connections"`
  }

  type RateLimitRuleCenterConfig struct {
      Address                  string          `json:"address" yaml:"address"`
      QueryTimeoutMilliseconds config.Duration `json:"query_timeout_milliseconds" yaml:"query_timeout_milliseconds"`
  }
```

- [ ] **Step 2: hertz conf 同步(字段同名;Mode 默认语义在中间件侧处理)**

在 hertz layout.yaml 的 conf 条目 RateLimitConfig 中加入同样三个字段(与 kitex 逐字一致)。hertz 中间件:`cfg.Mode == "shadow"` 时走 shadow 分支;**空串视为 enforce**(现状不变)。

- [ ] **Step 3: 重录 golden 并审查**

Run: `go test ./internal/scaffold/mono/ -run TestGenerateGolden -update-golden && git diff internal/scaffold/mono/testdata/ --stat`
Expected: 仅 conf.go 产物新增字段

- [ ] **Step 4: Commit**

```bash
git add internal/assets/_data/kitex/kitex-template/conf.yaml internal/assets/_data/hertz/layout.yaml internal/scaffold/mono/testdata/
git commit -m "feat(assets): rate_limit conf gains mode/static/rule_center fields"
```

---

## Task 7: rpcerror.RateLimited(10429)

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/rpcerror.yaml`
- Modify: `internal/assets/_data/kitex/kitex-template/rpcerror_test.yaml`

- [ ] **Step 1: 先在测试模板中加失败用例**

`rpcerror_test.yaml` body 末尾追加:

```go
  func TestRateLimited(t *testing.T) {
      err := RateLimited(30 * time.Second)
      bizErr, ok := kerrors.FromBizStatusError(err)
      if !ok {
          t.Fatalf("RateLimited must be a BizStatusError, got %T", err)
      }
      if bizErr.BizStatusCode() != 10429 {
          t.Errorf("biz code = %d, want 10429", bizErr.BizStatusCode())
      }
      if !strings.Contains(bizErr.BizMessage(), "rate limited") {
          t.Errorf("biz message = %q", bizErr.BizMessage())
      }
  }
```

- [ ] **Step 2: rpcerror.yaml body 增实现**

imports 增 `"time"` 与 `"github.com/cloudwego/kitex/pkg/kerrors"`(若缺);code 变量区增:

```go
      CodeRateLimited int32 = 10429
```

函数区增:

```go
  // RateLimited returns a BizStatusError (biz code 10429, mirroring HTTP 429).
  // BizStatusError is counted as a business error by Kitex, so it does not
  // trip caller-side error-ratio circuit breakers.
  func RateLimited(retryAfter time.Duration) error {
      msg := "rate limited"
      if retryAfter > 0 {
          msg = fmt.Sprintf("rate limited; retry after %s", retryAfter.Round(time.Second))
      }
      return kerrors.NewBizStatusError(CodeRateLimited, msg)
  }
```

- [ ] **Step 3: 重录 rpc golden 并验证**

Run: `go test ./internal/scaffold/rpc/ -run TestGenerateGoldenRPC -update-golden && go test ./internal/scaffold/rpc/ -v`
Expected: PASS;golden diff 仅 rpcerror.go/_test.go

- [ ] **Step 4: Commit**

```bash
git add internal/assets/_data/kitex/kitex-template/rpcerror.yaml internal/assets/_data/kitex/kitex-template/rpcerror_test.yaml internal/scaffold/rpc/testdata/
git commit -m "feat(assets): kitex rpcerror.RateLimited biz code 10429"
```

---

## Task 8: kitex 限流中间件真实实现

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml`(整体重写)
- Create: `internal/assets/_data/kitex/kitex-template/ratelimit_middleware_test.yaml`
- Modify: `internal/assets/_data/kitex/layout-rulecenter.yaml`

**Interfaces:**
- Consumes: 共享 `ratelimit.Resolver/Store/Lookup/BuildKey`(Task 2/4)、`rpcerror.RateLimited`(Task 7)、conf 新字段(Task 6)、middleware 包的 `NewRuleCenterClient`(Task 5 片段)
- Produces: `middleware.RateLimit(cfg conf.RateLimitConfig) endpoint.Middleware`;`middleware.RateLimitWith(cfg, MiddlewareDeps) endpoint.Middleware`(测试注入);`middleware.StaticLimitOption(s conf.StaticLimitConfig) kitexserver.Option`

- [ ] **Step 1: 重写 ratelimit_middleware.yaml**

```yaml
# Kitex custom template — rate-limit middleware (dual-track dynamic enforcement)
path: internal/base/middleware/ratelimit.go
update_behavior:
  type: cover
body: |-
  // Code generated by ncgo. Dynamic rate-limit enforcement for Kitex services.

  package middleware

  import (
      "context"
      "expvar"
      "strconv"
      "strings"
      "sync"
      "time"

      "github.com/bytedance/gopkg/cloud/metainfo"
      "github.com/cloudwego/kitex/pkg/endpoint"
      "github.com/cloudwego/kitex/pkg/klog"
      "github.com/cloudwego/kitex/pkg/rpcinfo"
      kitexserver "github.com/cloudwego/kitex/server"
      "github.com/cloudwego/kitex/server/limit"
      "github.com/redis/go-redis/v9"

      "{{.Module}}/internal/base/conf"
      "{{.Module}}/internal/pkg/ratelimit"
      "{{.Module}}/internal/pkg/rpcerror"
  )

  // MetaRetryAfter carries the suggested backoff to callers on rejection.
  const MetaRetryAfter = "rl-retry-after"

  var shadowDenied = expvar.NewMap("ratelimit_shadow_denied")

  // MiddlewareDeps allows tests to inject a fake resolver/store/lookup.
  type MiddlewareDeps struct {
      Resolver *ratelimit.Resolver
      Store    ratelimit.Store
      Lookup   func(ctx context.Context) ratelimit.Lookup
  }

  // RateLimit enforces resolved rules per RPC. Shadow mode (cfg.Mode !=
  // "enforce") counts but never rejects. Rules resolve via cfg.Source
  // (config/database/rule_center/grpc); the rule-center client is built
  // lazily from cfg, so no manual wiring is needed.
  func RateLimit(cfg conf.RateLimitConfig) endpoint.Middleware {
      return RateLimitWith(cfg, MiddlewareDeps{})
  }

  // RateLimitWith is RateLimit with injectable dependencies (tests).
  func RateLimitWith(cfg conf.RateLimitConfig, deps MiddlewareDeps) endpoint.Middleware {
      resolver := deps.Resolver
      if resolver == nil {
          resolver = ratelimit.NewResolver(cfg, buildRateLimitOptions(cfg))
      }
      store := deps.Store
      if store == nil {
          store = ratelimit.NewStore(cfg, rateLimitRedisClient(cfg))
      }
      lookupFn := deps.Lookup
      if lookupFn == nil {
          lookupFn = lookupFromRPCInfo
      }
      enforce := strings.EqualFold(strings.TrimSpace(cfg.Mode), "enforce")
      return func(next endpoint.Endpoint) endpoint.Endpoint {
          return func(ctx context.Context, req, resp interface{}) error {
              if !cfg.Enabled {
                  return next(ctx, req, resp)
              }
              lookup := lookupFn(ctx)
              resolved, err := resolver.Resolve(ctx, lookup)
              if err != nil {
                  if cfg.FailOpen {
                      return next(ctx, req, resp)
                  }
                  return rpcerror.RateLimited(0)
              }
              rule := ratelimit.NormalizeRule(resolved.Rule)
              if !rule.Enabled {
                  return next(ctx, req, resp)
              }
              ok, err := store.Allow(ctx, ratelimit.BuildKey(lookup, rule.KeyBy, cfg.KeyPrefix), rule)
              if err != nil {
                  if cfg.FailOpen {
                      return next(ctx, req, resp)
                  }
                  return rpcerror.RateLimited(0)
              }
              if !ok {
                  if enforce {
                      ctx = metainfo.WithValue(ctx, MetaRetryAfter, retryAfterSeconds(rule))
                      return rpcerror.RateLimited(rule.ClientTTLSeconds.Duration)
                  }
                  shadowDenied.Add(lookup.Service+"/"+lookup.Method, 1)
                  klog.CtxWarnf(ctx, "ratelimit shadow denied: service=%s method=%s caller=%s",
                      lookup.Service, lookup.Method, lookup.AppKey)
              }
              return next(ctx, req, resp)
          }
      }
  }

  // StaticLimitOption returns kitexserver.WithLimit as a coarse global safety
  // net, or nil when both limits are zero (default: not mounted).
  func StaticLimitOption(s conf.StaticLimitConfig) kitexserver.Option {
      if s.MaxQPS <= 0 && s.MaxConnections <= 0 {
          return nil
      }
      return kitexserver.WithLimit(&limit.Option{MaxQPS: s.MaxQPS, MaxConnections: s.MaxConnections})
  }

  func retryAfterSeconds(rule conf.RateLimitRuleConfig) string {
      d := rule.ClientTTLSeconds.Duration
      if d <= 0 {
          d = 60 * time.Second
      }
      return formatInt(int64(d.Seconds()))
  }

  func formatInt(v int64) string { return strconv.FormatInt(v, 10) }

  func lookupFromRPCInfo(ctx context.Context) ratelimit.Lookup {
      var lookup ratelimit.Lookup
      if ri := rpcinfo.GetRPCInfo(ctx); ri != nil {
          if to := ri.To(); to != nil {
              lookup.Service = to.ServiceName()
              lookup.Method = to.Method()
          }
          if from := ri.From(); from != nil {
              if from.Address() != nil {
                  lookup.ClientIP = from.Address().String()
              }
              if lookup.AppKey == "" {
                  lookup.AppKey = from.ServiceName()
              }
          }
      }
      if v := metainfo.GetPersistentValue(ctx, "x-caller-service"); v != "" {
          lookup.AppKey = v
      }
      lookup.Phase = "post_auth"
      return lookup
  }

  var (
      ruleCenterOnce   sync.Once
      ruleCenterClient ratelimit.GRPCClient
  )

  func buildRateLimitOptions(cfg conf.RateLimitConfig) ratelimit.Options {
      opts := ratelimit.Options{}
      if cfg.Source.Type == "rule_center" || cfg.Source.Type == "grpc" {
          ruleCenterOnce.Do(func() {
              addr := cfg.RuleCenter.Address
              if addr == "" {
                  addr = cfg.GRPC.Target
              }
              if addr == "" {
                  return
              }
              cli, err := NewRuleCenterClient(addr)
              if err != nil {
                  klog.Warnf("ratelimit: rule-center client init failed: %v (falling back to local rules)", err)
                  return
              }
              ruleCenterClient = cli
          })
          if cfg.Source.Type == "rule_center" {
              opts.RuleCenter = ruleCenterClient
          } else {
              opts.GRPC = ruleCenterClient
          }
      }
      return opts
  }
  func rateLimitRedisClient(cfg conf.RateLimitConfig) redis.UniversalClient {
      if cfg.Backend != "redis" || len(cfg.Redis.Addrs) == 0 {
          return nil
      }
      return redis.NewUniversalClient(&redis.UniversalOptions{
          Addrs:      cfg.Redis.Addrs,
          Username:   cfg.Redis.Username,
          Password:   cfg.Redis.Password,
          DB:         cfg.Redis.DB,
          MasterName: cfg.Redis.MasterName,
      })
  }
```

> 实现前先 `grep -n "redis" internal/assets/_data/kitex/kitex-template/*.yaml` 核对 `conf.RedisConfig` 字段名(conf.yaml 已列出 Addrs/Username/Password/DB/MasterName),按实际字段对齐。

- [ ] **Step 2: 写中间件测试模板**

`ratelimit_middleware_test.yaml`:

```yaml
# Kitex custom template — rate-limit middleware tests
path: internal/base/middleware/ratelimit_test.go
update_behavior:
  type: cover
body: |-
  package middleware

  import (
      "context"
      "strings"
      "testing"
      "time"

      "github.com/cloudwego/kitex/pkg/kerrors"

      config "github.com/byx-darwin/go-tools/go-framework/config"

      "{{.Module}}/internal/base/conf"
      "{{.Module}}/internal/pkg/ratelimit"
  )

  type fakeStore struct{ ok bool; err error; calls int }

  func (s *fakeStore) Allow(ctx context.Context, key string, rule conf.RateLimitRuleConfig) (bool, error) {
      s.calls++
      return s.ok, s.err
  }

  func fixedDeps(store ratelimit.Store) MiddlewareDeps {
      return MiddlewareDeps{
          Resolver: ratelimit.NewResolver(conf.RateLimitConfig{PostAuth: conf.RateLimitPhaseConfig{Enabled: true, DefaultRule: conf.RateLimitRuleConfig{Enabled: true, Strategy: "fixed_window", WindowSeconds: config.Duration{Duration: 60 * time.Second}, MaxRequests: 1}}}, ratelimit.Options{}),
          Store:    store,
          Lookup:   func(ctx context.Context) ratelimit.Lookup { return ratelimit.Lookup{Service: "svc", Method: "Ping", Phase: "post_auth"} },
      }
  }

  func callMW(t *testing.T, mw endpoint.Middleware) error {
      t.Helper()
      return mw(func(ctx context.Context, req, resp interface{}) error { return nil })(context.Background(), nil, nil)
  }

  func baseCfg(mode string, enabled, failOpen bool) conf.RateLimitConfig {
      return conf.RateLimitConfig{
          Enabled:  enabled,
          Mode:     mode,
          FailOpen: failOpen,
          PostAuth: conf.RateLimitPhaseConfig{Enabled: true, DefaultRule: conf.RateLimitRuleConfig{
              Enabled: true, Strategy: "fixed_window",
              WindowSeconds:    config.Duration{Duration: 60 * time.Second},
              MaxRequests:      1,
              ClientTTLSeconds: config.Duration{Duration: 30 * time.Second},
          }},
      }
  }

  func TestRateLimitEnforceRejects10429(t *testing.T) {
      store := &fakeStore{ok: false}
      mw := RateLimitWith(baseCfg("enforce", true, true), fixedDeps(store))
      err := callMW(t, mw)
      bizErr, ok := kerrors.FromBizStatusError(err)
      if !ok {
          t.Fatalf("want BizStatusError, got %v", err)
      }
      if bizErr.BizStatusCode() != 10429 {
          t.Errorf("biz code = %d, want 10429", bizErr.BizStatusCode())
      }
      if store.calls != 1 {
          t.Errorf("store.calls = %d, want 1", store.calls)
      }
  }

  func TestRateLimitShadowPassesButCounts(t *testing.T) {
      store := &fakeStore{ok: false}
      mw := RateLimitWith(baseCfg("shadow", true, true), fixedDeps(store))
      if err := callMW(t, mw); err != nil {
          t.Fatalf("shadow must pass, got %v", err)
      }
      if store.calls != 1 {
          t.Errorf("store.calls = %d, want 1 (shadow must still count)", store.calls)
      }
  }

  func TestRateLimitDisabledPasses(t *testing.T) {
      store := &fakeStore{ok: false}
      mw := RateLimitWith(baseCfg("enforce", false, true), fixedDeps(store))
      if err := callMW(t, mw); err != nil {
          t.Fatalf("disabled must pass, got %v", err)
      }
      if store.calls != 0 {
          t.Errorf("store.calls = %d, want 0", store.calls)
      }
  }

  func TestRateLimitStoreErrorFailOpen(t *testing.T) {
      store := &fakeStore{err: errors.New("redis down")}
      mw := RateLimitWith(baseCfg("enforce", true, true), fixedDeps(store))
      if err := callMW(t, mw); err != nil {
          t.Fatalf("fail_open must pass on store error, got %v", err)
      }
  }

  func TestStaticLimitOptionNilWhenZero(t *testing.T) {
      if opt := StaticLimitOption(conf.StaticLimitConfig{}); opt != nil {
          t.Fatalf("zero static config must return nil option")
      }
  }
```

> 测试模板 imports 需相应包含 `errors`、`endpoint`(`github.com/cloudwego/kitex/pkg/endpoint`)。

- [ ] **Step 3: layout-rulecenter.yaml 引用新模板与共享片段**

`templates:` 列表中 `kitex-template/ratelimit_middleware.yaml` 后追加:

```yaml
  - kitex-template/ratelimit_middleware_test.yaml
  - ../ratelimit/resolver.yaml
  - ../ratelimit/resolver_test.yaml
  - ../ratelimit/store.yaml
  - ../ratelimit/store_test.yaml
  - ../ratelimit/rule_center_client.yaml
```

> 相对路径以 kitex CLI layout 解析规则为准:实现时先验证 `kitex` 模板列表是否支持 `../`;若不支持,改为在 `writeKitexTemplate`(files.go)中把共享片段复制进 `template/kitex-template/`(文件名加 `shared_` 前缀)并在 layout 中按新名引用 —— 两种路径择一,以 golden 通过为准。

- [ ] **Step 4: 重录 rpc preset golden 并审查**

Run: `go test ./internal/scaffold/rpc/ -run TestGenerateGoldenRPCWithPreset -update-golden && git diff internal/scaffold/rpc/testdata/ --stat`
Expected: diff = middleware 占位符→真实实现、新增 middleware_test、新增 internal/pkg/ratelimit/* 产物

- [ ] **Step 5: Commit**

```bash
git add internal/assets/_data/kitex/ internal/scaffold/rpc/testdata/ internal/scaffold/mono/files.go
git commit -m "feat(assets): real kitex rate-limit middleware (dual-track, shadow default)"
```

---

## Task 9: server.yaml 静态轨 wire 标记

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/server.yaml`

- [ ] **Step 1: 加第二个标记**

server.yaml body 中 `opts = append(opts, extraOptions...)` 行之前插入:

```go
      // Optional static rate-limit safety net (after `ncgo add infra rate-limit`):
      // middleware.StaticLimitOption mounts kitexserver.WithLimit when configured.
      // ncgo:wire:ratelimit:static-limit
```

(第一个标记 `// ncgo:wire:ratelimit:server-middleware` 已存在于 WithMiddleware 链注释区,保留。)

- [ ] **Step 2: 重录 golden + Commit**

Run: `go test ./internal/scaffold/rpc/ -run TestGenerateGoldenRPC -update-golden && go test ./internal/scaffold/rpc/ ./internal/scaffold/mono/`

```bash
git add internal/assets/_data/kitex/kitex-template/server.yaml internal/scaffold/rpc/testdata/
git commit -m "feat(assets): kitex server gains ratelimit static-limit wire marker"
```

---

## Task 10: `ncgo add infra rate-limit`(kitex-only)

**Files:**
- Modify: `internal/scaffold/infra/infra.go`(kinds/assetFiles/conf/nextSteps)
- Modify: `internal/scaffold/infra/wire.go`(marker 常量 + wireKitex case)
- Modify: `internal/scaffold/infra/infra_test.go`

**Interfaces:**
- Consumes: 共享片段(Task 2/4)、kitex 中间件模板(Task 8)、server.yaml 两标记(Task 9)、`insertAfterMarkerOrAnyWithPlan`/`insertOnceMarkerOrAnchorWithPlan`(既有)
- Produces: `KindRateLimit = "rate_limit"`(+ alias `rate-limit`);kitex-only 校验;写 4 个 pkg 文件 + 覆盖 middleware;conf/dev/conf.yaml 合并 rate_limit 块;wire 激活两标记

- [ ] **Step 1: 写失败测试**

`infra_test.go` 追加(仿既有 polaris/redis 用例的 kitex fixture):

```go
func TestAddInfraRateLimitKitex(t *testing.T) {
	root := newKitexProjectFixture(t) // fixture 搭建仿既有 polaris 用例:先 grep -n "polaris" internal/scaffold/infra/infra_test.go 找到 kitex manifest + server.go + conf/dev/conf.yaml 的既有搭建代码,复用其 helper/写法
	res, err := Add(Options{Root: root, Kind: "rate-limit", Wire: true})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	for _, want := range []string{
		"internal/pkg/ratelimit/resolver.go",
		"internal/pkg/ratelimit/store.go",
		"internal/base/middleware/ratelimit.go",
	} {
		assertFileContains(t, root, want, "package ")
	}
	assertFileContains(t, root, "internal/base/middleware/ratelimit.go", "func RateLimit(")
	assertFileContains(t, root, "conf/dev/conf.yaml", "mode: shadow")
	server := readFile(t, root, "internal/base/server/server.go")
	if !strings.Contains(server, "middleware.RateLimit(cfg.RateLimit)") {
		t.Errorf("server.go missing middleware wiring")
	}
	if !strings.Contains(server, "middleware.StaticLimitOption(") {
		t.Errorf("server.go missing static-limit wiring")
	}
	_ = res
}

func TestAddInfraRateLimitRejectsHertz(t *testing.T) {
	root := newHertzProjectFixture(t)
	_, err := Add(Options{Root: root, Kind: "rate-limit", Wire: true})
	if err == nil || !strings.Contains(err.Error(), "kitex") {
		t.Fatalf("want kitex-only error, got %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/scaffold/infra/ -run 'TestAddInfraRateLimit' -v`
Expected: FAIL — unknown kind

- [ ] **Step 3: 实现 kind**

`infra.go`:
- `KindRateLimit = "rate_limit"`;`kitexOnlyKinds()` 追加;`SupportedKinds()` 追加;`normalizeKind` 增 alias `"rate-limit" → KindRateLimit`;
- `assetFiles`:rate_limit case 返回多文件 —— 从 `assets.FS()` 读 `ratelimit/{resolver,resolver_test,store,store_test}.yaml` 解析 body(`renderAssetBody` 渲染 `{{.Module}}`),目标 `internal/pkg/ratelimit/*.go`;再读 `kitex/kitex-template/ratelimit_middleware.yaml` body 渲染后目标 `internal/base/middleware/ratelimit.go`(action=cover,已存在不报 exists 错);
- conf 合并:新函数 `planKitexRateLimitConfig(root)`:读 `conf/dev/conf.yaml`,无 `rate_limit:` 块则追加:

```yaml
rate_limit:
  enabled: true
  mode: shadow
  backend: memory
  fail_open: true
  source:
    type: config
    cache_ttl_seconds: 60s
    fallback_on_error: true
  static:
    max_qps: 0
    max_connections: 0
```

已有块则将 `enabled:` 置 true、`mode:` 置 shadow(文本替换,作用域限定 rate_limit 块内,仿 `updateConfForRuleCenter` 技法);
- `nextSteps` rate_limit case:

```go
return []string{
    "review conf/dev/conf.yaml rate_limit block (source.type: config|database|rule_center)",
    "observe shadow logs (grep 'ratelimit shadow denied'), then set mode: enforce",
    "optional: set static.max_qps / static.max_connections for a global safety net",
    "go mod tidy",
}
```

- [ ] **Step 4: 实现 wire case**

`wire.go` marker 常量区增:

```go
	markerRateLimitServerMiddleware = "// ncgo:wire:ratelimit:server-middleware"
	markerRateLimitStaticLimit      = "// ncgo:wire:ratelimit:static-limit"
```

`wireSupportedKind` 增 KindRateLimit;`wireKitex` switch 增:

```go
	case KindRateLimit:
		s, err = addGoImportWithPlan(s, module+"/internal/base/middleware", serverPath, &serverPlan)
		if err != nil {
			return nil, err
		}
		s, err = insertAfterMarkerOrAnyWithPlan(s, "middleware.RateLimit(", markerRateLimitServerMiddleware, []string{
			"\t\t\tinterceptor.RequestID(),\n",
		}, "\t\t\tmiddleware.RateLimit(cfg.RateLimit),\n", serverPath, &serverPlan, "insert_ratelimit_middleware", "middleware.RateLimit")
		if err != nil {
			return nil, err
		}
		s, err = insertOnceMarkerOrAnchorWithPlan(s, "middleware.StaticLimitOption(", markerRateLimitStaticLimit, "\topts = append(opts, extraOptions...)\n", "\tif opt := middleware.StaticLimitOption(cfg.RateLimit.Static); opt != nil {\n\t\topts = append(opts, opt)\n\t}\n", serverPath, &serverPlan, "insert_ratelimit_static", "middleware.StaticLimitOption")
		if err != nil {
			return nil, err
		}
```

- [ ] **Step 5: 运行测试至通过**

Run: `go test ./internal/scaffold/infra/ -run 'TestAddInfraRateLimit' -v`
Expected: PASS(如 infra golden 机制覆盖 kind 输出,同步 `-update-golden` 并审查 diff)

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/
git commit -m "feat(infra): add infra rate-limit for kitex services (dual-track wiring)"
```

---

## Task 11: `add rule-center` 放开 kitex

**Files:**
- Modify: `internal/scaffold/rulecenter/rulecenter.go`(Add :32-86)
- Modify: `internal/scaffold/rulecenter/rulecenter_test.go`(TestAddRejectsKitex :77)

- [ ] **Step 1: 翻转测试**

`TestAddRejectsKitex` → `TestAddAcceptsKitex`:

```go
func TestAddAcceptsKitex(t *testing.T) {
	dir := t.TempDir()
	makeKitexManifest(t, dir) // 既有 helper
	writeConfDev(t, dir)      // conf/dev/conf.yaml 含 rate_limit: 块(仿 hertz 用例)
	res, err := Add(Options{Root: dir, Addr: "rule-center:8888"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertClientWritten(t, dir)                       // internal/pkg/middleware/rule_center_client.go 存在且含 NewRuleCenterClient
	assertConfSourceType(t, dir, "rule_center")       // conf source.type 改写成功
	serverPath := filepath.Join(dir, "internal", "base", "server", "server.go")
	if _, err := os.Stat(serverPath); err == nil {
		b, _ := os.ReadFile(serverPath)
		if strings.Contains(string(b), "RuleCenter") {
			t.Errorf("kitex branch must NOT wire server.go")
		}
	}
	_ = res
}
```

- [ ] **Step 2: 改 Add 的 kind 分支**

rulecenter.go Add 中:

```go
	if m.Service.Kind != "hertz" && m.Service.Kind != "kitex" {
		return nil, fmt.Errorf("rule-center: only supported for hertz/kitex services (got %s)", m.Service.Kind)
	}
```

步骤 3(wireRuleCenterInServer)包裹:

```go
	if m.Service.Kind == "hertz" {
		// 3. Wire RuleCenter client into server.go if it exists
		...(原逻辑)
	}
```

(kitex 中间件按设计 §6.1 自建 client,无需 wire。)

- [ ] **Step 3: 运行测试**

Run: `go test ./internal/scaffold/rulecenter/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/rulecenter/
git commit -m "feat(rulecenter): support kitex services (client + conf, self-built client wiring)"
```

---

## Task 12: e2e 扩展 Kitex RPC 压测

**Files:**
- Create: `internal/scaffold/test/ratelimit/attack_grpc.go`、`attack_grpc_test.go`
- Modify: `internal/scaffold/test/ratelimit/e2e.go`(E2EOptions/E2E :58-163)

**Interfaces:**
- Consumes: 既有 `attackResult`/`classifyResult`(e2e.go :400-548)
- Produces: `runRPCAttackCapture(ctx, opts E2EOptions) (*attackResult, error)` —— grpcurl worker pool;biz code 10429 计入 `Status429`

- [ ] **Step 1: 写解析器失败测试**

`attack_grpc_test.go`:

```go
package ratelimit

import "testing"

func TestClassifyGRPCResults(t *testing.T) {
	outputs := []grpcCallResult{
		{bizCode: 0}, {bizCode: 10429}, {bizCode: 10429}, {errOther: true},
	}
	ar := aggregateGRPCResults(outputs)
	if ar.TotalReqs != 4 || ar.Status200 != 1 || ar.Status429 != 2 || ar.StatusOther != 1 {
		t.Fatalf("unexpected aggregate: %+v", ar)
	}
}

func TestParseGRPCURLBizCode(t *testing.T) {
	if got := parseBizCode(`{"code":10429,"msg":"rate limited"}`+"\n"); got != 10429 {
		t.Errorf("parseBizCode = %d, want 10429", got)
	}
	if got := parseBizCode(`{"found":false}`); got != 0 {
		t.Errorf("parseBizCode = %d, want 0", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/scaffold/test/ratelimit/ -run 'TestParseGRPCURL|TestClassifyGRPC' -v`
Expected: FAIL — 未定义

- [ ] **Step 3: 实现 attack_grpc.go**

```go
package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type grpcCallResult struct {
	bizCode  int
	errOther bool
	latency  time.Duration
}

var bizCodeRE = regexp.MustCompile(`"code"\s*:\s*(\d+)`)

// parseBizCode extracts a kitex biz status code from grpcurl JSON output.
// Returns 0 when the call carried no biz error.
func parseBizCode(output string) int {
	var payload struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &payload); err == nil {
		return payload.Code
	}
	if m := bizCodeRE.FindStringSubmatch(output); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func aggregateGRPCResults(results []grpcCallResult) *attackResult {
	ar := &attackResult{}
	var total time.Duration
	var p99s []time.Duration
	for _, r := range results {
		ar.TotalReqs++
		switch {
		case r.errOther:
			ar.StatusOther++
		case r.bizCode == 10429:
			ar.Status429++
		case r.bizCode == 0:
			ar.Status200++
		default:
			ar.StatusOther++
		}
		total += r.latency
		p99s = append(p99s, r.latency)
	}
	if ar.TotalReqs > 0 {
		ar.AvgLatency = total / time.Duration(ar.TotalReqs)
	}
	ar.P99Latency = percentile(p99s, 0.99)
	return ar
}

// runRPCAttackCapture fires rate rps for duration against host:port invoking
// rpcMethod via grpcurl, aggregating biz-status outcomes (10429 = rejected).
func runRPCAttackCapture(ctx context.Context, opts E2EOptions, rpcMethod, payload string) (*attackResult, error) {
	if _, err := exec.LookPath("grpcurl"); err != nil {
		return nil, fmt.Errorf("grpcurl not found: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest")
	}
	dur, err := time.ParseDuration(opts.Duration)
	if err != nil {
		return nil, fmt.Errorf("duration: %w", err)
	}
	target := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	total := opts.Rate * int(dur.Seconds())
	results := make([]grpcCallResult, total)
	sem := make(chan struct{}, opts.Rate)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < total; i++ {
		// pace: sleep until the i-th slot's scheduled time
		if want := start.Add(time.Duration(i) * time.Second / time.Duration(opts.Rate)); time.Now().Before(want) {
			time.Sleep(time.Until(want))
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			t0 := time.Now()
			cmd := exec.CommandContext(ctx, "grpcurl", "-plaintext", "-d", payload, target, rpcMethod)
			out, err := cmd.CombinedOutput()
			res := grpcCallResult{latency: time.Since(t0)}
			if err != nil && parseBizCode(string(out)) == 0 {
				res.errOther = true
			} else {
				res.bizCode = parseBizCode(string(out))
			}
			results[idx] = res
		}(i)
	}
	wg.Wait()
	return aggregateGRPCResults(results), nil
}
```

percentile 实现(同文件):

```go
func percentile(vals []time.Duration, p float64) time.Duration {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), vals...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
```

(attack_grpc.go imports 相应增 `sort`。)

- [ ] **Step 4: e2e.go 接入 kitex 分支**

`E2EOptions` 增 `RPCMethod string`、`RPCPayload string`;`E2E` 第 7 步(runAttackCapture)改为:

```go
	var ar *attackResult
	if kind == manifest.KindKitex {
		method := opts.RPCMethod
		if method == "" {
			method = serviceName + ".HealthCheck"
		}
		payload := opts.RPCPayload
		if payload == "" {
			payload = `{}`
		}
		ar, err = runRPCAttackCapture(ctx, opts, method, payload)
	} else {
		ar, err = runAttackCapture(ctx, opts.Root, opts.Host, opts.Port, opts.Rate, opts.Duration, opts.Paths)
	}
	if err != nil {
		return result, fmt.Errorf("run attack: %w", err)
	}
```

readiness(第 5 步):kitex 用 TCP 探测替换 HTTP —— `waitForReadyTCP(ctx, host, port, 2s, 30s)`(net.Dial 循环,新函数,同文件)。

- [ ] **Step 5: CLI 暴露新 flag**

`internal/cli/test.go` e2e 命令增 `--rpc-method`、`--rpc-payload` flags 传入 E2EOptions(仿既有 paths flag 写法)。

- [ ] **Step 6: 运行测试**

Run: `go test ./internal/scaffold/test/ratelimit/ ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/test/ratelimit/ internal/cli/test.go
git commit -m "feat(e2e): rate-limit e2e supports kitex via grpcurl RPC attacker"
```

---

## Task 13: CLI/MCP 一致性 + 全量回归

**Files:**
- Modify: `internal/cli/add_infra.go`(帮助文本/flags 若按 kind 枚举列出 —— 先 grep 确认)

- [ ] **Step 1: 确认 CLI/MCP 无需结构改动**

Run: `grep -rn "rate-limit\|SupportedKinds" internal/cli/add_infra.go internal/mcp/*.go | grep -v test | head`
Expected: kind 为通用字符串参数(设计假设成立)则本步记录结论;若帮助文本硬编码 kind 列表,追加 `rate-limit`。

- [ ] **Step 2: 全量质量闸门**

Run: `gofmt -l . && go vet ./... && go build ./... && go test ./...`
Expected: gofmt 输出为空;其余 PASS

- [ ] **Step 3: Commit(如有帮助文本变更)**

```bash
git add -A && git commit -m "docs(cli): list rate-limit in add infra help" || echo "no changes"
```

---

## Task 14: 文档

**Files:**
- Modify: `internal/assets/_data/docs/hertz/rate-limit-dynamic-design.zh-CN.md` + `.en.md`
- Modify: `README.md`(命令清单)

- [ ] **Step 1: 设计文档增 Kitex 章节**

zh-CN 文档目录 §11 后新增 `## 12. Kitex 服务限流`,覆盖:双轨模型、shadow→enforce 运维流程、10429 语义与 caller 处理建议、静态兜底配置建议、与 Hertz 共享基建的边界。en 版同步。

- [ ] **Step 2: README 增命令示例**

`ncgo add infra rate-limit` 与 `ncgo test rate-limit e2e --rpc-method` 各一段。

- [ ] **Step 3: Commit**

```bash
git add internal/assets/_data/docs/ README.md
git commit -m "docs: kitex rate-limit enforcement (dual-track, shadow-first)"
```

---

## 任务依赖图

```
T1(展开器) → T2(片段) → T3(layout 换用) → T4(store) → T5(client)
T6(conf) ──┐
T7(rpcerror) ─┼→ T8(中间件) → T9(标记) → T10(add infra) → T13(回归)
T2/T4/T5 ────┘                    ↘ T11(rulecenter)  ↗
T10 → T12(e2e) → T13 → T14(文档)
```

T6/T7 可与 T3–T5 并行;T11 仅依赖 T5;T12 依赖 T10(需要可执行的 add infra 场景)。

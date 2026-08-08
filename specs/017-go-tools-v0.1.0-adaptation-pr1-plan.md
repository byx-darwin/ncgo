# PR1 实施计划（v2）— 生成项目适配 go-tools v0.1.0：修通编译 + 补全缺口

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修通 origin/main 上 go-tools 深度集成的编译错误（errcode/rpcerror 引错包 + CodePermissionDenied 不存在），并补全缺口（go.mod 1.26.5+require、Docker 1.26.5、optional add-on oops→goerror、kitex 硬编码码、goGetDeps、文档、e2e 编译检查），使 `ncgo new` 生成的 Hertz/Kitex 项目可编译且完整对接 go-tools v0.1.0。

**Architecture:** 沿用 origin/main 的深度集成（生成项目 = go-tools 之上的薄业务层：Responder / go-framework/config / go-common/log / go-middleware/db）。本 PR 只「修通 + 补全」，不自创码段。改动全部在模板（`internal/assets/_data/`）与脚手架代码（`internal/scaffold/`），不改 ncgo 业务逻辑。

**Tech Stack:** Go 1.25（ncgo 构建）/ 生成项目 go 1.26.5 · go-tools v0.1.0（go-common/go-framework 必需，go-middleware 条件）· hertz-template per-file yaml + kitex-template · golden 测试。

## Global Constraints

- 框架码常量（CodeSystem 等）来自 **`go-framework/error`**（别名 `frameworkerror`），**不是** go-common/error；go-common/error（别名 `goerror`）只提供机制（Code/In/Extract/HTTPStatus/RegisterHTTPStatuses + 码段常量）。
- 生成 go.mod：`go 1.26.5` + `require go-common v0.1.0` + `go-framework v0.1.0`（go-middleware 由 tidy 补）。
- Docker 基础镜像统一 `golang:1.26.5`（裸版）/ `golang:1.26.5-alpine`。
- `CodePermissionDenied` → `frameworkerror.CodeAuthFailed`（10002）。
- oops 构造（oops.In/Code）→ `goerror.In/Code`（goerror 包装 oops）；`oops.AsOops` → `goerror.AsOopsError`。
- ncgo 自身 `go.mod` **零改动**（不依赖 go-tools）。
- 模板/脚手架输出 contract-sensitive：golden diff 逐提交审查。
- 验证链：`go build ./... && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`；golden 更新用精确包路径 `-update-golden`（不能传 `./internal/scaffold/...` 全树）。

## go-framework/error 框架码参考

`CodeSystem=10000`、`CodeParamInvalid=10001`、`CodeAuthFailed=10002`、`CodeConfigNotFound=10003`、`CodeConfigInvalid=10004`、`CodePolarisInit=10005`、`CodePolarisGetConfig=10006`、`CodeRPCUnavailable=10010`、`CodeRPCTimeout=10011`、`CodeRPCDecodeError=10012`、`CodeRPCEncodeError=10013`。

## 旧 scaffold 码 → 处理（optional add-on / kitex 硬编码用）

`10000→frameworkerror.CodeSystem`、`10308(config_invalid)→frameworkerror.CodeConfigInvalid(10004)`、`10301(rpc_failed)→frameworkerror.CodeRPCUnavailable(10010)`、`10303(database)/10304(cache)/10306(search) unavailable`：go-framework 无等价码 → 用项目段码（建议 `40503 database_unavailable`/`40504 cache_unavailable`/`40506 search_unavailable`，≥40100）并在生成包 init() 用 `goerror.RegisterHTTPStatuses` 注册为 503。字符串码（如 kitex registry 的 `"registry_config_invalid"`）保持字符串码，仅迁 oops→goerror 机制。

## File Structure

| 文件 | 动作 |
|------|------|
| `internal/assets/_data/hertz/hertz-template/errcode_go.yaml` | Modify（import + 6 常量改 frameworkerror） |
| `internal/assets/_data/kitex/kitex-template/rpcerror.yaml` | Modify（import + 4 常量修复 + oops→goerror） |
| `internal/scaffold/{mono,bff,rpc}/testdata/**/template/**/{errcode_go.yaml,rpcerror.yaml}` | Regenerate（golden 副本） |
| `internal/assets/_data/hertz/layout.yaml`（go.mod 条目） | Modify（go 1.26.5 + require） |
| `internal/scaffold/mono/`（kitex go.mod 预写） | Modify（writeKitexGoMod，钉 kitex go.mod） |
| `internal/scaffold/shared/container.go`、`internal/assets/_data/hertz/Dockerfile.vegeta`、各 testdata Dockerfile、`internal/mcp/demo/Dockerfile` | Modify（golang 1.26.5） |
| `internal/assets/_data/hertz/optional/{redis,kafka,es,clickhouse,observability_logging}.go`、`kitex/optional/{registry_etcd,observability_logging}.go` | Modify（oops→goerror + 码对齐 + HTTP 注册） |
| `internal/assets/_data/kitex/kitex-template/{client,conf,repository,usecase}.yaml`、`hertz/hertz-template/{repository_go,usecase_go}.yaml` | Modify（硬编码码对齐 + oops→goerror） |
| `internal/scaffold/infra/infra.go`（goGetDeps） | Modify（+ go-common） |
| `README.md/.zh-CN.md`、`docs/examples*.md` | Modify（go-tools/1.26.5/Responder） |
| `.github/workflows/` 或 `scripts/smoke.sh` 或新 Go 测试 | Modify/Create（e2e 编译检查） |

---

## Task 1: 修复 CRITICAL 编译错误（errcode_go.yaml + rpcerror.yaml + golden）

**Files:**
- Modify: `internal/assets/_data/hertz/hertz-template/errcode_go.yaml`
- Modify: `internal/assets/_data/kitex/kitex-template/rpcerror.yaml`
- Regenerate: 含这两个文件的 golden 副本（`grep -rl "goerror.CodeSystem\|goerror.CodePermissionDenied" internal/scaffold/*/testdata/`）

- [ ] **Step 1: 跑 golden 基线**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... ./internal/scaffold/bff/... -count=1`
Expected: PASS（改动前基线）

- [ ] **Step 2: 修 errcode_go.yaml**

把 body 改为：

```go
package errcode

import frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"

// Re-export predefined framework error codes from go-framework/error.
var (
    CodeSystem         = frameworkerror.CodeSystem
    CodeParamInvalid   = frameworkerror.CodeParamInvalid
    CodeAuthFailed     = frameworkerror.CodeAuthFailed
    CodeConfigInvalid  = frameworkerror.CodeConfigInvalid
    CodeRPCTimeout     = frameworkerror.CodeRPCTimeout
    CodeRPCUnavailable = frameworkerror.CodeRPCUnavailable
)
```

（同时把文件头注释 `Re-exports go-common/error` 改为 `go-framework/error`。）

- [ ] **Step 3: 修 rpcerror.yaml**

import 块改为：

```go
import (
    "fmt"

    goerror "github.com/byx-darwin/go-tools/go-common/error"
    frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
    "github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
    "github.com/cloudwego/kitex/pkg/kerrors"
)
```

var 块改为：

```go
var (
    CodeInternalError    = frameworkerror.CodeSystem
    CodeNotImplemented   int32 = 10010
    CodePermissionDenied = frameworkerror.CodeAuthFailed
    CodeRPCTimeout       = frameworkerror.CodeRPCTimeout
    CodeConfigInvalid    = frameworkerror.CodeConfigInvalid
)
```

构造改 goerror：`InternalErrorf`/`TimeoutError`/`PermissionDenied` 中 `oops.In("kitex.server")` → `goerror.In("kitex.server")`；移除 `"github.com/samber/oops"` import。`ToBizError`/`BizCode`/`FormatBiz`/`codeFromOops` 不变（若 codeFromOops 不再被用则随生成代码实际情况保留/删除）。

- [ ] **Step 4: 验证模板无残留错误引用**

Run: `! grep -rnE "goerror\.Code(System|ParamInvalid|AuthFailed|ConfigInvalid|RPCTimeout|RPCUnavailable|PermissionDenied)" internal/assets/_data/`（这些常量必须来自 frameworkerror，不能来自 goerror）
Expected: 无输出。

- [ ] **Step 5: 重新生成 golden 并审查**

Run（精确包路径）: `go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... ./internal/scaffold/bff/... -update-golden -count=1`
`git diff internal/scaffold/*/testdata/` 审查：仅 errcode_go.yaml/rpcerror.yaml 副本的 import/常量/oops→goerror 变化。

- [ ] **Step 6: 跑 golden 测试**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... ./internal/scaffold/bff/... -count=1`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/assets/_data/hertz/hertz-template/errcode_go.yaml internal/assets/_data/kitex/kitex-template/rpcerror.yaml internal/scaffold/*/testdata/
git commit -m "fix(scaffold): 修复生成项目 go-tools 错误码引用（go-framework/error + CodePermissionDenied→CodeAuthFailed）"
```

---

## Task 1b: 修通其余生成代码编译/渲染 bug（让 e2e 编译测试转绿）

> 范围扩充（用户确认）：origin/main 基线的 `TestGenerate*Compiles`/渲染测试是红的，除 Task 1 外还有真实编译/渲染 bug，本任务修到这些测试转绿。④⑤⑥ triage：真实缺口则修，测试 bug/环境则另立 Issue 并在报告说明。

**Files（先调查确认）:** kitex `conf.yaml`（配置类型）、hertz `layout.yaml`（redis_shared 空条目）、可能涉及 conf.go 模板 / data.go / next-steps 逻辑

- [ ] **Step 1: 跑全部编译/渲染测试，建立失败清单**

Run: `go test ./internal/scaffold/mono/... -run 'Compiles|Renders|NextSteps|PostGenerate' -count=1 2>&1 | grep -E "FAIL|--- |undefined|EOF|not found" | head -40`

- [ ] **Step 2: 修 kitex 配置类型（确认的 go-tools 适配 bug）**

`internal/assets/_data/kitex/kitex-template/conf.yaml` 引用了不存在的 `kitexconfig.RPCConfig/RegistryConfig/TimeoutConfig`。先读 go-tools 真实 API：`go-framework/config/kitex/server.go`（ServerConfig 字段：RPC RPCOption / Limit LimitOption / Timeout ServerTimeout 等）与 `client.go`（ClientConfig/ResolverOption/ClientTimeout 等）。把模板的 `RPCConfig{...}/RegistryConfig{...}/TimeoutConfig{...}` 重写为 go-framework/config/kitex 的真实类型与字段（注册/发现可能合并进 RPCOption/ResolverOption）。逐字段对照，确保生成的 conf.go 可编译且语义保留。

- [ ] **Step 3: 修 hertz redis_shared.go 空文件**

`internal/assets/_data/hertz/layout.yaml:1807` 的 `internal/base/data/redis_shared.go` 条目 body 为空 → 生成空文件编译失败。判断意图：redis_shared 内容已在 `hertz/optional/redis_shared.go`（可选 add-on），则该 layout 空条目应**删除**（redis_shared 由 `ncgo add infra redis` 提供，不默认生成）；若应默认生成则填充正确内容。按调查结果处理，并同步 redis_shared_test.go 条目。

- [ ] **Step 4: triage ④⑤⑥（sqlc/db 时序、conf.yaml redis 块、next-steps 断言）**

逐个判定根因：
- with-database `internal/db/gen` 找不到：是否测试未先 `make sqlc`（CLAUDE.md 载 Hertz WithDatabase 须先 make sqlc）→ 若测试时序 bug，记录另立 Issue；若生成流程真缺，修。
- conf.yaml 缺 redis 块：是渲染 bug 还是测试期望问题。
- next-steps 断言：是 goGetDeps/next-steps 逻辑 bug 还是测试期望过时。
真实缺口→修；测试 bug/环境→记录另立 Issue（不静默 skip 测试）。

- [ ] **Step 5: 重新生成受影响 golden、跑编译/渲染测试至绿（或仅剩已记录的测试/环境类失败）、审查、提交**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... -update-golden -count=1`（按需）；`go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... -count=1`
```bash
git add internal/assets/_data/ internal/scaffold/
git commit -m "fix(scaffold): 修通生成项目编译/渲染 bug（kitex 配置类型 + redis_shared 空文件等）"
```

---

## Task 2: 生成 go.mod 升 go 1.26.5 + require go-tools

**Files:**
- Modify: `internal/assets/_data/hertz/layout.yaml`（go.mod 条目，约 82–88 行）
- Modify: kitex go.mod 预写（`internal/scaffold/mono/` + `internal/scaffold/shared/files.go` 或对应位置）
- Regenerate: 含 go.mod 的 golden（若有）

- [ ] **Step 1: 改 hertz go.mod 静态模板**

```yaml
  - path: go.mod
    delims: ["{{", "}}"]
    body: |-
      module {{.GoModule}}

      go 1.26.5

      require (
          github.com/byx-darwin/go-tools/go-common v0.1.0
          github.com/byx-darwin/go-tools/go-framework v0.1.0
      )
```

- [ ] **Step 2: kitex go.mod 钉版本（预写 + initGoMod 短路）**

调查 origin/main 的 kitex go.mod 生成流程：`grep -rn "go mod init\|initGoMod\|go.mod" internal/scaffold/mono/ internal/scaffold/rpc/ internal/scaffold/shared/ internal/exec/`。在 `mono.Generate` 实际调用 kitex 之前预写一个含 `go 1.26.5` + `require go-common/go-framework v0.1.0` 的 go.mod，使 kitex 工具的 `go mod init` 因「go.mod 已存在」短路而原样复用（旧分支 `feat/6-go-tools-error-migration` 的 commit 23fc514 已验证此法，可 `git show 23fc514` 参考）。若 origin/main 流程不同导致不可行，记录并退而用 `go mod edit -require` 后处理或 tidy（在报告标注）。

- [ ] **Step 3: 重新生成相关 golden、审查、跑测试**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... ./internal/scaffold/bff/... -update-golden -count=1 && go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... ./internal/scaffold/bff/... -count=1`
审查 go.mod 相关 diff。

- [ ] **Step 4: 提交**

```bash
git add internal/assets/_data/hertz/layout.yaml internal/scaffold/
git commit -m "feat(scaffold): 生成 go.mod 升 go 1.26.5 并 require go-common/go-framework v0.1.0"
```

---

## Task 3: Docker 基础镜像升 golang:1.26.5

**Files:**
- Modify: `internal/scaffold/shared/container.go`（第 65、434 行 `golang:1.22-alpine`）
- Modify: `internal/assets/_data/hertz/Dockerfile.vegeta`（第 1 行 `golang:1.22`）
- Modify: 各 testdata Dockerfile + `internal/mcp/demo/Dockerfile`

- [ ] **Step 1: 改源**

`grep -rn "golang:1.22" internal/ | grep -v testdata` 找全源（container.go×2 alpine、Dockerfile.vegeta 裸版）：`golang:1.22-alpine`→`golang:1.26.5-alpine`，`golang:1.22`→`golang:1.26.5`。

- [ ] **Step 2: 重新生成受影响 golden（container.go 影响 mono/bff/rpc/infra Dockerfile）并审查**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... ./internal/scaffold/bff/... ./internal/scaffold/infra/... -update-golden -count=1`
`git diff` 审查：仅 Dockerfile 基础镜像行变化。`internal/mcp/demo/Dockerfile` 若非 golden 直接改。

- [ ] **Step 3: 跑测试 + 提交**

Run: `go test ./internal/scaffold/... -count=1`（精确包，避开未注册 -update-golden 的包）
```bash
git add internal/scaffold/shared/container.go internal/assets/_data/hertz/Dockerfile.vegeta internal/scaffold/*/testdata/ internal/mcp/demo/Dockerfile
git commit -m "fix(scaffold): 生成项目 Docker 基础镜像升至 golang:1.26.5"
```

---

## Task 4: optional add-on oops→goerror + 码对齐 + HTTP 注册

**Files:**
- Modify: `internal/assets/_data/hertz/optional/{redis,kafka,es,clickhouse,observability_logging}.go`
- Modify: `internal/assets/_data/kitex/optional/{registry_etcd,observability_logging}.go`
- Regenerate: `internal/scaffold/infra/testdata/`（及其它含 add-on 的 golden）

- [ ] **Step 1: 列出所有 add-on 的 oops/码**

Run: `grep -rnE "oops\.(In|Code|AsOops)|Code\([0-9]+\)|Code\(\"" internal/assets/_data/hertz/optional/ internal/assets/_data/kitex/optional/ internal/assets/_data/optional/`

- [ ] **Step 2: 按 Global Constraints + 「旧 scaffold 码→处理」表迁移**

- 数值码：`10308→frameworkerror.CodeConfigInvalid`、`10301→frameworkerror.CodeRPCUnavailable`、`10000→frameworkerror.CodeSystem`；`10303/10304/10306`（database/cache/search unavailable）→ 项目段码（40503/40504/40506）并在该 add-on 生成包 `init()` 调 `goerror.RegisterHTTPStatuses(map[int]int{40503:503,40504:503,40506:503})`（注意 RegisterHTTPStatuses 重复注册 panic——同一码只注册一次，可放在共享 add-on 的一个 init）。
- 字符串码（kitex registry `"registry_config_invalid"`）：保持字符串码，仅 `oops.In`→`goerror.In`。
- `oops.In/Code/AsOops`→`goerror.In/Code/AsOopsError`；import `samber/oops`→goerror（必要时保留 frameworkerror）。
- add-on 文件头「Required dependency」注释 `go get samber/oops` → `go get github.com/byx-darwin/go-tools/go-common`（及 go-framework 若用 frameworkerror）。

- [ ] **Step 3: 验证无残留 + 重新生成 infra golden + 审查 + 跑测试**

Run: `! grep -rnE "oops\.(In|Code|AsOops)\(" internal/assets/_data/hertz/optional/ internal/assets/_data/kitex/optional/ internal/assets/_data/optional/`
Run: `go test ./internal/scaffold/infra/... -update-golden -count=1 && go test ./internal/scaffold/infra/... -count=1`
审查 diff。

- [ ] **Step 4: 提交**

```bash
git add internal/assets/_data/hertz/optional/ internal/assets/_data/kitex/optional/ internal/assets/_data/optional/ internal/scaffold/infra/testdata/
git commit -m "feat(scaffold): optional add-on 迁移 goerror 并对齐错误码 + 注册 HTTP 状态"
```

---

## Task 5: kitex client/conf/repository/usecase 硬编码码对齐 + oops→goerror

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/{client,conf,repository,usecase}.yaml`
- Modify: `internal/assets/_data/hertz/hertz-template/{repository_go,usecase_go}.yaml`
- Regenerate: 受影响 golden

- [ ] **Step 1: 列出硬编码码与 oops**

Run: `grep -rnE "oops\.(In|Code)|Code\([0-9]+\)" internal/assets/_data/kitex/kitex-template/{client,conf,repository,usecase}.yaml internal/assets/_data/hertz/hertz-template/{repository_go,usecase_go}.yaml`

- [ ] **Step 2: 按映射对齐**

- `Code(10000)`（repository db tx failed）→ `frameworkerror.CodeSystem`（或生成包 errcode/rpcerror 的 CodeSystem/CodeInternalError）。
- `Code(10308)`（config）→ `frameworkerror.CodeConfigInvalid`；`Code(10301)`（client rpc）→ `frameworkerror.CodeRPCUnavailable`。
- `oops.In`→`goerror.In`；import 调整。
- `usecase` 的 `CodeNotImplemented`（10010）：保留为占位（非编译错误），或在 kitex 用 rpcerror.CodeNotImplemented / hertz 用 errcode 对应常量；注释说明 10010 与 go-framework CodeRPCUnavailable 同值的语义取舍。

- [ ] **Step 3: 验证 + 重新生成 golden + 审查 + 跑测试 + 提交**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... -update-golden -count=1 && go test ./internal/scaffold/mono/... ./internal/scaffold/rpc/... -count=1`
```bash
git add internal/assets/_data/kitex/kitex-template/ internal/assets/_data/hertz/hertz-template/ internal/scaffold/*/testdata/
git commit -m "feat(scaffold): kitex/hertz 模板硬编码错误码对齐 go-framework + oops→goerror"
```

---

## Task 6: infra goGetDeps 增加 go-common

**Files:** Modify `internal/scaffold/infra/infra.go`（goGetDeps，约 62–72 行）+ Test

- [ ] **Step 1: 写失败测试**

在 `internal/scaffold/infra/infra_test.go` 加：

```go
func TestGoGetDepsIncludeGoCommon(t *testing.T) {
    for _, kind := range []string{KindRedis, KindKafka, KindES, KindClickHouse, KindRegistryEtcd, KindObservabilityLog} {
        deps := goGetDeps[kind]
        found := false
        for _, d := range deps {
            if d == "github.com/byx-darwin/go-tools/go-common" {
                found = true
            }
        }
        if !found {
            t.Errorf("goGetDeps[%s] missing go-common: %v", kind, deps)
        }
    }
}
```

- [ ] **Step 2: 跑测试确认 FAIL，再给 goGetDeps 各 add-on 追加 `"github.com/byx-darwin/go-tools/go-common"`**（保留 oops；用 frameworkerror 的 add-on 可再追加 go-framework）

- [ ] **Step 3: 跑测试确认 PASS + 提交**

```bash
go test ./internal/scaffold/infra/... -count=1
git add internal/scaffold/infra/infra.go internal/scaffold/infra/infra_test.go
git commit -m "feat(scaffold): infra add-on 下一步依赖补充 go-common"
```

---

## Task 7: 文档（README / examples，中英对齐）

**Files:** `README.md`/`README.zh-CN.md`/`docs/examples.md`/`docs/examples.zh-CN.md`

- [ ] **Step 1: 先 Read 现状，增量补充**

记录：生成项目依赖 go-tools v0.1.0（go-common + go-framework，go-middleware 条件）；生成项目需 **Go 1.26.5**（ncgo CLI 仍 Go 1.25+，明确区分）；响应层用 `go-framework/hertz.Responder`、错误码 re-export `go-framework/error`、日志 `go-common/log`；Docker 基础镜像 golang:1.26.5。

- [ ] **Step 2: EN/ZH 对齐；markdown 格式检查（pre-commit 文件钩子）**

- [ ] **Step 3: 提交**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs: 生成项目对接 go-tools v0.1.0（Go 1.26.5 + Responder + 错误码）"
```

---

## Task 8: 新增 e2e「生成项目可编译」检查（堵盲区）

**Files:** Create/Modify（优先一个 Go 测试或 smoke.sh 段；CI workflow 可选）

- [ ] **Step 1: 实现 e2e 编译检查**

方案 A（推荐，Go 测试，可用 build tag 或 `-e2e` flag 门控以免拖慢常规测试）：生成一个 hertz mono 项目到 t.TempDir()，`go mod tidy`（GOTOOLCHAIN=auto，需网络解析 go-tools v0.1.0），`go build ./...`，断言成功。kitex 同理（若 kitex 工具链可用，否则 skip 并记录）。
方案 B（smoke.sh 段）：在 smoke.sh 末尾加「生成 demo → go mod tidy → go build」。
需 go 1.26.5 工具链 + proxy 网络；环境不具备时 skip 并明确标注（不静默通过）。

- [ ] **Step 2: 本地跑通该检查（hertz 至少）**

Run（示例）: `go test ./internal/scaffold/mono/... -run TestGeneratedProjectCompiles -count=1`（按实际命名）
Expected: hertz 生成项目 `go build ./...` 成功（验证 Task 1 的编译修复确实生效）。

- [ ] **Step 3: 提交**

```bash
git add <新测试文件 或 scripts/smoke.sh 或 .github/workflows/>
git commit -m "test(scaffold): 新增生成项目可编译 e2e 检查（堵 golden 纯文本盲区）"
```

---

## Task 9: 全量验证 + PR 准备

- [ ] **Step 1: 全量验证链**

Run: `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`
Expected: 全绿。

- [ ] **Step 2: gofmt + 复核 golden diff 仅预期变化**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`（无输出）；`git diff --stat origin/main...HEAD` 抽查。

- [ ] **Step 3: 创建 PR（orchestrator 执行，body 含 `Closes #6`）**

PR 描述含：修通的 2 个编译 bug、补全的缺口清单、行为/版本变更（Go 1.26.5、Docker、go.mod require）、e2e 检查结果、与 Issue #7–#10 的关系重估、已知后续项（kitex --no-generate 手动流 tidy 漂移、CodeNotImplemented=10010 语义取舍）。

## 验证顺序

1. 聚焦：`go test ./internal/scaffold/infra/... -run TestGoGetDepsIncludeGoCommon`
2. 包级：mono/rpc/bff/infra golden 测试
3. e2e：生成项目 `go build`（Task 8）
4. 全量：`go test ./... -count=1` + `go vet` + `go build`
5. smoke：`./scripts/smoke.sh`

## 风险

- kitex go.mod 钉版本依赖 initGoMod 短路，需在 origin/main 流程验证（Task 2 Step 2，旧分支 23fc514 可参考）。
- e2e 编译检查需 go 1.26.5 + 网络；CI 不具备则落为本地/门控检查并标注。
- optional 项目段码（40503/40504/40506）须注册 HTTP 映射且避免重复注册 panic。
- golden diff 大，逐提交审查避免误 bless。

# 设计文档：ncgo ai init claude 架构感知

## 1. 背景与目标

### 问题

当前 `ncgo ai init claude` 对所有项目（Hertz HTTP、Kitex RPC、微工作区）生成完全相同的 `.claude/` 文件，不区分服务架构。而 `ai sync` 已通过 `Service.Kind` 选择了架构特定的设计文档。这导致 `.claude/rules/go.md` 缺少 HTTP middleware、RPC interceptor 等架构特定的指导，agents 也不具备框架特定的知识。

### 目标

1. `ai init claude` 从 manifest 自动检测架构，注入架构特定规则到 `.claude/rules/go.md`
2. 微工作区为每个服务创建 `.claude/services/<name>/` 子目录，存放架构特定文件
3. `ncgo add` 执行后自动增量更新 `.claude/` 内容
4. agents (implementer/reviewer/debugger) 通过统一引用段落按需读取子目录规则文件

## 2. 设计决策

### 2.1 分层注入架构内容

采用"根级通用 + 子目录特定"的分层方案：

| 层级 | 内容 |
|------|------|
| 根级 `.claude/` | 通用 Go 规则、通用 agent 定义、skills、commands |
| `.claude/services/<name>/` | 架构特定的 rules、reviewer-checklist、implementer-guide、debugger-playbook |

优势：
- 根文件保持一份，不用为每个架构维护多份副本
- agent 按需读取子目录，不强制依赖
- 微工作区每个服务独立，互不干扰

### 2.2 单体 vs 微工作区的差异

| 维度 | 单体服务 | 微工作区 |
|------|----------|----------|
| 架构规则注入 | 直接注入到 `.claude/rules/go.md` | 每个服务子目录单独存放 `rules.md` |
| 根级 go.md | 有架构特定规则 | 保持通用 |
| services/ 子目录 | 不创建 | 每个服务一个子目录 |

### 2.3 ncgo add 自动更新

add 命令完成后自动：
- 检查 `.claude/` 是否存在，不存在则 bootstrap 最小结构
- 微工作区：增量更新 `.claude/README.md` 服务列表
- 为新服务创建 `.claude/services/<name>/` 子目录
- 不覆盖已存在的文件（非破坏性）

## 3. 文件目录结构

### 3.1 运行时产物（单体服务）

```
.claude/
├── README.md                          ← 根级，包含服务形状指引
├── rules/
│   ├── agent-engineering.md           ← 通用
│   └── go.md                          ← 注入 Hertz 或 Kitex 规则
├── agents/
│   ├── implementer.md                 ← 通用 + 子目录引用
│   ├── reviewer.md                    ← 通用 + 子目录引用
│   └── debugger.md                    ← 通用 + 子目录引用
├── skills/                            ← 通用
├── commands/                          ← 通用
└── hooks/                             ← 通用
```

### 3.2 运行时产物（微工作区）

```
.claude/
├── README.md                          ← 根级，列出所有服务及类型
├── rules/
│   └── go.md                          ← 保持通用
├── agents/                            ← 通用 + 子目录引用
├── services/                          ← 新：按服务子目录
│   ├── user-api/                      ← Hertz 服务
│   │   ├── rules.md
│   │   ├── reviewer-checklist.md
│   │   ├── implementer-guide.md
│   │   └── debugger-playbook.md
│   └── user-rpc/                      ← Kitex 服务
│       ├── rules.md
│       ├── reviewer-checklist.md
│       ├── implementer-guide.md
│       └── debugger-playbook.md
├── skills/
├── commands/
└── hooks/
```

### 3.3 嵌入式模板资源

```
internal/assets/_data/claude/
├── arch/
│   ├── hertz/
│   │   ├── rules.md
│   │   ├── reviewer-checklist.md
│   │   ├── implementer-guide.md
│   │   └── debugger-playbook.md
│   └── kitex/
│       ├── rules.md
│       ├── reviewer-checklist.md
│       ├── implementer-guide.md
│       └── debugger-playbook.md
├── rules/
│   ├── go.md                          ← 添加 {{ARCHITECTURE_RULES}}
│   └── agent-engineering.md
├── agents/
│   ├── implementer.md                 ← 添加 {{ARCH_HINT}}
│   ├── reviewer.md                    ← 添加 {{ARCH_HINT}}
│   └── debugger.md                    ← 添加 {{ARCH_HINT}}
├── skills/
├── commands/
└── hooks/
```

## 4. 架构规则内容

### 4.1 Hertz 规则片段（注入到 go.md 或 services/<name>/rules.md）

- Hertz 使用 `*app.RequestContext`，不能跨 handler 边界传递
- middleware（`internal/pkg/middleware/`）处理横切关注点：auth、rate-limit、CORS
- 路由由 `hz` 从 IDL 生成，注册在 `router.GeneratedRegister`
- 使用 `response.OK(c, resp)` / `response.Err(c, err)` 统一响应
- 错误码为 5 位：`1xxxx` 用于请求/auth/rate-limit 错误

### 4.2 Kitex 规则片段

- 使用 `context.Context` 通过 RPC 调用链传递
- interceptors（`internal/pkg/interceptor/`）处理横切关注点
- 错误通过 `rpcerror.ToBizError(err)` 映射为 `kitex.BizStatusError`
- 生成客户端在 `pkg/client/<service>/`，由 adapter 消费
- 不要在 handler 中间创建 `context.Background()`

## 5. Agent 引用段落

注入到 `{{ARCH_HINT}}` 占位符的通用引用：

```markdown
## Service-Specific Guidance

When working on a service that has a `.claude/services/<service-name>/` directory,
read the relevant files there before making changes:

- `rules.md` — architecture-specific Go rules
- `reviewer-checklist.md` — service-specific review checklist
- `implementer-guide.md` — implementation patterns for this architecture
- `debugger-playbook.md` — framework-specific troubleshooting
```

单体服务时：这段内容替换为通用指引（不引用子目录，因为不存在）。

微工作区时：保留这段内容，agent 运行时按需读取。

## 6. 实现文件清单

| 文件 | 变更 |
|------|------|
| `internal/ai/init.go` | 新增 `initContext`、`detectInitContext()`、架构规则注入函数、微工作区服务子目录生成 |
| `internal/assets/_data/claude/rules/go.md` | 添加 `{{ARCHITECTURE_RULES}}` 占位符 |
| `internal/assets/_data/claude/agents/reviewer.md` | 添加 `{{ARCH_HINT}}` 占位符 |
| `internal/assets/_data/claude/agents/implementer.md` | 添加 `{{ARCH_HINT}}` 占位符 |
| `internal/assets/_data/claude/agents/debugger.md` | 添加 `{{ARCH_HINT}}` 占位符 |
| `internal/cli/add.go` | add 成功后自动更新 `.claude/` 目录 |
| `internal/ai/init_test.go` | 新增 5 个测试 |

## 7. 新增测试

| 测试 | 验证内容 |
|------|----------|
| `TestInitClaudeGoMdIncludesHertzRules` | Hertz manifest → go.md 包含 Hertz 规则 |
| `TestInitClaudeGoMdIncludesKitexRules` | Kitex manifest → go.md 包含 Kitex 规则 |
| `TestInitClaudeMicroWorkspaceCreatesServiceDirs` | 微工作区 → 每个服务有子目录 |
| `TestInitClaudeUnknownShapeHasEmptyArchRules` | 无 manifest → 占位符为空 |
| `TestAddServiceUpdatesClaudeDir` | add service → 自动更新 `.claude/` |

## 8. 验证

1. `go test ./internal/ai/... -count=1`
2. `go test ./internal/cli/... -count=1`
3. `go build ./... && go vet ./...`

# 官方模版 Registry 闭环 + export 修复 — Design Spec

- **Date**: 2026-08-08
- **Workflow**: wf-2026-08-08-001（gf-workflow full mode）
- **Status**: Approved（brainstorming 阶段用户批准）
- **Related specs**: `specs/archive/specs/2026-05-14-hertz-code-template-export-design.md`、`specs/archive/specs/2026-05-15-kitex-code-template-export-design.md`

## 1. Background — 审计结论

用户诉求：分析项目"反向生产模版"功能是否实现。多角色审计结论：

**功能主体已实现、闭环基本完整**（2026-05-15 commit 波次落地）：

| 面 | 状态 |
|---|---|
| 核心逻辑 `template.Export`（HertzRules/KitexRules、排除 `internal/pb/`+`kitex_gen/`、`{{.Module}}` 替换、PascalCase 服务名替换、loop_service 路径参数化、Makefile 导出） | ✅ |
| CLI `ncgo export templates`（`--root`/`--kind`，manifest 自动检测 kind） | ✅ |
| Hertz Apply 侧（`template.Apply`，`ncgo new` 中 `hz new` 后自动 overlay；Kitex 按设计由 `kitex -template-dir` 应用） | ✅ |
| MCP `ncgo_export_templates`（text/json 双输出 + 结构化字段 outputDir/kind/templates + sandbox 校验） | ✅ |
| 测试（12 单元 + 2 集成 export→apply round trip + 3 CLI） | ✅ |
| 文档（README EN/ZH 命令表、docs/examples EN/ZH §6） | ✅ |

**但存在 5 项与归档设计的差异/缺陷**：

1. proto ServiceInfo 变量替换未接线（`Export` 不做 proto 解析）
2. body/import 路径中小写服务名未变量化（`TestReplaceServiceName_ImportPath` 注释确认此为已知局限）→ **换服务名生成必编译失败**
3. 无 golden 快照测试（设计 Testing §3）
4. `scripts/smoke.sh` 无 export 端到端场景（设计 Testing §4）
5. MCP `ncgo_export_templates` 描述（"Export embedded scaffold templates to disk for customization"）与实际行为（从现有项目反向导出）不符

**更大的缺口**：归档设计明确 `ncgo new --kind kitex` 消费导出模板"需手动替换内置模板，或由**后续自动检测机制**处理"——该机制从未实现。`ncgo new` 的模板目录（`writeKitexTemplate`/`copyHertzTemplateYAML`）永远从内置 assets 覆盖写入，没有读入外部模板的入口。

## 2. 用户模型（关键澄清）

用户的使用模型是 **"换名复制"**（renamed clone），比归档设计更简单：

- 模板 = 成熟项目的快照；模板内容（handler、usecase、方法代码）就是基础项目的起始代码
- `ncgo new` 消费模板 = 原样沿用模板内容，只替换 go module 路径与服务名
- 目的：像 rule-center preset 一样**批量生产统一的基础项目**，且模板由**官方集中审核管理**（固定 registry 仓库，分支/PR + 审核合入）

由此推导：

- ServiceInfo 循环化（差异 1）**不需要**——模板方法代码即预期产物，新项目方法与模板天然一致（IDL 随模板一起走）
- 消费时方法集校验也**不需要**
- 但 **IDL 必须纳入模板范围**：`kitex_gen/`、`internal/pb/` 被排除导出，新项目编译必须重跑生成器，而生成器需要 IDL；若 IDL 不在模板包内，占位 IDL 的方法集会与模板 body 冲突

## 3. Design Decisions（澄清过程记录）

| 决策点 | 选择 | 备选（否决原因） |
|---|---|---|
| 交付目标 | 全量修复 + 消费闭环 + registry | 仅审计（用户明确要全量修复） |
| 消费交互形态 | `--template-dir` flag + `--template <name>` | 命名 preset 注册（状态管理重）；自动检测（隐式行为难预测） |
| 适用命令 | 仅 `ncgo new`（mono，hertz+kitex） | `add rpc`/`add bff`（micro 上下文复杂，预留后续迭代） |
| registry 衔接 | 内置客户端命令（`ncgo template list/pull`） | 纯 git + --template-dir（体验弱）；内置 preset 发布（模板更新绑死 ncgo 发版） |
| 方法集策略 | 换名复制模型，不做循环化/校验 | ServiceInfo 循环化（实现重、已导出模板不兼容）；校验+降级（用户模型下无必要） |
| IDL | 纳入导出/模板范围 | 用户生成时自带 IDL（与换名复制模型冲突） |

## 4. 总体闭环

```
【生产侧】成熟项目
   ncgo export templates           ← 修复：含 IDL、小写服务名变量化
        ▼
   template/<kind>-template/*.yaml + idl/*.proto（模版包）
        ▼
【治理侧】官方模版 registry 仓库（独立 git 仓库，本设计只定规范）
   贡献者：创建分支 → PR      官方：审核 → 合入
        ▼
【消费侧】
   ncgo template pull <name>       ← 新增：拉到本地缓存
   ncgo new my-svc --template <name>   （或 --template-dir <本地路径>）
        ▼
   基础项目 = 模版换名复制（module + 服务名），方法/结构全部沿用
   生成器重跑（kitex -template-dir / hz + Apply overlay）→ kitex_gen/pb 产物一致
```

## 5. 模版包规范（registry 目录结构）

```
<registry-repo>/
├── base-kitex/                      # 模版名 = 目录名
│   ├── template.yaml                # 元信息
│   ├── kitex-template/*.yaml        # ncgo export 产物（原样）
│   ├── idl/*.proto                  # 变量化后的 IDL
│   └── README.md
└── base-hertz/
    ├── template.yaml
    ├── hertz-template/*.yaml
    ├── idl/*.proto
    └── README.md
```

`template.yaml` 元信息：

```yaml
name: base-kitex            # 与目录名一致
kind: kitex                 # hertz | kitex，消费时与 --kind 校验
description: 官方 Kitex 基础服务模版（标准分层 + 健康检查）
version: 1                  # 模版包修订号（registry 维护者递增）
```

- registry 仓库本身的创建、CI、审核规则属于基础设施，不在本次代码范围内；设计交付规范文档（README/docs 章节）
- `ncgo export templates` 的产物目录结构与模版包对齐，贡献者只需补充 `template.yaml` + `README.md`

## 6. Export 侧修改

| 修改 | 说明 |
|---|---|
| IDL 纳入导出 | 扫描 `idl/**/*.proto`（排除生成物），写入模版包 `idl/`；proto 内服务名变量化为 `{{.ServiceName}}`（如 `service UserRpc {` → `service {{.ServiceName}} {`），与现有渲染器变量语法一致；消息名中与模块路径相关的引用随 module 替换 |
| 小写服务名替换 | body 与 import 路径中的小写服务名（`userrpc`/`user-rpc` 去横线形态）→ `{{ToLower .ServiceName}}`；修复换名生成编译失败缺陷 |
| ServiceInfo 循环化 | **不做**；在模板文档中说明"模板方法代码即基础代码，新项目沿用"的换名复制语义 |

导出产物保持向后兼容：既有 `template/<kind>-template/*.yaml` 结构不变，新增 `idl/` 输出目录。

## 7. 消费侧：`ncgo new --template-dir <dir>`

- 适用：`ncgo new`（mono，hertz + kitex）；`--template-dir` 指向模版包目录
- 流程：
  1. 读 `<dir>/template.yaml`，校验 `kind` 与 `--kind`（或默认 hertz）匹配；不匹配报清晰错误
  2. 模版包的 `<kind>-template/` 与 `idl/` **替换**内置 assets 写入（`writeKitexTemplate`/`copyHertzTemplateYAML`/`writeIDLPlaceholder` 增加"外部模版包来源"参数）
  3. 其余流程不变：kitex `-template-dir template/kitex-template` / `hz new` + `template.Apply` overlay → 生成器产物与模板 body 天然一致
- 缺省行为（不传 flag）完全不变：内置模板 + IDL 占位符
- 互斥约束：`--preset` 与 `--template-dir` 互斥；`--template` 与 `--template-dir` 互斥（同时传入报清晰错误），避免多模板来源混淆

## 8. Registry 客户端

| 命令 | 行为 |
|---|---|
| `ncgo template list` | 列出 registry 中的模版（读 registry 仓库根目录下各模版包的 `template.yaml`），输出 name/kind/description |
| `ncgo template pull <name>` | 拉取模版包到本地缓存（`os.UserCacheDir()/ncgo/templates/<name>`），存在则更新 |
| `ncgo new --template <name>` | 等价于 `--template-dir <缓存>/<name>`；缓存缺失时提示先 `ncgo template pull` |

- registry 地址：内置默认官方仓库 URL；`--registry` flag / `NCGO_REGISTRY` 环境变量覆盖
- 版本策略（本次）：pull 取 registry 默认分支最新；不做版本锁定/tag 选择（后续迭代）
- 网络失败行为：清晰报错，不静默降级
- MCP 双轨：`ncgo_template_list` / `ncgo_template_pull` 工具，遵循仓库惯例（`content[0].text` 可读文本 + 顶层结构化字段 + isError 语义）

## 9. 其余审计差异处理

| 差异 | 处理 |
|---|---|
| MCP `ncgo_export_templates` 描述错误 | 修正为"Export code templates from an existing ncgo project to template/<kind>-template/"（与 CLI Short 一致） |
| golden 测试缺失 | 补 export 输出快照测试（固定输入项目 → 模版包快照，沿用仓库 testdata/golden 惯例） |
| smoke 测试缺失 | `scripts/smoke.sh` 增加 export→new 闭环场景（用 `--no-generate` 避免依赖本机 hz/kitex） |

## 10. 测试与文档

**测试**（契约面变化必须同步）：

- export：IDL 导出 + 服务名变量化、小写替换（含 import 路径）单元测试；更新 golden 快照
- CLI：`ncgo new --template-dir` 匹配/不匹配/与 preset 互斥；`ncgo template list/pull`（registry 用本地 fixture 仓库，不依赖网络）
- MCP：`ncgo_template_list`/`ncgo_template_pull` 的 InputSchema、`content[0].text`、顶层字段、错误响应
- 集成：export（含 IDL）→ new --template-dir round trip（生成侧用 `--no-generate` 或 mock runner）
- smoke：export→new 闭环最小场景

**文档**（EN/ZH 对齐）：

- README.md / README.zh-CN.md：命令表新增 `ncgo template list/pull`；`ncgo new` flag 说明
- docs/examples.md / docs/examples.zh-CN.md：新增章节——模版包规范、registry 贡献与审核流程（分支 → PR → 官方审核）、`ncgo template` 与 `--template`/`--template-dir` 用法
- 归档设计文档保持原样（历史记录），本文档记录差异降级决策

## 11. 范围边界（本次不做）

- `ncgo add rpc` / `ncgo add bff` 模版支持（接口预留，后续迭代）
- ServiceInfo 循环化（换名复制模型下不需要）
- 模版版本锁定 / tag / commit 级引用
- registry 仓库的创建、CI、审核自动化
- micro 工作区级别的模版

## 12. Risks

| 风险 | 缓解 |
|---|---|
| 小写服务名替换误伤（如代码中出现与服务名同形的业务词） | 限定替换上下文：import 路径段、目录路径段；body 中标识符级替换沿用现有 PascalCase 策略扩展；golden 测试锁定行为 |
| IDL 变量化破坏 proto 语法 | 渲染后跑 protolint/protoc 校验纳入测试 |
| registry 网络不可用 | pull/list 失败报清晰错误；`--template-dir` 本地路径不受影响 |
| 模版包结构与内置管线耦合 | 消费入口收敛在 mono 包的模板来源参数，单测覆盖两种来源 |
| 既有导出用户（无 idl/）消费行为 | `--template-dir` 对缺少 idl/ 的模版包回退到 IDL 占位符并提示（向后兼容） |

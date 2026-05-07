# Micro 工作区详细设计

读者: ncgo 维护者及读取或刷新 `ncgo new --mode micro` 生成仓库根目录 AI 上下文的 Agent。

服务级架构细节请分别参考内置的 `docs/hertz/` 与 `docs/kitex/` 设计文档。

动态限流行为属于具体服务模板而非 micro 工作区 profile 本身。当前独立专题仅适用于
Hertz HTTP 服务:
[`docs/hertz/rate-limit-dynamic-design.zh-CN.md`](../hertz/rate-limit-dynamic-design.zh-CN.md)。

## 1. 总览

micro 工作区 profile 对应 `ncgo new --mode micro` 创建的仓库根目录。根目录围绕
`ncgo.workspace`、共享 `.claude` / `.cursor` 上下文、仓库级 hooks，以及
`ncgo doctor --root .`、`ncgo protolint --root .` 这类聚合命令展开。

可部署单元位于 `services/`。每个生成的 BFF 或 RPC 服务都保留自己的
`.ncgo/manifest.yaml`、module path、IDL 与生成器模板树。工作区级 AI 上下文应描述
仓库形状与服务清单，而不是替代服务级上下文。

## 2. 工作区职责

- 根级元数据保存在 `ncgo.workspace`
- 服务通过 `name`、`kind`、`dir` 登记在工作区中
- hand-authored 的 `.claude/*` 规则应放在工作区根目录
- `AGENTS.local.md` 只应保存工作区根目录自己的本地备注

## 3. 服务职责

- 使用 `ncgo add rpc <name>` 或 `ncgo add bff <name>` 增加服务
- 每个服务保留自己的 `.ncgo/manifest.yaml`
- 需要服务级 AI 上下文时，请执行 `ncgo ai sync --root services/<name>`
- 除非任务明确跨服务，否则大多数改动都应尽量限定在单个服务内

## 4. 校验与 Agent 工作流

- 检查服务清单或聚合 proto 契约时，应在工作区根目录运行聚合发现类命令
- build、test、代码生成与服务特定校验，应优先在目标服务目录内执行，除非任务本身覆盖整个仓库
- 当改动涉及多个服务时，要明确指出每一处变更由哪个服务负责，并尽量分别验证受影响的服务

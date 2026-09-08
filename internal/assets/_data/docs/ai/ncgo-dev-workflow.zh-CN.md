## 用 ncgo 实现一个功能

本项目由 `ncgo` CLI 生成和扩展。按照以下工作流端到端添加新功能。
每个步骤都有程序化契约，因此 AI 代理可以直接驱动它。

### 工作流

1. **添加领域** — `ncgo add domain <name> --root .`
   生成 `internal/usecase/<name>/`、`internal/repository/<name>/` 和
   `internal/base/data/<name>_register.go`，并将领域记录在
   `.ncgo/manifest.yaml` 中。领域名称匹配 `^[a-z][a-z0-9_]{0,62}$`。

2. **添加用例方法** — `ncgo add method <domain>.<Method> --root .`
   在领域用例文件的 `// ncgo:methods:start` 和 `// ncgo:methods:end`
   标记之间插入 `func (u *UseCase) <Method>() error` 桩代码。
   方法名称匹配 `^[A-Z][A-Za-z0-9_]{0,62}$`。

3. **更新 IDL** — 编辑 `idl/<service>.proto`
   当功能暴露新的或变更的接口时，添加或修改 rpc/api 方法定义；
   如果只是内部用例逻辑变更，可跳过此步及下一步。

4. **从 IDL 重新生成代码** — `make update`
   Hertz 运行 `hz update`；Kitex 运行 `kitex` 生成器。从 IDL
   重新生成 handler 桩代码和生成类型（`internal/pb` 或 `kitex_gen`）。
   若功能只改动用例逻辑、不涉及 IDL，可跳过此步及上一步。

5. **把 RPC 桩追加到 usecase.go** — `ncgo add rpc-method --service <name> --rpc <Method> --root .`
   将方法桩追加到顶层 `internal/usecase/<service>/usecase.go`，签名从上一步
   重新生成的 handler 文件中提取。之所以需要这一步，是因为该文件的
   `update_behavior: skip` 意味着生成器永远不会自动往里面追加新方法。不会处理
   项目自有的聚合文件（例如手写的 `composite.go`）——这些仍需人工处理。

6. **实现 handler** — 将生成的 handler 接入第 2 步添加的用例方法，
   遵循 `handler/* → usecase/*` 分层规则（handler 不导入 repo/data）。

7. **重新生成数据库代码** — `make sqlc`
   当服务使用数据库时需要（`cfg.Database.Enabled`）。Kitex 服务在
   `go mod tidy` 之前始终需要此步骤；Hertz 服务仅在启用数据库脚手架时才需要。

8. **验证** — `go build ./... && go vet ./... && go test ./... -count=1`
   每次方法插入后，脚手架必须保持可构建状态。

9. **用 ncgo check 校验** — `ncgo check --root .`
   验证改动内部一致：每个用例都有配对的 `// ncgo:methods:start|end`
   锚点、`manifest.Domains` 与 `internal/usecase/*/` 一致、渲染的 AI 上下文
   声明的 domains 与 manifest 一致。通过退出 `0`，校验失败退出 `1`，命令错误退出 `2`。

10. **刷新 AI 上下文** — `ncgo ai sync --root .`
   重新渲染本项目的 AI 工件（见下文），使代理上下文反映新增的领域和方法。
   sync 后重跑 `ncgo check` 确认过期检查通过。

### 锚点

- `// ncgo:methods:start` / `// ncgo:methods:end` — 方法插入区域，
  位于 `internal/usecase/<domain>/<domain>.go`。不要手动编辑生成的方法；
  使用 `ncgo add method`。
- `// ncgo:wire:domain` — 可选的 `data.Register<Name>` 接线标记。
  当存在时，`ncgo add domain --wire` 会在该处插入注册调用。

### 验证清单

- [ ] `.ncgo/manifest.yaml` 列出了新领域
- [ ] `internal/usecase/<domain>/<domain>.go` 在锚点之间包含新方法
- [ ] 若功能新增/变更接口，`idl/<service>.proto` 已更新且重跑了 `make update`
- [ ] handler 调用了用例方法（handler 不导入 repo/data）
- [ ] `go build ./...` 通过
- [ ] `ncgo check --root .` 退出 0
- [ ] `ncgo ai sync --root .` 完成并报告已写入的托管文件
- [ ] sync 后 `ncgo check --root .` 仍退出 0

### 失败处理

- `ncgo add domain` 失败"已存在" — 该领域已存在；
  直接运行 `ncgo add method` 或使用 `--force`。
- `ncgo add method` 失败"缺少标记" — 用例文件被手动编辑或从未生成；
  使用 `ncgo add domain <name> --force` 重新生成领域。
- `make update` 失败 — 确认 `hz`/`kitex` 已安装并在 `PATH` 中，且 IDL
  文件语法正确；参见项目设计文档 `docs/ncgo/<profile>/design-doc.zh-CN.md`。
- `make sqlc` 失败 — 确认 `sqlc` 已安装且 schema 文件完整；
  参见项目设计文档 `docs/ncgo/<profile>/design-doc.zh-CN.md`。
- `ncgo check` 因 `check.anchor` 退出 1 — 用例丢失了
  `// ncgo:methods:start|end` 标记；用 `ncgo add domain <name> --force` 修复。
- `ncgo check` 因 `check.manifest.consistency` 退出 1 — `manifest.Domains`
  与 `internal/usecase/*/` 漂移；运行 `ncgo add domain` 或修正 manifest。
- `ncgo check` 因 `check.context.stale` 退出 1 — 渲染的 AI 上下文声明的
  domains 与 manifest 不一致；运行 `ncgo ai sync --root .`。
- `ncgo ai sync` 拒绝覆盖 — 文件缺少 `<!-- ncgo:managed -->` 标记；
  仅当你拥有该文件时才使用 `--force`。

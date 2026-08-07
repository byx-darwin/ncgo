## ncgo 项目规则

> 嵌入式设计文档（`docs/ncgo/<profile>/design-doc.*.md`）中的路径是
> **模板内部**路径（例如 `kitex/kitex-template/main.yaml`）；
> 生成项目的实际路径不同。请阅读设计文档以了解本项目的路径映射。

- 不要手动编辑生成的文件，应修复模板或生成器。
- 遵守分层边界：handler → usecase → repository。
- 在 `go mod tidy` 之前运行 `make sqlc`（Kitex 始终需要；Hertz 仅在启用数据库时需要）。
- 通过 `ncgo add method <domain>.<Method>` 添加用例方法，不要手动添加。
- 修改 manifest 或生成的代码后，运行 `ncgo ai sync --root .`。
- 完整工作流：参见 `AGENTS.md` 中的"Implementing a Feature with ncgo"。
- 架构参考：`docs/ncgo/<profile>/design-doc.zh-CN.md`。

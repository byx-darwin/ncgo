# Issue #45 Phase 2 — 交接/恢复点

> 写于 2026-08-07。上下文清除后的恢复指南。

## 当前状态
- main @ f30d8ec（干净基线，仅剩会话前预存文件 .cache/、.claude/skills/、.gitignore）
- Issue #45 Phase 1 已完成并合并：PR #47 (327994f) + golangci 清理 (8d9321e) + 历史文档提交 (f30d8ec)

## 下一步：Issue #45 Phase 2（动态层）
设计草案已在 **`docs/superpowers/specs/2026-08-07-issue-45-ai-agent-ncgo-dev-design.md` §7**：

1. **新增 MCP `ncgo_ai_context`**：go/parser 扫描真实代码返回结构化上下文
   - domains / methods / anchors / consistency
   - 输出：content[0].text 可读 + 顶层结构化字段（与现有 MCP 双输出约定一致）
   - InputSchema：root（必填）
2. **新增 `ncgo check` 命令**：校验 agent 改动，任一失败返回非零退出码
   - 锚点完整（usecase 文件 ncgo:methods:start|end 配对）
   - manifest 一致性（Domains 与实际 internal/usecase/*/ 一致）
   - 上下文过期（AGENTS/CLAUDE 是否落后 manifest）
   - 输出：text/json 结构化报告（类似 ncgo doctor）
   - 与 ncgo_ai_context 共享同一 internal/ai/scan 包

## 开放问题（Phase 2 开工前确认，设计文档 §7.3）
- ncgo check 退出码约定（0 通过 / 1 校验失败 / 2 命令错误）
- --target 是否也用于 ncgo check
- ncgo_ai_context 是否需要缓存

## 工作流约定（本仓库 gf-workflow）
- 流程：/gf-workflow <issue号> 四阶段（clarify→plan→execute→deliver）
- 合同：.cache/workflows/active/<wf-id>.json，归档到 archive/YYYY-MM/
- 模式自动检测：feat:→full，fix:→standard/ fast
- 参考合同：.cache/workflows/archive/2026-08/wf-2026-08-07-002.json（Phase 1 完整记录）

## 恢复动作
1. 为 Phase 2 创建新 Issue（可从设计文档 §7 复制需求）
2. 运行 /gf-workflow <issue号> 启动四阶段
3. 读到本文件即可接续

# ncgo Specifications

设计与规格文档索引。用户指南和示例请参见 `docs/examples.md` 和 `README.md`。

## 目录

| 编号 | 文档 | 语言 | 说明 |
|------|------|------|------|
| 001 | [architecture](001-architecture.md) | EN/ZH | 项目架构概览（包依赖、数据流、模板系统） |
| 002 | [ai-init-claude](002-ai-init-claude.md) | EN/ZH | AI 上下文初始化（`ncgo ai init claude`）设计方案 |
| 003 | [ai-sync-context](003-ai-sync-context.md) | EN/ZH | AI 上下文同步（`ncgo ai sync`）生成方案 |
| 004 | [i18n-system](004-i18n-system.zh-CN.md) ([EN](004-i18n-system.en.md)) | EN/ZH | 国际化系统设计（Agent+工具链混合方案） |
| 005 | [proto-io-system](005-proto-io-system.zh-CN.md) ([EN](005-proto-io-system.en.md)) | EN/ZH | Proto I/O 校验系统设计（规则、SARIF、CLI/MCP） |
| 006 | [canary-release](006-canary-release.zh-CN.md) ([EN](006-canary-release.en.md)) | EN/ZH | 金丝雀发布方案设计 |
| 007 | [observability-logging](007-observability-logging.zh-CN.md) ([EN](007-observability-logging.en.md)) | EN/ZH | 可观测性与日志方案设计 |
| 008 | [release-process](008-release-process.zh-CN.md) | ZH | 发布流程、标签与 Release Notes 模板 |
| 009 | [rate-limit-system](009-rate-limit-e2e-test-plan.md) | EN/ZH | 限流系统 E2E 测试方案与实现审计 |
| — | [prd](prd.md) | EN/ZH | 产品需求文档 |
| — | [roadmap](004-roadmap.zh-CN.md) | ZH | 开发路线图 |
| — | [context-handoff](005-context-handoff.zh-CN.md) | ZH | AI Agent 上下文交接笔记 |

## v1.0 计划与设计

| 编号 | 文档 | 说明 |
|------|------|------|
| 010 | [v1.0-plan](010-v1.0-plan.md) | v1.0 三阶段计划总览 |
| 011 | [orchestrator-design](011-orchestrator-design.md) | CLI/MCP 共享服务层设计 |
| 012 | [wire-ast-migration](012-wire-ast-migration.md) | wire.go 字符串操作 → go/ast 迁移 |
| 013 | [security-hardening](013-security-hardening.md) | 安全加固方案（MCP/路径沙箱/compose/dockerignore） |
| 014 | [interactive-mode](014-interactive-mode.md) | ncgo new 交互模式（bubbletea TUI） |
| 015 | [import-command](015-import-command.md) | ncgo import 反向生成 manifest |
| 016 | [k8s-generation](016-k8s-generation.md) | K8s 部署文件生成（Kustomize overlay） |

## 归档

`archive/` 目录包含历史设计文档和已合并的详细规格：

- `archive/plans/` — AI 生成的实现计划（7 篇）
- `archive/specs/` — 历史设计规格（7 篇）
- `archive/i18n-*.zh-CN.md` — i18n 原始设计文档（6 篇，已合并为 004）
- `archive/proto-io-*.zh-CN.md` — Proto I/O 原始设计文档（4 篇，已合并为 005）

## 用户文档

以下文档面向最终用户，保留在 `docs/` 目录：

- `docs/examples.md` / `docs/examples.zh-CN.md` — CLI/MCP 使用示例
- `README.md` / `README.zh-CN.md` — 项目概览与快速入门
- `CONTRIBUTING.md` / `CONTRIBUTING.zh-CN.md` — 贡献指南

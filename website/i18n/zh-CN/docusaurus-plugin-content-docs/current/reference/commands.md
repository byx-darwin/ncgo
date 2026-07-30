---
sidebar_position: 1
title: 命令参考
---

# 命令参考

顶层命令（来自 `ncgo --help`）：

| 命令 | 说明 |
| --- | --- |
| `ncgo new` | 生成一个新的服务或工作区 |
| `ncgo add` | 向已有项目添加能力（`infra` / `domain` / `rpc` / `bff`） |
| `ncgo import` | 将已有 hz/kitex 项目导入 ncgo |
| `ncgo ai` | AI 协作辅助（同步上下文、初始化 `.claude`） |
| `ncgo doctor` | 诊断宿主工具与当前项目 |
| `ncgo protolint` | 使用 Proto I/O 规则检查 `.proto` 文件 |
| `ncgo i18n` | 以结构化输出检查项目 i18n 资源 |
| `ncgo extract` | 计划或执行 mono 到 micro 的提取 |
| `ncgo export` | 从已有项目导出代码模板 |
| `ncgo upgrade` | 升级项目元数据 |
| `ncgo mcp` | 面向 AI agent 的 MCP server |
| `ncgo test` | 运行项目的生成测试 |
| `ncgo completion` | 生成 shell 自动补全脚本 |
| `ncgo version` | 打印 ncgo、构建与 assets 版本 |

查看每个命令的 flag，请运行 `ncgo <command> --help`。
完整示例请参见 [指南](../guides/new-service.md)。

# ncgo 官网设计文档

- **状态**：待用户确认
- **日期**：2026-07-30
- **工作流**：`gitflow-workflow` · 合同 `wf-2026-07-29-001` · Phase 1（brainstorming 产出）
- **后续**：本文档经确认后，交由 `writing-plans` 生成实施计划

---

## 1. 目标与定位

为 ncgo（AI-friendly Go 微服务脚手架 CLI）构建项目官网，定位为 **落地页 + 文档站**：

- 让新访客在 30 秒内理解 ncgo 是什么、解决什么问题
- 提供一套**专门撰写**的、面向用户的使用文档（而非直接搬运仓库内部 spec）
- 中英双语，与项目既有文档的双语策略保持一致

**部署目标**：GitHub Pages（`https://byx-darwin.github.io/ncgo/`，预留自定义域名）。

**技术选型**：Docusaurus（React/Node）。理由：内置优秀的 i18n、全文搜索、MDX 与响应式主题，是开源 dev-tool 官网的事实标准，能最快得到专业且可长期维护的站点。

### 1.1 非目标（YAGNI，v1 明确不做）

- 博客内容（目录预留，不写文章）
- 文档版本化（versioning）
- Algolia 付费搜索（用 Docusaurus 自带本地搜索）
- 交互式在线 playground
- 独立的社区 / 论坛板块

---

## 2. 架构与仓库结构

### 2.1 代码位置：`website/` 子目录，与 Go 代码完全隔离

```
ncgo/
├── internal/          # Go 代码（不改动）
├── docs/              # 仓库既有文档（不改动）
├── specs/             # 内部设计文档（不改动，不上官网）
└── website/           # ✨ 新增：Docusaurus 项目
    ├── package.json
    ├── docusaurus.config.js
    ├── sidebars.js
    ├── docs/          # 英文文档（默认 locale: en）
    ├── blog/          # 预留，v1 不写内容
    ├── src/           # 落地页 React 组件 + 自定义样式
    ├── i18n/
    │   └── zh-CN/
    │       ├── docusaurus-plugin-content-docs/current/   # 中文文档
    │       └── docusaurus-theme-classic.json             # 中文 UI 文案
    └── static/        # favicon、图片
```

**隔离保证**：

- `website/` 是独立 npm 项目，`go build ./...` 与 `./scripts/smoke.sh` 完全不触及
- 现有 Go CI 工作流不改动；网站有独立的 GitHub Actions 工作流
- `.gitignore` 追加 `website/node_modules/`、`website/build/`
- `package.json` 通过 `engines` 字段固定 Node 20+

### 2.2 国际化（i18n）

- 默认 locale `en`，第二 locale `zh-CN`，导航栏带语言切换下拉
- 文档：EN 在 `website/docs/`，中文在 `website/i18n/zh-CN/docusaurus-plugin-content-docs/current/`
- UI 文案（navbar / footer / 主题文字）在 `website/i18n/{locale}/docusaurus-theme-classic.json`
- 两边信息架构严格对齐，先定稿 EN 再翻译 zh-CN

### 2.3 baseUrl 与域名

- 默认 `baseUrl: '/ncgo/'`（GitHub 项目页形态）
- 未来若绑定自定义域名，仅需将 `baseUrl` 改为 `/`，无需重构

---

## 3. 网站内容结构（信息架构）

### 3.1 顶部导航

`文档` · `命令参考` · `GitHub 仓库`（图标）· `中英文切换`（下拉）

### 3.2 文档区目录

```
docs/
├── 介绍 (intro)
│   └── ncgo 是什么 · 为什么需要它 · 适合谁 / 不适合谁
├── 快速开始 (getting-started)
│   ├── 安装 (install)
│   └── 30 秒上手 (30-second-tour)
├── 指南 (guides)
│   ├── 新建服务          —— ncgo new（Hertz / Kitex 单体）
│   ├── 微服务工作区      —— add rpc / add bff
│   ├── 领域与方法        —— add domain / add method
│   ├── 基础设施集成      —— add infra（Redis / Kafka / ES / 可观测…）
│   ├── AI 协作           —— ai init claude / ai sync / MCP server
│   └── 诊断与演进        —— doctor / upgrade / extract domain
├── 命令参考 (reference)
│   └── 全部命令 + flag 速查表
└── FAQ
```

### 3.3 内容取舍原则

- 写**用户视角**的使用文档；`specs/` 里的内部设计文档不搬上官网
- 每篇指南遵循固定骨架：**场景 → 命令 → 生成的目录树 → 下一步**
- 所有命令块**真实可复制**，目录树取自实际 `ncgo new` 等命令的输出，不凭空编造
- 命令参考做成**表格速查**，与 README 的 Common Commands 表对齐但各自独立维护

---

## 4. 落地页设计

### 4.1 核心思路：开场即产品

ncgo 最有辨识度的东西是**命令行**。首屏不放假大空的宣传语，而是放一个**会自己「跑起来」的终端**：用户一打开页面就看到 `ncgo new user-api` 被逐字敲入、输出滚动、旁边的项目目录树一个个文件「长」出来，`AGENTS.md` / `CLAUDE.md` 等 AI 上下文文件被高亮点亮。**一眼就懂这个工具在干什么。**

### 4.2 首屏布局（左右分栏，非居中堆叠）

```
┌───────────────────────────┬─────────────────────────────────────┐
│  ncgo                     │  ┌─ 终端（自动打字动画）────────────┐ │
│  AI-friendly Go 微服务脚手架│  │ $ ncgo new user-api \          │ │
│  —— 一条命令，生成可复现的  │  │     --module github.com/acme/…  │ │
│  Hertz / Kitex 服务骨架    │  │ ✔ manifest.yaml                │ │
│                           │  │ ✔ handler / service / repo     │ │
│  [复制安装命令]  [看文档→] │  │ ★ AGENTS.md  ★ CLAUDE.md       │ │
│                           │  └─────────────────────────────────┘ │
│                           │  ┌─ 生成的目录树（逐行生长）────────┐ │
│                           │  │ user-api/                        │ │
│                           │  │  ├─ .ncgo/manifest.yaml          │ │
│                           │  │  ├─ internal/…                   │ │
│                           │  │  └─ .claude/generated/…  ← 点亮 │ │
└───────────────────────────┴─────────────────────────────────────┘
```

- 左侧：一句点题的话 + 可一键复制的 `go install …` 命令 + 两个 CTA（安装 / 看文档）
- 右侧：**活终端 + 生长中的目录树**，打字机效果、闪烁光标

### 4.3 版式（字体搭配）

| 用途 | 字体 | 理由 |
|------|------|------|
| 大标题 / 展示 | **Chakra Petch** | 方正、有科技感，呼应「脚手架 / 工程」气质 |
| 正文 | **IBM Plex Sans** | 清晰易读，技术文档感 |
| 代码 / 终端 | **JetBrains Mono** | 开发者最熟悉的等宽字体 |

标题与正文在字号字重上拉开**强对比**。

### 4.4 配色（有层次，避免「纯黑 + 单色霓虹」）

- **底色**：深蓝墨（deep ink blue，非纯黑）+ 细网格纹理 + 柔和光晕，构成有纵深的环境背景
- **主色**：Go 青 `#00ADD8`（与项目既有 Go badge 一致，天然品牌锚点）
- **辅色**：琥珀金（标记「AI / agent」相关内容）、终端绿（成功输出）——多彩点缀，避免单一霓虹

### 4.5 页面版块（自上而下，避免「四张等大卡片」）

1. **首屏活终端**（见 4.2）
2. **工作流横轴**：`一条命令 → 生成骨架 → AI 上下文就绪 → 直接开发`，带连线的流程图，每步可展开
3. **特性介绍（不对称 Bento）**：大块讲「AI 友好」（展示 AGENTS.md / MCP），周围配不同尺寸的中小块（确定性脚手架 / 基础设施集成 / 生命周期工具），大小错落
4. **基础设施集成带**：Redis / Kafka / ES / ClickHouse / 可观测 等徽章横向滚动（marquee）
5. **命令速览（可交互 Tab）**：点 `doctor` / `add infra redis` / `mcp serve` 切换终端内容，模拟真实输出
6. **安装 CTA 大横幅**：超大等宽字体的安装命令 + 复制按钮
7. **页脚**：GitHub / 文档 / 语言切换 / License

### 4.6 动效（让页面「活」起来，降级安全）

- 终端打字机 + 光标闪烁 + 输出逐行出现
- 目录树文件逐个「生长」
- 滚动到对应版块时元素淡入上浮（scroll reveal）
- 命令芯片 / 卡片 hover 微交互（位移、描边点亮）
- 背景网格轻微视差
- 实现方式：CSS / framer-motion；**关闭 JS 时仍展示静态内容**（优雅降级）

---

## 5. 内容撰写计划（v1 交付清单）

### 5.1 文档清单（中英双语）

| 模块 | 文档 | 说明 |
|------|------|------|
| 介绍 | `intro` | ncgo 是什么 / 为什么 / 适合谁·不适合谁 |
| 快速开始 | `install` · `30-second-tour` | 安装 + 最短上手路径 |
| 指南 | 6 篇：新建服务 / 微服务 / 领域方法 / 基础设施 / AI 协作 / 诊断演进 | 用户视角操作指南 |
| 参考 | `commands` | 全命令 + flag 速查表 |
| FAQ | `faq` | 常见问题 |

### 5.2 撰写顺序与双语策略

- 先在默认 locale（`en`）定稿结构与内容，再翻译到 `zh-CN`，保证信息架构对齐
- 首页文案（hero、特性、CTA）同样双语

---

## 6. CI / 部署

新增独立工作流 `.github/workflows/website.yml`，**不改动现有 Go CI**：

```yaml
触发:
  - push → main 且 website/** 有变更  → 构建 + 部署到 GitHub Pages
  - pull_request 且 website/** 有变更 → 仅构建（预览校验，不部署）

部署链:
  npm ci → npm run build (website/)
  → actions/upload-pages-artifact
  → actions/deploy-pages

权限: pages: write, id-token: write
并发: 同环境只保留一个部署（concurrency group + cancel-in-progress）
```

- 发布地址：`https://byx-darwin.github.io/ncgo/`
- 构建失败（如断链）红灯拦截，坏站点不上线

---

## 7. 验证方式

| 检查 | 方式 |
|------|------|
| 构建通过 | `npm run build` 两个 locale 均成功；`onBrokenLinks: 'throw'` 断链直接报错 |
| 本地预览 | `npm run start`（en）/ `npm run start -- --locale zh-CN` 人工过一遍 |
| 双语完整性 | `npm run write-translations` + 检查 zh-CN 无遗漏未翻译文件 |
| 不影响 Go | `go build ./... && ./scripts/smoke.sh` 照常通过（website/ 完全隔离） |
| 部署冒烟 | 工作流跑通后访问线上 URL，确认首页 + 文档 + 语言切换正常 |
| 基础 SEO | favicon / title / description / og 标签齐全 |

---

## 8. 风险与开放问题

| # | 项 | 说明 / 缓解 |
|---|----|-------------|
| 1 | Node 工具链进入 Go 仓库 | 已通过 `website/` 目录 + 独立 CI 完全隔离 |
| 2 | 双语文档维护成本 | 先 EN 定稿再翻译；结构与命令块可复用，仅翻译散文部分 |
| 3 | GitHub Pages 需仓库开启 Pages | 部署前需在仓库 Settings 启用 Pages（Source: GitHub Actions） |
| 4 | 自定义域名 | v1 用 `byx-darwin.github.io/ncgo/`；域名事宜后续单独决策 |
| 5 | 字体加载 | 三个字体自托管于 `static/`，避免外部 CDN 依赖与隐私问题 |

---

## 9. 决策摘要

| # | 决策 | 结论 |
|---|------|------|
| 1 | 定位 | 落地页 + 文档站 |
| 2 | 托管 | GitHub Pages |
| 3 | 语言 | 中英双语（en 默认 + zh-CN） |
| 4 | 内容来源 | 专门撰写官网文档 |
| 5 | 技术栈 | Docusaurus |
| 6 | 代码位置 | 仓库 `website/` 子目录，与 Go 隔离 |
| 7 | 落地页风格 | 开场活终端、Chakra Petch / IBM Plex Sans / JetBrains Mono、深蓝墨 + Go 青 |

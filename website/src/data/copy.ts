import type { TermLine } from '../components/Terminal';
import type { TreeRow } from '../components/FileTree';
import type { CommandTab } from '../components/CommandTabs';

export const INSTALL_CMD = 'go install github.com/byx-darwin/ncgo@latest';

export const heroTerminal: TermLine[] = [
  { kind: 'command', text: 'ncgo new user-api --module github.com/acme/user-api' },
  { kind: 'output', text: '✔ .ncgo/manifest.yaml' },
  { kind: 'output', text: '✔ idl/app/user-api.proto' },
  { kind: 'output', text: '✔ template/' },
  { kind: 'success', text: '→ hz new: handler / service / repo generated' },
  { kind: 'highlight', text: '★ AGENTS.md    ★ CLAUDE.md' },
  { kind: 'highlight', text: '★ .claude/generated/project-context.md' },
];

export const heroTree: TreeRow[] = [
  { text: 'user-api/', depth: 0 },
  { text: '.ncgo/manifest.yaml', depth: 1 },
  { text: 'idl/app/user-api.proto', depth: 1 },
  { text: 'internal/handler/', depth: 1 },
  { text: 'internal/service/', depth: 1 },
  { text: 'internal/repo/', depth: 1 },
  { text: '.claude/generated/', depth: 1, ai: true },
  { text: 'AGENTS.md  CLAUDE.md', depth: 1, ai: true },
];

export const commandTabsData: CommandTab[] = [
  {
    id: 'doctor',
    label: 'doctor',
    lines: [
      { kind: 'command', text: 'ncgo doctor' },
      { kind: 'output', text: 'go      : ok (1.25)' },
      { kind: 'output', text: 'hz      : ok' },
      { kind: 'output', text: 'kitex   : ok' },
      { kind: 'success', text: 'project : manifest valid' },
    ],
  },
  {
    id: 'infra',
    label: 'add infra',
    lines: [
      { kind: 'command', text: 'ncgo add infra redis' },
      { kind: 'output', text: '✔ internal/infra/redis/' },
      { kind: 'success', text: 'redis helper wired into DI container' },
    ],
  },
  {
    id: 'mcp',
    label: 'mcp serve',
    lines: [
      { kind: 'command', text: 'ncgo mcp serve' },
      { kind: 'output', text: 'MCP stdio server listening' },
      { kind: 'highlight', text: 'tools: ncgo_version · ncgo_doctor · ncgo_ai_sync …' },
    ],
  },
];

export type WorkflowStep = { title: string; body: string };
export type Feature = { title: string; body: string; big?: boolean };

/** 基础设施徽章（产品名，中英一致，无需翻译） */
export const infraBadges = [
  'Redis', 'Kafka', 'Elasticsearch', 'ClickHouse', 'OpenTelemetry', 'Polaris', 'etcd',
];

export const copy = {
  en: {
    heroKicker: 'AI-friendly scaffold CLI',
    heroTitle: 'Scaffold Go microservices your agents can actually understand.',
    heroSub:
      'One command generates reproducible Hertz / Kitex service skeletons — with manifests, IDL placeholders, and AI context files under version control.',
    heroCtaDocs: 'Read the docs',
    heroCtaGithub: 'GitHub',
    workflowHeading: 'From one command to an agent-ready service',
    workflow: [
      { title: 'One command', body: 'ncgo new writes manifest, IDL placeholder and template inputs.' },
      { title: 'Generator runs', body: 'hz / kitex generate handler, service and repo layers.' },
      { title: 'AI context ready', body: 'AGENTS.md, CLAUDE.md and MCP tools are rendered alongside code.' },
      { title: 'Ship it', body: 'doctor, upgrade and extract keep the project healthy over time.' },
    ] as WorkflowStep[],
    featuresHeading: 'Built for reproducible, agent-friendly scaffolds',
    features: [
      { title: 'Agent-friendly by default', body: 'Renders AGENTS.md, CLAUDE.md, Cursor rules and exposes an MCP stdio server so AI tools understand every generated project.', big: true },
      { title: 'Deterministic scaffolding', body: 'Manifests and templates stay in version control — no one-off generator output.' },
      { title: 'Optional infra', body: 'Redis, Kafka, Elasticsearch, ClickHouse, observability and canary helpers.' },
      { title: 'Lifecycle tooling', body: 'doctor, upgrade, extract domain and protolint in one CLI.' },
    ] as Feature[],
    commandsHeading: 'See it in action',
    ctaHeading: 'Start in 30 seconds',
    footerNote: 'Built for the nc-skills-golang conventions.',
  },
  'zh-CN': {
    heroKicker: 'AI 友好的脚手架 CLI',
    heroTitle: '一条命令，生成 AI 能真正读懂的 Go 微服务骨架。',
    heroSub:
      '一次生成可复现的 Hertz / Kitex 服务骨架 —— manifest、IDL 占位符与 AI 上下文文件全部纳入版本控制。',
    heroCtaDocs: '阅读文档',
    heroCtaGithub: 'GitHub',
    workflowHeading: '从一条命令到 Agent 就绪的服务',
    workflow: [
      { title: '一条命令', body: 'ncgo new 写入 manifest、IDL 占位符与模板输入。' },
      { title: '生成器执行', body: 'hz / kitex 生成 handler、service、repo 分层。' },
      { title: 'AI 上下文就绪', body: 'AGENTS.md、CLAUDE.md 与 MCP 工具随代码一起渲染。' },
      { title: '持续演进', body: 'doctor、upgrade、extract 让项目长期保持健康。' },
    ] as WorkflowStep[],
    featuresHeading: '为可复现、对 Agent 友好的脚手架而生',
    features: [
      { title: '默认对 Agent 友好', body: '渲染 AGENTS.md、CLAUDE.md、Cursor 规则，并暴露 MCP stdio 服务器，让 AI 工具理解每个生成的项目。', big: true },
      { title: '确定性脚手架', body: 'manifest 与模板全部纳入版本控制，拒绝一次性生成器输出。' },
      { title: '可选基础设施', body: 'Redis、Kafka、Elasticsearch、ClickHouse、可观测与金丝雀发布助手。' },
      { title: '生命周期工具', body: 'doctor、upgrade、extract domain 与 protolint 集于一个 CLI。' },
    ] as Feature[],
    commandsHeading: '实际效果',
    ctaHeading: '30 秒上手',
    footerNote: '为 nc-skills-golang 约定而生。',
  },
} as const;

export type Locale = keyof typeof copy;

export function copyFor(locale: string) {
  return copy[(locale as Locale)] ?? copy.en;
}

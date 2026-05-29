# ncgo Release Labels 约定

本文件约定用于 GitHub Release 自动生成说明的标签使用方式。分类规则由
`.github/release.yml` 驱动；这里解释团队在提 PR / 合并变更时应该如何打标。

## 目标

- 让 GitHub 自动生成的 release notes 更稳定
- 让同一类变更进入正确的分类
- 降低每次发版时人工整理的成本

## 核心规则

1. 每个会进入发布说明的 PR，至少带一个 release label。
2. 如果同时满足多个类别，优先选择“对用户影响最大的那个”。
3. 纯内部噪音变更可使用 `skip-release-notes` 排除。
4. 若存在破坏性变更，必须额外带 `breaking-change` 或 `semver-major`。

## 标签与分类映射

| 标签 | Release 分类 | 何时使用 |
|---|---|---|
| `breaking-change` | Breaking Changes | 有兼容性破坏、命令行为变化、升级需要特别说明 |
| `semver-major` | Breaking Changes | 需要 major version 语义的重大变更 |
| `feature` | Features | 新增用户可感知功能 |
| `enhancement` | Features | 对现有能力的明显增强 |
| `fix` | Fixes | 修复错误行为、回归或错误输出 |
| `bug` | Fixes | 与 `fix` 类似，用于明确 bug 修复 |
| `docs` | Documentation | 文档、README、示例、发布说明改进 |
| `chore` | Internal | 维护性工作，但通常不值得放到面向用户的 highlights |
| `ci` | Internal | CI / workflow / automation 改动 |
| `refactor` | Internal | 重构、结构调整、无明显用户行为变化 |
| `test` | Internal | 测试补充、测试重构 |
| `skip-release-notes` | 排除 | 不希望出现在 release notes 中的变更 |

## 常见判断示例

### 适合 `feature`

- 新增 `ncgo add ...` 子命令
- 新增可选 infra 能力
- 新增 MCP 暴露能力

### 适合 `enhancement`

- 现有命令增加 `--plan` / `--dry-run` 这类增强能力
- 改进 README 首页、示例文档、可用性说明

### 适合 `fix` / `bug`

- 修复 `go install .` 不可用
- 修复生成器参数错误
- 修复输出路径或 manifest 写入问题

### 适合 `docs`

- README 重构
- 发布流程文档补充
- examples / FAQ / 安装说明增强

### 适合 `skip-release-notes`

- 纯拼写修正
- 仅注释调整
- 对外无意义的临时维护提交

## 推荐做法

- 尽量只给一个主 release label，避免分类重复
- 如果既有功能增强又有 breaking change，可同时打：
  - `breaking-change`
  - `feature` 或 `enhancement`
- 发布前可快速检查最近合并 PR 的标签，必要时补正

## 与发布流程的关系

- GitHub Release 通过 `gh release create --generate-notes` 自动生成说明
- 分类规则见 `.github/release.yml`
- 人工润色模板见 `docs/release-notes-template.zh-CN.md`
- PR 模板见 `.github/PULL_REQUEST_TEMPLATE.md`

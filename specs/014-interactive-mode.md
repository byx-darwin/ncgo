# ncgo new 交互模式设计

- 状态：Draft
- 关联：`specs/010-v1.0-plan.md` Phase 2 / P1-2

## 1. 问题

当前 `ncgo new` 需要记住 6+ 个 flag：

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz --db postgres --infra redis,kafka,es --preset default
```

忘写 `--module` 直接报错退出，新用户体验差。

## 2. 方案

缺必填参数时进入交互式提示流。增加 `--interactive`（默认 true，stdin 是终端时自动启用），`--no-interactive` 显式禁用。

### 2.1 交互流程

```
? Service name: user-api
? Go module path: github.com/acme/user-api           ← 根据 name + 当前目录推断默认值
? Service kind:  (•) hertz  ( ) kitex
? Database:      (•) none  ( ) postgres
? Add infra:     [ ] redis  [ ] kafka  [ ] es  [ ] clickhouse
                 [ ] logging  [ ] canary  [ ] otel
? Preset:        (•) default  ( ) rule-center
? Directory:     ./user-api                            ← 默认为当前目录/service-name

  This will create a new hertz service at ./user-api
  Module: github.com/acme/user-api
  Infra: redis, logging

? Proceed? (Y/n)
```

### 2.2 实现

用 [bubbletea](https://github.com/charmbracelet/bubbletea)：

```go
// internal/cli/interactive/new.go
func runNewInteractive() error {
    m := newModel()
    p := tea.NewProgram(m)
    if _, err := p.Run(); err != nil {
        return err
    }
    // m.opts 已由用户输入填充
    return runNewMono(context.Background(), m.opts)
}
```

依赖：`go get github.com/charmbracelet/bubbletea`

### 2.3 降级策略

- `--no-interactive`：回到当前纯 flag 模式
- stdin 不是终端（管道/CI）：自动降级
- `--module` 已传：跳过交互，直接执行（即使 stdin 是终端）

### 2.4 命令行接口

```bash
# 交互模式（默认）
ncgo new user-api

# 非交互模式（CI/脚本）
ncgo new user-api --module github.com/acme/user-api --no-interactive

# 交互但指定部分参数（跳过的选项不提示）
ncgo new user-api --module github.com/acme/user-api --kind kitex
```

## 3. 范围边界

v0.7 只做 `ncgo new` 的交互模式。`ncgo add` 系列命令保持纯 flag 模式（add 场景用户通常已经在终端里了，且参数少）。

## 4. 测试

- 单元测试：model Update 逻辑（输入 → 状态转换）
- 集成测试：`ncgo new --no-interactive` 回归（确保降级路径不变）
- bubbletea 有 `teatest` 可用于组件级测试

# wire.go AST 迁移方案

- 状态：Draft
- 关联：`specs/010-v1.0-plan.md` Phase 1 / P0-2

## 1. 问题

`internal/scaffold/infra/wire.go`（510 行）使用 `strings.Contains` + `strings.Replace` 修改生成的 Go 源码：

```go
// 当前做法：字符串替换（脆弱）
idx := strings.Index(src, "import (\n")
insertAt := idx + len("import (\n")
return src[:insertAt] + "\t" + quoted + "\n" + src[insertAt:], true, nil
```

断裂场景：
- `import (` 后面没有 `\n`（`import ("fmt")` 单行写法）
- import 分组中间有空行
- cgo（`import "C"`）
- 上游模板改变缩进（tab → spaces）

## 2. 方案

用 `go/ast` + `go/format` 重写插入和替换逻辑。保留 `// ncgo:wire:` 标记注释作为 AST 遍历锚点。

### 2.1 核心流程

```
1. go/parser.ParseFile(src) → *ast.File
2. 遍历 ast.File.Decls，找标记注释
3. 根据标记类型执行 AST 操作：
   - "import"    → 在 import decl 中插入新 import
   - "init"      → 在 func main()/Init() 中插入语句
   - "middleware" → 在 middleware 链中插入
4. go/format.Node(buf, fset, node) → 格式化输出
5. 写回文件
```

### 2.2 标记注释规范（不变）

```go
// ncgo:wire:logging:import    ← 锚点，此行下方插入 import
// ncgo:wire:logging:init      ← 锚点，此行下方插入初始化语句
// ncgo:wire:canary:middleware ← 锚点，此行下方插入中间件
```

### 2.3 AST 操作示例

**插入 import**：
```go
func insertImport(f *ast.File, importPath string) {
    for _, decl := range f.Decls {
        genDecl, ok := decl.(*ast.GenDecl)
        if !ok || genDecl.Tok != token.IMPORT { continue }
        // 在已有 import 列表末尾追加
        genDecl.Specs = append(genDecl.Specs, &ast.ImportSpec{
            Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(importPath)},
        })
    }
}
```

**在标记注释后插入语句**：
```go
func insertAfterMarker(body *ast.BlockStmt, markerPrefix string, stmt ast.Stmt) bool {
    for i, s := range body.List {
        if hasCommentPrefix(s, markerPrefix) {
            // 在标记行之后插入
            body.List = append(body.List[:i+1], append([]ast.Stmt{stmt}, body.List[i+1:]...)...)
            return true
        }
    }
    return false
}
```

**插入 middleware**：
```go
// 找到 h.Use(middleware1, middleware2, ...) 调用
// 在参数列表末尾追加新 middleware
```

### 2.4 模板代码块外部化

wire.go 中嵌入的大量 Go 代码字符串（`hertzLoggingInit()` 38 行、`kitexLoggingInit()` 等）移到嵌入模板文件：

```
internal/assets/_data/
  wire/
    hertz/
      logging_init.go.tmpl
      canary_init.go.tmpl
    kitex/
      logging_init.go.tmpl
      canary_init.go.tmpl
```

wire.go 改为 `assets.FS().ReadFile()` 加载模板，`text/template` 渲染。

### 2.5 向后兼容

- 命令行接口不变：`ncgo add infra logging --wire` 行为完全一致
- 标记注释格式不变
- 生成的 Go 代码格式化输出，gofmt 保证一致性

## 3. 迁移步骤

1. 新建 `internal/scaffold/infra/astwire/` 包，实现 AST 操作函数
2. 为每种 wiring 类型写 golden test（输入源文件 → AST 处理后输出 → 与 golden 对比）
3. wire.go 中逐个函数迁移（每个函数独立，可以逐步替换）
4. 确认所有测试通过后，删除旧字符串操作代码
5. 外部化 Go 代码模板到 assets/_data/wire/

## 4. 测试

每个 wiring 函数配一个 golden test：

```
internal/scaffold/infra/astwire/
  wire_test.go
  testdata/
    hertz_logging_input.go     ← 处理前的源文件
    hertz_logging_golden.go    ← 期望输出
    hertz_canary_input.go
    hertz_canary_golden.go
    kitex_logging_input.go
    kitex_logging_golden.go
    ...
```

## 5. 风险

- AST 操作比字符串替换复杂，需要理解 Go AST 结构
- go/parser 对不合法的 Go 代码会报错（但生成的代码总是合法的）
- 格式化输出可能与原始缩进风格略有不同（但 gofmt 保证一致性，这是好事）

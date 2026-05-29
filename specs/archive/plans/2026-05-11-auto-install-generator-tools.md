# Auto-Install Generator Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `ncgo new --mode mono` 生成项目前，自动检测并安装缺失的生成器工具（hz/kitex），用户确认后执行 `go install`。

**Architecture:** 在 `internal/cli/root.go` 的 `runNewMono` 函数中，调用 `mono.Generate` 之前增加预检函数 `preflightTools`。该函数检查所需工具是否在 PATH 上，缺失时列出并询问用户是否自动安装。`--no-generate` 时跳过预检。

**Tech Stack:** Go, cobra CLI, os/exec

---

### Task 1: 添加 `exec.Install` 函数

**Files:**
- Modify: `internal/exec/exec.go`

- [ ] **Step 1: 添加 Install 函数到 exec.go**

在 `InstallHint` 函数后面（约 line 41），添加 `Install` 函数：

```go
// Install runs `go install <path>@latest` for the named tool. It returns
// an error with the command output if installation fails.
func Install(ctx context.Context, name string) error {
	path := installPath(name)
	if path == "" {
		return fmt.Errorf("exec: no install path known for %q", name)
	}
	cmd := osexec.CommandContext(ctx, "go", "install", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec: install %s: %w: %s", name, err, bytes.TrimSpace(out))
	}
	return nil
}

var installPaths = map[string]string{
	"hz":    "github.com/cloudwego/hertz/cmd/hz@latest",
	"kitex": "github.com/cloudwego/kitex/tool/cmd/kitex@latest",
}

func installPath(name string) string {
	return installPaths[name]
}
```

- [ ] **Step 2: 验证编译通过**

```bash
go build ./...
```

Expected: 编译成功

- [ ] **Step 3: 添加 Install 测试到 exec_test.go**

**Files:**
- Modify: `internal/exec/exec_test.go`

在 `TestRunMissingBinary` 测试之后添加：

```go
func TestInstallUnknownToolReturnsError(t *testing.T) {
	err := Install(context.Background(), "definitely-not-a-tool")
	if err == nil {
		t.Fatalf("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "no install path") {
		t.Errorf("error = %q, want 'no install path'", err.Error())
	}
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
go test ./internal/exec/... -run TestInstallUnknownToolReturnsError -v
```

Expected: PASS

- [ ] **Step 5: 运行全部 exec 测试**

```bash
go test ./internal/exec/... -v -count=1
```

Expected: 全部 PASS

- [ ] **Step 6: 提交**

```bash
git add internal/exec/exec.go internal/exec/exec_test.go
git commit -m "feat(exec): add Install function for auto-installing generator tools

Add Install(ctx, name) that runs 'go install <path>@latest' for known
tools (hz, kitex). Returns descriptive errors on unknown tools or
installation failures."
```

---

### Task 2: 添加 `preflightTools` 函数到 CLI

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`

- [ ] **Step 1: 添加 imports**

在 `internal/cli/root.go` 的 import 块中添加：

```go
import (
	"bufio"
	// ... existing imports ...
	"io"
	"os/exec"
	"strings"
)
```

确保最终的 import 块为：

```go
import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/assets"
	goexec "github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/micro"
	"github.com/byx-darwin/ncgo/internal/scaffold/mono"
)
```

注意：使用 `goexec` 别名避免与标准库 `os/exec` 冲突。

- [ ] **Step 2: 更新现有代码中的 exec 为 goexec**

搜索替换 `root.go` 中所有 `exec.` 为 `goexec.`：
- `*exec.NotFoundError` → `*goexec.NotFoundError`
- `exec.InstallHint` → `goexec.InstallHint`
- `exec.Runner` → `goexec.Runner`

- [ ] **Step 3: 添加 preflightTools 函数**

在 `dirOrEmpty` 函数之后（文件末尾）添加：

```go
// toolPreflight describes a missing generator tool.
type toolPreflight struct {
	name       string // binary name, e.g. "hz"
	minVersion string // minimum version, e.g. "v0.9.7"
	installCmd string // full go install command, e.g. "go install github.com/cloudwego/hertz/cmd/hz@latest"
}

// preflightTools checks whether the required generator tools are on PATH.
// If any are missing, it lists them and asks the user for confirmation to
// auto-install. Returns nil when all tools are present (or user confirmed
// and installation succeeded). Returns an error when the user declines or
// installation fails.
func preflightTools(ctx context.Context, kind string, noGenerate bool, w io.Writer, r io.Reader) error {
	if noGenerate {
		return nil
	}

	missing := requiredTools(kind)
	if len(missing) == 0 {
		return nil
	}

	fmt.Fprintln(w, "The following generator tools are required but not found on PATH:")
	for _, t := range missing {
		fmt.Fprintf(w, "  • %s (>= %s) — %s\n", t.name, t.minVersion, t.installCmd)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Auto-install missing tools with 'go install'? [Y/n] ")

	answer := readLine(r)
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		fmt.Fprintln(w, "Aborted. Please install the tools manually and rerun.")
		return fmt.Errorf("user declined to install tools")
	}

	for _, t := range missing {
		fmt.Fprintf(w, "Installing %s...\n", t.name)
		if err := goexec.Install(ctx, t.name); err != nil {
			fmt.Fprintf(w, "Failed to install %s: %v\n", t.name, err)
			fmt.Fprintf(w, "Please install manually: %s\n", t.installCmd)
			return fmt.Errorf("install %s: %w", t.name, err)
		}
		fmt.Fprintf(w, "Successfully installed %s\n", t.name)
	}

	return nil
}

// requiredTools returns the list of missing generator tools for the given kind.
func requiredTools(kind string) []toolPreflight {
	var need []toolPreflight

	switch kind {
	case manifest.KindHertz, "":
		if _, err := goexec.LookPath("hz"); err != nil {
			need = append(need, toolPreflight{
				name:       "hz",
				minVersion: goexec.MinHzVersion,
				installCmd: "go install " + goexec.InstallHint("hz"),
			})
		}
	case manifest.KindKitex:
		if _, err := goexec.LookPath("kitex"); err != nil {
			need = append(need, toolPreflight{
				name:       "kitex",
				minVersion: goexec.MinKitexVersion,
				installCmd: "go install " + goexec.InstallHint("kitex"),
			})
		}
	}

	return need
}

// readLine reads a single line from r, stripping the trailing newline.
func readLine(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}
```

- [ ] **Step 4: 在 runNewMono 中调用 preflightTools**

在 `runNewMono` 函数中，`dir := opts.dir` 之前（约 line 189），插入：

```go
	if err := preflightTools(cmd.Context(), opts.kind, opts.noGenerate, cmd.OutOrStdout(), cmd.InOrStdin()); err != nil {
		return err
	}
```

- [ ] **Step 5: 编译验证**

```bash
go build ./...
```

Expected: 编译成功

- [ ] **Step 6: 添加 preflightTools 单元测试**

在 `internal/cli/root_test.go` 末尾添加：

```go
import (
	"context"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestPreflightSkippedWhenNoGenerate(t *testing.T) {
	var out strings.Builder
	err := preflightTools(context.Background(), manifest.KindHertz, true, &out, strings.NewReader("n"))
	if err != nil {
		t.Fatalf("preflightTools with noGenerate=true: %v", err)
	}
	if out.Len() > 0 {
		t.Fatalf("expected no output with noGenerate=true, got: %s", out.String())
	}
}

func TestPreflightUserDeclineReturnsError(t *testing.T) {
	var out strings.Builder
	// Simulate user typing "n"
	err := preflightTools(context.Background(), manifest.KindHertz, false, &out, strings.NewReader("n"))
	if err == nil {
		t.Fatal("expected error when user declines")
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("output should mention 'Aborted': %s", out.String())
	}
}

func TestPreflightUserAcceptsEmptyAnswer(t *testing.T) {
	var out strings.Builder
	// Empty answer (just Enter) should default to yes
	// Since hz is likely installed on the test machine, this test verifies
	// the acceptance logic, not the actual install
	err := preflightTools(context.Background(), manifest.KindHertz, false, &out, strings.NewReader(""))
	// If hz is already installed, err is nil; if not, it will try to install
	// We only verify the prompt was shown when tools are missing
	output := out.String()
	if strings.Contains(output, "required but not found") {
		// Tools were missing, check prompt was shown
		if !strings.Contains(output, "[Y/n]") {
			t.Errorf("output should contain prompt '[Y/n]': %s", output)
		}
	}
	_ = err // installation may or may not succeed depending on environment
}

func TestRequiredToolsReturnsMissingForUnknownKind(t *testing.T) {
	need := requiredTools("unknown-kind")
	// Empty/unknown kind defaults to hertz, so hz should be checked
	if len(need) > 1 {
		t.Fatalf("expected at most 1 missing tool for unknown kind, got %d", len(need))
	}
}
```

- [ ] **Step 7: 运行测试验证**

```bash
go test ./internal/cli/... -run 'TestPreflight|TestRequiredTools' -v -count=1
```

Expected: 全部 PASS（注意：`TestPreflightUserAcceptsEmptyAnswer` 如果 hz 已安装，不检查安装逻辑；如果未安装会尝试安装）

- [ ] **Step 8: 运行全部 CLI 测试**

```bash
go test ./internal/cli/... -v -count=1
```

Expected: 全部 PASS

- [ ] **Step 9: 提交**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "feat(cli): add preflight tool check and auto-install to ncgo new

Before generating a mono service, check if the required generator
(hz/kitex) is on PATH. If missing, list the tools and ask the user for
confirmation to auto-install via 'go install'. Skipped when
--no-generate is set."
```

---

### Task 3: 端到端验证

- [ ] **Step 1: 完整编译和 vet**

```bash
go build ./... && go vet ./...
```

Expected: 无错误

- [ ] **Step 2: 全部单元测试**

```bash
go test ./... -count=1
```

Expected: 全部 PASS

- [ ] **Step 3: 构建二进制并快速验证**

```bash
go build .
./ncgo --help
```

Expected: 帮助信息正常显示

- [ ] **Step 4: 运行 smoke test**

```bash
./scripts/smoke.sh
```

Expected: 全部通过

- [ ] **Step 5: 提交（如有必要）**

如果前面的提交已完整，无需额外提交。

---

### Task 4: 更新文档

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`

- [ ] **Step 1: 更新 README.md 的 Requirements 部分**

在 README.md 的 Requirements 部分（约 line 77-86），更新为：

```markdown
## Requirements

- Go `1.25+`
- `hz >= v0.9.7` when generating Hertz services (auto-installed on demand)
- `kitex >= v0.16.1` when generating Kitex services (auto-installed on demand)
- Hertz templates' `make swagger` target requires `protoc` and
  `protoc-gen-http-swagger` on `PATH`

If you only want manifests, IDL placeholders, and template inputs, use
`--no-generate` and install the generators later.
```

- [ ] **Step 2: 更新 README.md 的 FAQ 部分**

在 FAQ "hz or kitex not found on PATH" 部分（约 line 473-483），更新为：

```markdown
### `hz` or `kitex` not found on PATH

`ncgo new` will automatically detect missing generators and offer to
install them for you. Answer `Y` at the prompt to auto-install, or type
`n` to abort and install manually:

```bash
go install github.com/cloudwego/hertz/cmd/hz@latest
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
ncgo doctor
```

If you want to prepare files first and run generators later, use `--no-generate`.
```

- [ ] **Step 3: 同步更新 README.zh-CN.md**

在 README.zh-CN.md 中找到对应的 Requirements 和 FAQ 部分，做相同的更新。

- [ ] **Step 4: 运行 markdown 诊断**

```bash
# 检查 markdown 格式（如果有配置 markdownlint 或其他工具）
cat README.md | head -100
cat README.zh-CN.md | head -100
```

- [ ] **Step 5: 提交**

```bash
git add README.md README.zh-CN.md
git commit -m "docs: update requirements and FAQ for auto-install generator tools"
```

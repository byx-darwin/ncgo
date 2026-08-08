# Issue #52 — ncgo check context.stale 修复（内容比对） 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** 修复 `ncgo check` 的 `check.context.stale` 永不触发的问题。改为内容比对：解析上下文文件里的 `domains:` 事实行，与当前 `manifest.Domains` 比对，不一致即 stale。

**Architecture:** 改 `internal/cli/check.go` 的 `contextStaleChecks`：不再依赖 `manifest.GeneratedAt` 时间戳（该值在 `add domain` 时不更新，导致 stale 永不触发），改为从已渲染的上下文文件（CLAUDE.md / AGENTS.md，存在者）解析 `- domains: `[a, b]`` 行并与 manifest 比对。`ai.ReadGeneratedAt` 不再用于 stale 判定（保留函数，可能被其他用途引用）。

**Tech Stack:** Go, `internal/manifest`, `internal/ai`, `internal/cli/check.go`, regexp/strings 解析。

## Global Constraints

- `ncgo check` 退出码 0/1/2 不变；`check.context.stale` 仍为 error 严重度。
- 上下文文件缺失时跳过（不强制 ai sync 已运行）——与现状一致。
- CLI/doctor 输出结构 contract-sensitive，`check.context.stale` 的 Message/Hint 语义保持。
- 不改变 `ncgo check` 的 CLI 契约（无新 flag）。
- 最终验证：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。

---

### Task 1: 内容比对实现

**Files:**
- Modify: `internal/cli/check.go`（`contextStaleChecks` 改为内容比对）
- Create: `internal/cli/check_test.go` 新增测试（或复用现有文件）

**Interfaces:**
- Consumes: `manifest.Manifest.Domains []string`，上下文文件内容（CLAUDE.md/AGENTS.md）。
- Produces: 新辅助函数 `parseContextDomains(content string) []string`（从 `- domains: `[a, b]`` 行提取列表）。

- [ ] **Step 1: 写失败测试** — 在 `internal/cli/check_test.go` 追加：

```go
func TestRunCheckExitOneOnStaleDomains(t *testing.T) {
	root := seedCheckProject(t)
	// manifest 已含 device；上下文文件宣称只有 []，或含 device + 已删的 ghost
	claude := filepath.Join(root, "CLAUDE.md")
	content := "<!-- ncgo:managed -->\n# Project Context for Claude Code\n\n## Project Facts\n\n- domains: `[device, ghost]`\n"
	if err := os.WriteFile(claude, []byte(content), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runCheck(cmd, &checkOptions{root: root})
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.code != 1 {
		t.Fatalf("err = %v, want exitCodeError code 1 (context claims ghost domain)", err)
	}
}

func TestParseContextDomains(t *testing.T) {
	tests := []struct{ in, want string }{
		{"- domains: `[device, order]`", "device,order"},
		{"- domains: `[device]`", "device"},
		{"- domains: `[]`", ""},
		{"no domains line here", ""},
	}
	for _, tt := range tests {
		got := strings.Join(parseContextDomains(tt.in), ",")
		if got != tt.want {
			t.Errorf("parseContextDomains(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
```

> 注：`seedCheckProject` 已有（Task 4 of Issue #50 时创建），manifest 含 domain `device`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/cli/ -run 'TestRunCheckExitOneOnStaleDomains|TestParseContextDomains' -count=1`
Expected: FAIL（`parseContextDomains` 未定义；stale 检查仍是时间戳逻辑，domains 不一致不报 stale）。

- [ ] **Step 3: 实现 `parseContextDomains` + 改造 `contextStaleChecks`**

在 `internal/cli/check.go` 加：

```go
// parseContextDomains extracts the domain list from a rendered context file's
// `- domains: \`[a, b]\`` fact line. Returns nil when the line is absent.
func parseContextDomains(content string) []string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- domains: `[") {
			continue
		}
		rest := strings.TrimPrefix(line, "- domains: `[")
		rest = strings.TrimSuffix(rest, "]`")
		if rest == "" {
			return []string{}
		}
		parts := strings.Split(rest, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
```

改造 `contextStaleChecks`：

```go
// contextStaleChecks compares the domains declared in a rendered context file
// (CLAUDE.md or AGENTS.md, whichever exists) against the current manifest. A
// mismatch means the AI context is stale (a domain was added/removed without
// re-running `ai sync`). Missing context files are skipped (not a failure).
func contextStaleChecks(root string, m *manifest.Manifest) []doctor.Check {
	var out []doctor.Check
	path := ""
	for _, rel := range contextFileTargets() {
		if rel == ".claude/skills/ncgo-dev/SKILL.md" || rel == ".cursor/rules/ncgo.mdc" {
			continue // SKILL.md / .mdc do not carry the domains fact line
		}
		candidate := filepath.Join(root, rel)
		if pathExists(candidate) {
			path = candidate
			break
		}
	}
	if path == "" {
		return []doctor.Check{okContextCheck("no rendered context file present; nothing to compare")}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return []doctor.Check{{
			ID: "check.context.stale", OK: false, Severity: doctor.SeverityError,
			Message: fmt.Sprintf("read %s: %v", path, err), File: path,
		}}
	}
	rendered := parseContextDomains(string(body))
	if rendered == nil {
		return []doctor.Check{okContextCheck("context file has no domains fact line")}
	}
	if !sameStringSet(rendered, m.Domains) {
		return []doctor.Check{{
			ID: "check.context.stale", OK: false, Severity: doctor.SeverityError,
			Message: fmt.Sprintf("%s is stale: context declares domains %v, manifest has %v", filepath.Base(path), rendered, m.Domains),
			File:    path,
			Hint:    "run `ncgo ai sync --root .`",
		}}
	}
	return []doctor.Check{okContextCheck("AI context domains match manifest")}
}

func okContextCheck(msg string) doctor.Check {
	return doctor.Check{ID: "check.context.stale", OK: true, Severity: doctor.SeverityError, Message: msg}
}

// sameStringSet reports whether two slices contain the same strings
// (order-insensitive).
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}
```

需要 `strings` 导入（check.go 目前没有 `strings`，需加；`os` 已有，`fmt` 已有，`filepath` 已有）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/cli/ -count=1`
Expected: PASS（含新测试；`TestRunCheckExitOneOnStaleContext` 旧测试——若它依赖时间戳语义，需检查是否仍成立；若无 CLAUDE.md 则跳过不失败）。

> **注意**：现有 `TestRunCheckExitOneOnStaleContext`（Issue #50 的测试）写了 `CLAUDE.md` 带旧时间戳 `2020-01-01` 但**没有 domains 行** → 新逻辑返回 `okContextCheck("context file has no domains fact line")` → 该测试会 FAIL（期望 exit 1）。需更新该测试：改为写 `- domains: `[device, ghost]`` 行（或删除，由新测试覆盖）。

- [ ] **Step 5: 更新旧 stale 测试** — 将 `TestRunCheckExitOneOnStaleContext` 改为内容比对语义（写含错误 domains 的 CLAUDE.md），或删除由 `TestRunCheckExitOneOnStaleDomains` 覆盖。
- [ ] **Step 6: 跑完整 cli 测试 + smoke 验证**

Run: `go test ./internal/cli/ -count=1 && ./scripts/smoke.sh`
Expected: PASS。

- [ ] **Step 7: Commit**

```bash
git add internal/cli/check.go internal/cli/check_test.go
git commit -m "fix(cli): ncgo check stale detection via content comparison (domains fact vs manifest)"
```

---

### Task 2: 全量验证

- [ ] **Step 1**: `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`
- [ ] **Step 2**: 修复发现的问题，重跑直到全绿。
- [ ] **Step 3**: Commit（若有残留改动）。

---

## Self-Review

**Spec coverage:** AC1（内容比对）→ Task 1 ✓; AC2（add domain 后 check 报 stale）→ 由新测试验证 ✓; AC3（ai sync 后通过）→ smoke/测试覆盖 ✓; AC4（golden/test 不受影响）→ domain 测试不锁 manifest 时间戳，已确认 ✓; AC5（demo 复现全绿）→ Task 2 验证 ✓。

**Placeholder scan:** 无 TBD；`TestRunCheckExitOneOnStaleContext` 更新已在 Step 5 明确。

**Type consistency:** `parseContextDomains(content string) []string`、`sameStringSet(a, b []string) bool` 定义一次，测试与实现一致。`contextFileTargets` 保留（SKILL/.mdc 跳过不比对）。

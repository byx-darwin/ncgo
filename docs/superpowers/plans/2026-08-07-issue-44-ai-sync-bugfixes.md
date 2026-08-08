<!-- Issue: 44 -->

# ai sync 基础缺陷修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `ncgo ai sync` 的 4 项基础缺陷（symlink 写入逃逸、docs/ncgo 标记死锁、链接改写 bug、dry-run 少报），恢复 docs/ncgo 的可刷新幂等契约并封掉越界写入面。

**Architecture:** 所有修复集中在 `internal/ai/sync.go`（writeTarget / writeStandaloneDocs / rewriteDocLinks）与 `internal/ai/render.go`（ManagedMarker 相关）。新增一个路径安全校验 helper `safeWritePath`，在 writeTarget/writeStandaloneDocs 写盘前统一执行；链接改写改为只映射 `docs/<profile>/` → `../<profile>/`、保留 `../<profile>/` 原样。

**Tech Stack:** Go 1.25+，标准库（os / filepath / errors / io/fs / strings / bytes）。

## 已有关联 Issue

本计划引用已创建的 **Issue #44**（https://github.com/byx-darwin/ncgo/issues/44）。该 Issue 已存在，因此跳过"创建 Issue"任务；Task 1 直接将其标记为 in-progress。

**Issue 标题:** fix(ai): ai sync 基础缺陷——symlink 写入逃逸、docs 标记死锁、链接改写、dry-run 少报

**Issue 标签:** bug

**验收标准（来自 Issue #44）：**
- [ ] writeTarget/writeStandaloneDocs 写入前解析并校验最终路径在 --root 内（no-follow），悬空 symlink 进入 skip 而非创建
- [ ] docs/ncgo/** 写入内容前置 <!-- ncgo:managed -->；新增"连续两次 sync 第二次正常刷新 docs"回归测试
- [ ] rewriteDocLinks 保留 ../<profile>/ 兄弟链接语义；新增断言改写后 href 目标可解析
- [ ] writeStandaloneDocs 在 dry-run 下照常记录 Skip 条目（与主目标一致），更新相关测试

## Global Constraints

- 保持对外契约稳定：CLI flags、MCP 字段、4 个受管目标路径、managed marker 语义不变。
- 改动集中在 `internal/ai/`，不跨包重构。
- 每个缺陷单独一个 commit，commit message 引用 `(#44)`。
- 遵循 TDD：先写/改测试（含失败用例），再改实现。
- 修改测试预期属于行为变更的一部分，必须与实现同一 commit。

## File Structure

```
internal/ai/sync.go        # 修改：safeWritePath + writeTarget/writeStandaloneDocs 接入 + rewriteDocLinks
internal/ai/sync_test.go   # 修改+新增：symlink 测试、两遍 sync 测试、链接可解析测试、dry-run 测试更新
internal/ai/render.go      # 无改动（仅引用 ManagedMarker）
docs/                      # 如行为变更影响 README/examples，随 commit 更新（预计无需大改）
```

## Tasks

### Task 1: 同步 Issue #44 状态为 in-progress

**Description:** 将 Issue #44 标记为 `status: in-progress`，表示开发已开始。

- [ ] **Step 1: 运行 sync-status.sh**

```bash
bash /Users/baoyx/.claude/skills/writing-plans-with-issue/scripts/sync-status.sh in-progress
```

- [ ] **Step 2: 确认**

```bash
echo "✅ Issue #$(cat .claude/gh-issue/current-issue.txt) 已标记为 in-progress"
```

### Task 2: 修复 symlink 写入逃逸（Fix 1）

**Description:** 新增 `safeWritePath(root, relPath)` helper，在 writeTarget 与 writeStandaloneDocs 写盘前校验最终路径不通过 symlink 逃出 root；悬空 symlink 进入 skip 而非创建。`--force` 不绕过此安全边界。

**背景：** 当前 `writeTarget`（sync.go:369-395）与 `writeStandaloneDocs`（sync.go:405-436）用 `os.WriteFile` 跟随最终组件 symlink，全仓无 Lstat/EvalSymlinks 校验。悬空 symlink 的 `os.ReadFile` 返回 ErrNotExist → 落入"文件不存在"分支 → 直接沿 symlink 在 root 外创建文件。

- [ ] **Step 1: 新增失败测试（sync_test.go）**

```go
// TestSyncRefusesSymlinkEscape：root 下 AGENTS.md 是 symlink → root 外文件。
// 运行 Sync（无 --force），断言：AGENTS.md 被 skip（reason 含 "symlink"）；root 外目标文件内容未被修改。
// TestSyncForceStillRefusesSymlinkEscape：同上但 Force:true，仍 skip（安全边界不因 --force 绕过）。
// TestSyncRefusesDanglingSymlink：AGENTS.md 是悬空 symlink → root 外不存在路径。
// 断言：skip；root 外目标路径未被创建。
```

- [ ] **Step 2: 实现 `safeWritePath`（sync.go）**

```go
// safeWritePath 校验将 relPath 写入 root 是否安全（不通过 symlink 逃出 root），
// 返回规范化写入路径 full 与可选 skip 原因。--force 不绕过安全边界。
func safeWritePath(root, relPath string) (full string, skip string, err error) {
	absRoot, _ := filepath.Abs(root)
	resolvedRoot, rerr := filepath.EvalSymlinks(absRoot)
	if rerr != nil {
		resolvedRoot = filepath.Clean(absRoot)
	}
	full = filepath.Join(resolvedRoot, filepath.Clean(relPath))
	// 从 resolvedRoot 逐组件向下走：已有组件若为 symlink，解析后必须仍落在 resolvedRoot 内；
	// 悬空 symlink（任意层级）→ 返回 skip；叶子缺失且父链安全 → 允许创建。
	rel, err := filepath.Rel(resolvedRoot, full)
	if err != nil {
		return "", "", err
	}
	if rel == "." {
		return full, "", nil
	}
	parts := strings.Split(rel, string(filepath.Separator))
	cur := resolvedRoot
	for i, part := range parts {
		isLeaf := i == len(parts)-1
		cur = filepath.Join(cur, part)
		li, lerr := os.Lstat(cur)
		if errors.Is(lerr, fs.ErrNotExist) {
			continue // 缺失组件：父链已验证安全，MkdirAll 可创建
		}
		if lerr != nil {
			return "", "", fmt.Errorf("ai sync: lstat %s: %w", cur, lerr)
		}
		if li.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, terr := filepath.EvalSymlinks(cur)
		if terr != nil {
			return "", fmt.Sprintf("refusing to write through dangling symlink %s", part), nil
		}
		if !pathWithin(resolvedRoot, target) {
			return "", fmt.Sprintf("refusing to write through symlink outside project root: %s", part), nil
		}
		cur = target // 后续组件相对 symlink 目标继续
	}
	return full, "", nil
}

func pathWithin(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
```

- [ ] **Step 3: writeTarget 接入**（在 `full := filepath.Join(...)` 处替换为 safeWritePath；skip 时先于 marker 判断追加 `res.Skipped` 并 return nil；`--force` 不改变 skip 分支）

- [ ] **Step 4: writeStandaloneDocs 接入**（同上；安全 skip 追加到 `res.Skipped` 并 continue）

- [ ] **Step 5: 运行验证**

```bash
go test ./internal/ai/... -run 'TestSyncRefusesSymlink|TestSyncForceStillRefusesSymlink|TestSyncRefusesDanglingSymlink' -count=1
go test ./internal/ai/... -count=1
```

- [ ] **Step 6: 提交**

```bash
git add internal/ai/sync.go internal/ai/sync_test.go
git commit -m "fix(ai): refuse writing through symlinks escaping project root (#44)"
```

### Task 3: 修复 docs/ncgo 标记死锁（Fix 2）

**Description:** `writeStandaloneDocs` 写盘前在内容头部注入 `ManagedMarker + "\n"`，使二次 sync 命中 `isManaged` 走覆盖分支；新增两遍 sync 回归测试。

**背景：** 嵌入设计文档资产本身不含 marker，`writeStandaloneDocs` 原样写出 → docs/ncgo/** 无标记 → 第二次 sync 判"无标记→跳过"，docs 永不刷新。测试 `TestSyncWritesStandaloneDocs` 只跑一遍，漏检。

- [ ] **Step 1: 新增失败测试（sync_test.go）**

```go
// TestSyncRefreshesStandaloneDocsOnSecondPass：连续两次 Sync（Hertz manifest）。
// 第二次 res.Written 必须包含 docs/ncgo/hertz/design-doc.en.md（而非 skip），
// 且 docs/ncgo 文件内容包含 ManagedMarker。
```

- [ ] **Step 2: 修改 `writeStandaloneDocs`**

```go
// 写盘前：
content := ManagedMarker + "\n" + rewriteDocLinks(string(b))
```

- [ ] **Step 3: 运行验证**

```bash
go test ./internal/ai/... -run 'TestSyncRefreshesStandaloneDocsOnSecondPass|TestSyncWritesStandaloneDocs' -count=1
go test ./internal/ai/... -count=1
```

- [ ] **Step 4: 提交**

```bash
git add internal/ai/sync.go internal/ai/sync_test.go
git commit -m "fix(ai): mark standalone docs managed so second sync refreshes them (#44)"
```

### Task 4: 修复链接改写语义（Fix 3）

**Description:** `rewriteDocLinks` 只把 `docs/<profile>/` 映射为 `../<profile>/`，保留 `../<profile>/` 原样（物化到 docs/ncgo/<profile>/ 后它本来就是正确的兄弟链接）；新增"改写后 href 目标可解析"断言。

**背景：** 当前 `rewriteDocLinks`（sync.go:479-490）把 `../kitex/` 也改写为 `./kitex/`，落到 docs/ncgo/hertz/ 后 href 解析到不存在的 docs/ncgo/hertz/kitex/。现有测试 `TestRewriteDocLinks` 与 `TestSyncWritesStandaloneDocsZhCN` 锁定了错误预期，需同步更新。

- [ ] **Step 1: 更新失败测试**

```go
// TestRewriteDocLinks 预期修正：
//   "absolute hertz link":  `docs/hertz/rate-limit-dynamic-design.en.md` → `../hertz/rate-limit-dynamic-design.en.md`
//   "relative kitex link":  [kitex](../kitex/design-doc.en.md) → [kitex](../kitex/design-doc.en.md)  （保持不变）
//   "absolute kitex link":  [kitex docs](docs/kitex/design-doc.en.md) → [kitex docs](../kitex/design-doc.en.md)
//   "absolute micro link":  `docs/micro/design-doc.en.md` → `../micro/design-doc.en.md`
//   "relative micro link":  [micro docs](../micro/design-doc.en.md) → 保持不变
// TestSyncWritesStandaloneDocsZhCN：改为断言包含 ../kitex/ 且不包含 ./kitex/。
// TestSyncWritesStandaloneDocs：原断言（不包含 docs/hertz/ 与 ../hertz/）仍成立，无需改。
```

- [ ] **Step 2: 修改 `rewriteDocLinks`**

```go
for _, origProfile := range []string{"hertz", "kitex", "micro"} {
	oldAbs := "docs/" + origProfile + "/"
	newRel := "../" + origProfile + "/"
	content = strings.ReplaceAll(content, oldAbs, newRel)
	// 注意：../<profile>/ 保持不变（物化后仍是正确兄弟链接）
}
```

- [ ] **Step 3: 新增可解析性断言测试**

```go
// TestStandaloneDocHrefsResolve：Hertz manifest sync 后，扫描 docs/ncgo/** 中所有 `](相对href)`，
// 每个 href 解析到项目内存在文件；绝对/锚点 href 跳过。
```

- [ ] **Step 4: 运行验证**

```bash
go test ./internal/ai/... -run 'TestRewriteDocLinks|TestSyncWritesStandaloneDocs|TestStandaloneDocHrefsResolve' -count=1
go test ./internal/ai/... -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/ai/sync.go internal/ai/sync_test.go
git commit -m "fix(ai): preserve sibling doc links when rewriting standalone docs (#44)"
```

### Task 5: 修复 dry-run 少报（Fix 4）

**Description:** `writeStandaloneDocs` 在 dry-run 时照常为每个 docSpec 追加 `Skip{Path, Reason: "dry-run"}`（与主目标 writeTarget 一致），删除提前 return。

- [ ] **Step 1: 更新失败测试**

```go
// TestSyncDryRunWritesNoStandaloneDocs 增强：断言 res.Skipped 包含 docs/ncgo/hertz/design-doc.en.md（reason=dry-run），
// 且文件确实未创建。workspace 版 TestSyncWorkspaceDryRunWritesNothing 同步补断言。
```

- [ ] **Step 2: 修改 `writeStandaloneDocs`**（移除 `if opts.DryRun { return nil }`；改为循环内对每个 spec 追加 Skip）

- [ ] **Step 3: 运行验证**

```bash
go test ./internal/ai/... -run 'TestSyncDryRun' -count=1
go test ./internal/ai/... -count=1
```

- [ ] **Step 4: 提交**

```bash
git add internal/ai/sync.go internal/ai/sync_test.go
git commit -m "fix(ai): report standalone docs in dry-run output (#44)"
```

### Task 6: 全量验证 + 文档检查

**Description:** 仓库级验证（CI-equivalent），确认无回归。

- [ ] **Step 1: 全量验证**

```bash
go build ./... && go build . && go vet ./... && go test ./... -count=1
```

- [ ] **Step 2: 冒烟**

```bash
./scripts/smoke.sh
```

- [ ] **Step 3: gofmt 检查**

```bash
gofmt -l $(find . -name '*.go' -not -path './.git/*')
```

- [ ] **Step 4: 确认无文档需同步**（若 README/examples 有描述与修复后行为不一致，随对应 commit 更新并跑 markdown 诊断）

### Task 7: 创建 PR 并关联 Issue

**Description:** 创建 PR，body 含 `Closes #44`，并将 Issue 状态标记为 in-review。

- [ ] **Step 1: 运行 link-pr.sh**

```bash
bash /Users/baoyx/.claude/skills/writing-plans-with-issue/scripts/link-pr.sh
```

- [ ] **Step 2: 确认**

```bash
git log --oneline -5
# 输出 PR URL，等待 review 后合并
```

> 若采用本地合并路径，则改为 Task 1 后接 finishing-a-development-branch + finish-issue.sh（push 后自动关闭 #44）。

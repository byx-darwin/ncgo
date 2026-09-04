# Golden Test 覆盖补齐（infra add-on + Kitex+database）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐 `internal/scaffold/mono` 的 "Kitex + database" golden 场景，以及 `internal/scaffold/infra` 的 kafka/es/clickhouse add-on golden test。

**Architecture:** 纯测试新增，遵循两个包里已有的 golden test 模式（`golden.Tree` 用于 mono 整树快照，`golden.File` 用于 infra 逐文件快照）。不修改任何生产代码或模板。

**Tech Stack:** Go testing, 仓库自带的 `internal/testutil/golden` 快照包。

**Spec:** `docs/superpowers/specs/2026-09-04-golden-test-coverage-gap-design.md`

## Global Constraints

- 不修改 `internal/scaffold/mono` / `internal/scaffold/infra` 的生产代码或 `internal/assets/_data/` 模板。
- 新增测试必须遵循现有命名与结构模式（`TestGenerateGolden*`），可用 `-update-golden` 重新生成。
- 最终必须 `go test ./internal/scaffold/... -count=1` 全绿。

---

### Task 1: mono "Kitex + database" golden 场景

**Files:**
- Modify: `internal/scaffold/mono/golden_test.go`（追加一个测试函数，不改动已有函数）
- Test data (generated via `-update-golden`): `internal/scaffold/mono/testdata/mono-kitex-with-database/`

**Interfaces:**
- Consumes: 包内已有的 `goldenOpts(t *testing.T, name string, withDB bool) Options`（`internal/scaffold/mono/golden_test.go:17`）、`Generate(ctx context.Context, opts Options) (*Result, error)`、`manifest.KindKitex`、`golden.Tree(t *testing.T, rel string, gotRoot string)`。
- Produces: 无下游消费者（叶子任务）。

- [ ] **Step 1: 追加新测试函数**

在 `internal/scaffold/mono/golden_test.go` 文件末尾（`TestGenerateGoldenTemplateRuleCenter` 函数之后）追加：

```go

// TestGenerateGoldenKitexWithDatabase covers the Kitex + database combination,
// which was previously untested even though both dimensions are individually
// covered by TestGenerateGoldenKitexDefault and TestGenerateGoldenWithDatabase.
func TestGenerateGoldenKitexWithDatabase(t *testing.T) {
	opts := goldenOpts(t, "demo", true)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	golden.Tree(t, "mono-kitex-with-database", res.Dir)
}
```

- [ ] **Step 2: 运行测试确认因缺 testdata 而失败**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitexWithDatabase -count=1 -v`
Expected: FAIL，报错包含 `golden: read testdata/mono-kitex-with-database/...: no such file or directory (run `go test ... -update-golden` to create)`

- [ ] **Step 3: 生成 golden 快照**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitexWithDatabase -update-golden -count=1`
Expected: 通过，`internal/scaffold/mono/testdata/mono-kitex-with-database/` 目录被创建。

- [ ] **Step 4: 复核生成的快照**

Run: `find internal/scaffold/mono/testdata/mono-kitex-with-database -maxdepth 2`

预期：目录结构应与 `internal/scaffold/mono/testdata/mono-kitex-default/` 基本一致，仅在数据库相关文件（如 `.ncgo/manifest.yaml` 中的 `Infra`/DSN 相关字段、`conf/dev/conf.yaml` 等）上与 `mono-kitex-default` 存在差异 —— 可用 `diff -rq internal/scaffold/mono/testdata/mono-kitex-default internal/scaffold/mono/testdata/mono-kitex-with-database` 快速检查差异集中在预期文件上，未出现意料之外的整批新增/缺失文件。

- [ ] **Step 5: 再次运行测试确认现在通过（无 -update-golden）**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitexWithDatabase -count=1 -v`
Expected: PASS

- [ ] **Step 6: 运行 mono 包全部测试确认无回归**

Run: `go test ./internal/scaffold/mono/... -count=1`
Expected: PASS（全部既有场景 + 新场景）

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/mono/golden_test.go internal/scaffold/mono/testdata/mono-kitex-with-database
git commit -m "test(scaffold): add golden coverage for Kitex + database combination"
```

---

### Task 2: infra kafka golden test

**Files:**
- Modify: `internal/scaffold/infra/golden_test.go`（追加一个测试函数）
- Test data (generated via `-update-golden`): `internal/scaffold/infra/testdata/infra-kafka/`

**Interfaces:**
- Consumes: 包内已有的 `seedProject(t *testing.T, infra []string) string`（`internal/scaffold/infra/infra_test.go:14`）、`Add(opts Options) (*Result, error)`、`Options{Root, Kind, Force, DryRun}`、`Result.WrittenPaths []string`、`manifest.Load`、`golden.File(t *testing.T, rel string, got []byte)`、`goldenReadFile(t *testing.T, path string) []byte`（`internal/scaffold/infra/golden_test.go:97`）、`KindKafka`（`internal/scaffold/infra/infra.go:35`）。
- Produces: 无下游消费者（叶子任务，与 Task 3/4 相互独立，可并行审阅）。

- [ ] **Step 1: 追加新测试函数**

在 `internal/scaffold/infra/golden_test.go` 中，紧接在 `TestGenerateGoldenInfraRedis` 函数之后（`goldenReadFile` 辅助函数之前）追加：

```go

// TestGenerateGoldenInfraKafka locks the output of kafka add-on for a Hertz service.
func TestGenerateGoldenInfraKafka(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindKafka, Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add kafka: %v", err)
	}
	m, _ := manifest.Load(root)
	if !strings.Contains(strings.Join(m.Infra, ","), "kafka") {
		t.Error("manifest infra should include kafka")
	}
	for _, p := range res.WrittenPaths {
		golden.File(t, filepath.Join("infra-kafka", filepath.Base(p)), goldenReadFile(t, p))
	}
}
```

- [ ] **Step 2: 运行测试确认因缺 testdata 而失败**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraKafka -count=1 -v`
Expected: FAIL，报错为 golden 文件缺失（同 Task 1 Step 2 的错误形态）。

- [ ] **Step 3: 生成 golden 快照**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraKafka -update-golden -count=1`
Expected: 通过，`internal/scaffold/infra/testdata/infra-kafka/` 被创建。

- [ ] **Step 4: 再次运行测试确认现在通过**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraKafka -count=1 -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/infra/golden_test.go internal/scaffold/infra/testdata/infra-kafka
git commit -m "test(scaffold): add golden coverage for kafka infra add-on"
```

---

### Task 3: infra es golden test

**Files:**
- Modify: `internal/scaffold/infra/golden_test.go`（追加一个测试函数，紧接 Task 2 新增函数之后）
- Test data (generated via `-update-golden`): `internal/scaffold/infra/testdata/infra-es/`

**Interfaces:**
- Consumes: 同 Task 2（`seedProject`、`Add`、`Options`、`Result.WrittenPaths`、`manifest.Load`、`golden.File`、`goldenReadFile`）、`KindES`（`internal/scaffold/infra/infra.go:36`）。
- Produces: 无下游消费者。

- [ ] **Step 1: 追加新测试函数**

紧接 Task 2 的 `TestGenerateGoldenInfraKafka` 之后追加：

```go

// TestGenerateGoldenInfraES locks the output of es add-on for a Hertz service.
func TestGenerateGoldenInfraES(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindES, Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add es: %v", err)
	}
	m, _ := manifest.Load(root)
	if !strings.Contains(strings.Join(m.Infra, ","), "es") {
		t.Error("manifest infra should include es")
	}
	for _, p := range res.WrittenPaths {
		golden.File(t, filepath.Join("infra-es", filepath.Base(p)), goldenReadFile(t, p))
	}
}
```

- [ ] **Step 2: 运行测试确认因缺 testdata 而失败**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraES -count=1 -v`
Expected: FAIL（golden 文件缺失）。

- [ ] **Step 3: 生成 golden 快照**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraES -update-golden -count=1`
Expected: 通过，`internal/scaffold/infra/testdata/infra-es/` 被创建。

- [ ] **Step 4: 再次运行测试确认现在通过**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraES -count=1 -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/infra/golden_test.go internal/scaffold/infra/testdata/infra-es
git commit -m "test(scaffold): add golden coverage for es infra add-on"
```

---

### Task 4: infra clickhouse golden test

**Files:**
- Modify: `internal/scaffold/infra/golden_test.go`（追加一个测试函数，紧接 Task 3 新增函数之后）
- Test data (generated via `-update-golden`): `internal/scaffold/infra/testdata/infra-clickhouse/`

**Interfaces:**
- Consumes: 同 Task 2/3（`seedProject`、`Add`、`Options`、`Result.WrittenPaths`、`manifest.Load`、`golden.File`、`goldenReadFile`）、`KindClickHouse`（`internal/scaffold/infra/infra.go:37`）。
- Produces: 无下游消费者。

- [ ] **Step 1: 追加新测试函数**

紧接 Task 3 的 `TestGenerateGoldenInfraES` 之后追加：

```go

// TestGenerateGoldenInfraClickHouse locks the output of clickhouse add-on for a Hertz service.
func TestGenerateGoldenInfraClickHouse(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindClickHouse, Force: false, DryRun: false})
	if err != nil {
		t.Fatalf("Add clickhouse: %v", err)
	}
	m, _ := manifest.Load(root)
	if !strings.Contains(strings.Join(m.Infra, ","), "clickhouse") {
		t.Error("manifest infra should include clickhouse")
	}
	for _, p := range res.WrittenPaths {
		golden.File(t, filepath.Join("infra-clickhouse", filepath.Base(p)), goldenReadFile(t, p))
	}
}
```

- [ ] **Step 2: 运行测试确认因缺 testdata 而失败**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraClickHouse -count=1 -v`
Expected: FAIL（golden 文件缺失）。

- [ ] **Step 3: 生成 golden 快照**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraClickHouse -update-golden -count=1`
Expected: 通过，`internal/scaffold/infra/testdata/infra-clickhouse/` 被创建。

- [ ] **Step 4: 再次运行测试确认现在通过**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGoldenInfraClickHouse -count=1 -v`
Expected: PASS

- [ ] **Step 5: 运行 infra 包全部测试确认无回归**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS（全部既有场景 + kafka/es/clickhouse 三个新场景）

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/golden_test.go internal/scaffold/infra/testdata/infra-clickhouse
git commit -m "test(scaffold): add golden coverage for clickhouse infra add-on"
```

---

### Task 5: 全量验证

**Files:**
- 无新文件；只运行验证命令。

**Interfaces:**
- Consumes: Task 1-4 的全部产出。
- Produces: 无（终结任务）。

- [ ] **Step 1: 跑 scaffold 包整体测试**

Run: `go test ./internal/scaffold/... -count=1`
Expected: PASS，全绿，无 flaky/跳过。

- [ ] **Step 2: 跑仓库级 build + vet 做最小回归确认**

Run: `go build ./... && go vet ./...`
Expected: 无输出、无错误退出码。

- [ ] **Step 3: gofmt 检查改动文件**

Run: `gofmt -l internal/scaffold/mono/golden_test.go internal/scaffold/infra/golden_test.go`
Expected: 空输出（已格式化）。

（本任务不产生新 commit；如 Step 1-3 发现问题，回到对应 Task 修复后重新提交。）

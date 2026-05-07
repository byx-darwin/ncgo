# Write Tests

Use this skill whenever adding or changing behavior. Tests are not optional.

## Decision: which level to write

| What changed | Write |
| --- | --- |
| Pure logic, helper, formatter, schema parser, output builder | Unit test |
| CLI command, flag, JSON output, MCP tool | Integration test |
| New scaffold template or generated file tree | Golden test |
| New CLI subcommand or MCP tool registered | Smoke / registration test |

Write the smallest level that gives reliable signal. Add a higher level only when the lower level cannot cover the contract.

---

## 1. Unit Tests

Place in the same package as the code under test (`package foo`, not `package foo_test`).

```go
func TestMyFuncHappyPath(t *testing.T) {
    got, err := MyFunc(input)
    if err != nil {
        t.Fatalf("MyFunc: %v", err)
    }
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

**Patterns used in ncgo:**
- `t.TempDir()` for isolated filesystem state
- `seedManifest(t, ...)` helper to bootstrap a valid project root
- Table-driven tests (`for _, tc := range cases`) for multiple input variants
- Test error paths explicitly: conflict, missing file, invalid input

---

## 2. Integration Tests (CLI)

Place in `internal/cli/`, same package `cli`. Wire a real `cobra.Command` and capture output.

```go
func TestRunAddInfraTextOutput(t *testing.T) {
    root := seedAddInfraProject(t)
    var out bytes.Buffer
    cmd := &cobra.Command{}
    cmd.SetOut(&out)

    err := runAddInfra(cmd, infra.KindRedis, &addInfraOptions{root: root})
    if err != nil {
        t.Fatalf("runAddInfra: %v", err)
    }
    if !strings.Contains(out.String(), "wrote ") {
        t.Fatalf("unexpected output:\n%s", out.String())
    }
}
```

**What to assert:**
- text output contains expected keywords (`"wrote "`, `"next steps:"`, `"ok"`)
- JSON output parses cleanly and top-level fields match schema
- files were written (or NOT written in `--plan` / `--dry-run` mode)
- error message text when inputs are invalid

**For MCP tools**, assert `content[0].text` is non-empty and structured fields are present.

---

## 3. Golden Tests (scaffold / template output)

Use `internal/testutil/golden`. Snapshots live in `testdata/` next to the test file.

```go
func TestGenerateGoldenMyCase(t *testing.T) {
    res, err := Generate(context.Background(), opts)
    if err != nil {
        t.Fatalf("Generate: %v", err)
    }
    golden.Tree(t, "my-case", res.Dir)   // entire directory tree
    // or for a single file:
    golden.File(t, "my-case/manifest.yaml", got)
}
```

**To regenerate after an intentional template change:**
```bash
go test ./internal/scaffold/mono/... -run Golden -update-golden
```

Rules:
- MUST update golden fixtures when templates change; do not leave stale snapshots
- MUST commit the updated `testdata/` alongside the template change
- Use a descriptive case name (`mono-default`, `mono-kitex-default`, `domain-device`)

---

## 4. Smoke / Registration Tests

Verify that commands and MCP tools are wired up. Cheap and fast.

```go
func TestRootCmdIncludesMyCommand(t *testing.T) {
    cmd, _, err := newRootCmd().Find([]string{"my", "command"})
    if err != nil {
        t.Fatalf("Find: %v", err)
    }
    if cmd == nil || cmd.Name() != "command" {
        t.Fatalf("command not registered")
    }
}
```

For full binary smoke checks use `./scripts/smoke.sh` (bash + Python MCP frame test). Do not add new bash to `smoke.sh` for normal logic coverage; use it only for end-to-end binary path verification.

---

## Checklist before finishing

- [ ] happy path covered
- [ ] at least one error / conflict path covered
- [ ] `--plan` / `--dry-run` mode verified to NOT write files (if applicable)
- [ ] golden fixtures updated if template output changed
- [ ] new CLI subcommand or MCP tool has a registration test
- [ ] run `go test ./internal/<pkg>/... -count=1` and confirm green

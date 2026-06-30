# Scaffold Code Changes — hertz-template Integration

**Date**: 2026-06-30
**Status**: Draft
**Author**: ncgo team
**Depends on**: `01-kitex-template-go-tools-integration.md`, `02-hertz-template-design.md`

## 1. Goal

Define code changes needed to integrate hertz-template into the scaffold flow. This covers:

1. kitex-template go-tools updates
2. hertz-template creation and embedding
3. scaffold flow changes (writeHertzTemplate + Apply)
4. test updates

## 2. Affected Files

```
Changed files:
  internal/assets/_data/kitex/kitex-template/*
    [main.yaml, conf.yaml, conf_dev.yaml, server.yaml, interceptor.yaml, rpcerror.yaml]
  internal/assets/_data/hertz/hertz-template/*
    [12 new files: main_go.yaml, conf_go.yaml, conf_dev_yaml.yaml, server_go.yaml,
     data_go.yaml, usecase_go.yaml, repository_go.yaml, response_go.yaml,
     errcode_go.yaml, middleware_go.yaml, makefile_yaml.yaml, sqlc_yaml.yaml]
  internal/assets/_data/hertz/layout.yaml
    [remove body from files now managed by hertz-template]
  internal/scaffold/mono/files.go
    [writeHertzTemplate: copy hertz-template/*.yaml]
  internal/scaffold/mono/golden_test.go / testdata/
    [update golden snapshots]
  internal/scaffold/template/apply.go
    [no changes needed — Apply() already reads hertz-template if it exists]
```

## 3. Change Breakdown

### Change 1: Update kitex-template (6 files)

**Files**: `internal/assets/_data/kitex/kitex-template/{main,conf,conf_dev,server,interceptor,rpcerror}.yaml`

Per `01-kitex-template-go-tools-integration.md` section 4:
- `main.yaml`: Import `go-common/log`, add `goclog.Init()` call
- `conf.yaml`: Use `go-framework/config.LoadYAML[T]()`, embed `kitexconfig.ServerConfig`
- `conf_dev.yaml`: Update YAML format (int seconds → time.Duration strings)
- `server.yaml`: Use `kitexconfig.ServerConfig` fields, add observability integration
- `interceptor.yaml`: Delegate `AccessLog()` to `go-framework/kitex/middleware`
- `rpcerror.yaml`: Use `go-common/error` codes, use `rpcerror.OopsStatusAdapter`

**Risk**: Config format change is backward-incompatible (int → Duration). Only affects new scaffolds, not existing projects.

### Change 2: Create hertz-template (12 new files)

**Files**: `internal/assets/_data/hertz/hertz-template/*.yaml` (new directory)

All 12 files per `02-hertz-template-design.md` section 3:
1. `main_go.yaml` — go-common/log init
2. `conf_go.yaml` — go-framework/config loading + hertz.ServerConfig
3. `conf_dev_yaml.yaml` — new config format
4. `server_go.yaml` — go-framework/hertz.NewHTTPServer
5. `data_go.yaml` — go-middleware/db（条件 WithDatabase）
6. `usecase_go.yaml` — 业务逻辑层（loop_service, skip）
7. `repository_go.yaml` — 数据访问层（loop_service, skip, WithDatabase）
8. `response_go.yaml` — go-framework/hertz.Responder 封装
9. `errcode_go.yaml` — go-common/error 错误码
10. `middleware_go.yaml` — go-framework/hertz/middleware 重导出
11. `makefile_yaml.yaml` — Makefile
12. `sqlc_yaml.yaml` — sqlc 配置（条件 WithDatabase）

**Format**: Each file follows `TemplateFile` schema:
```yaml
path: <target>
update_behavior:
  type: cover|skip
loop_service: true|false
body: |-
  <Go text/template body>
```

**Template variables**: `{{.Module}}`, `{{.ServiceName}}`, `{{.ServiceInfo.ServiceName}}`, `{{.ServiceInfo.Methods}}`, `{{ToLower .ServiceName}}`, `{{.WithDatabase}}`, `{{.Infra}}`

### Change 3: Trim layout.yaml

**File**: `internal/assets/_data/hertz/layout.yaml`

Remove body content from files now managed by hertz-template. Keep empty directory entries so hz creates the necessary directory structure.

**Files to strip body from**:
```
main.go
internal/base/conf/conf.go
internal/base/server/server.go
internal/pkg/response/response.go
internal/pkg/errcode/errcode.go
internal/pkg/middleware/*.go (all middleware files)
internal/base/data/data.go
conf/dev/conf.yaml
```

**Files to KEEP** (hz still generates them):
```
internal/handler/*/handler.go      (hz package.yaml → body has proto methods)
internal/router/*.go               (hz route registration → depends on proto annotations)
internal/pb/*.go                   (hz protobuf generation)
internal/pkg/i18n/*                (hz i18n toolchain)
internal/docs/*                    (hz swagger/openapi)
internal/handler/health/health.go  (hz built-in health endpoint)
go.mod                             (hz generates module path)
internal/pkg/ratelimit/*           (rate-limit middleware)
internal/pkg/middleware/*_test.go  (test files — skip from template export)
tools/i18n/*                       (i18n CLI tools)
```

**Approach**: Change `body: |-\n...` to `body: ""` for each stripped entry. The entry still defines the directory/file path, so hz creates the empty file or directory.

### Change 4: writeHertzTemplate()

**File**: `internal/scaffold/mono/files.go`

Add code to copy `hertz/hertz-template/*.yaml` from embedded assets to `<dir>/template/hertz-template/`.

```go
func writeHertzTemplate(dir string, opts Options) error {
    tplDir := filepath.Join(dir, "template")
    if err := os.MkdirAll(tplDir, 0o755); err != nil {
        return fmt.Errorf("scaffold: mkdir %s: %w", tplDir, err)
    }

    // Existing: copy layout.yaml, package.yaml, data.json
    srcFS := assets.FS()
    for _, name := range []string{"layout.yaml", "package.yaml"} {
        b, err := fs.ReadFile(srcFS, "hertz/"+name)
        if err != nil {
            return fmt.Errorf("scaffold: read embedded hertz/%s: %w", name, err)
        }
        if err := os.WriteFile(filepath.Join(tplDir, name), b, 0o644); err != nil {
            return fmt.Errorf("scaffold: write %s: %w", name, err)
        }
    }
    data, err := renderDataJSON(opts)
    if err != nil {
        return fmt.Errorf("scaffold: render data.json: %w", err)
    }
    if err := os.WriteFile(filepath.Join(tplDir, "data.json"), data, 0o644); err != nil {
        return fmt.Errorf("scaffold: write data.json: %w", err)
    }

    // NEW: copy hertz-template/*.yaml
    hertzTplDir := filepath.Join(tplDir, "hertz-template")
    if err := os.MkdirAll(hertzTplDir, 0o755); err != nil {
        return fmt.Errorf("scaffold: mkdir %s: %w", hertzTplDir, err)
    }
    entries, err := fs.ReadDir(srcFS, "hertz/hertz-template")
    if err != nil {
        return nil // Assets directory not initialized yet — non-fatal
    }
    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
            continue
        }
        b, err := fs.ReadFile(srcFS, "hertz/hertz-template/"+e.Name())
        if err != nil {
            return fmt.Errorf("scaffold: read embedded hertz/hertz-template/%s: %w", e.Name(), err)
        }
        tgt := filepath.Join(hertzTplDir, e.Name())
        if err := os.WriteFile(tgt, b, 0o644); err != nil {
            return fmt.Errorf("scaffold: write %s: %w", tgt, err)
        }
    }

    // Existing: rule_center_client.go
    if opts.RuleCenterAddr != "" {
        // ... unchanged ...
    }
    return nil
}
```

### Change 5: template.Apply() integration

**File**: `internal/scaffold/mono/mono.go` (no code change needed)

Current code already calls `template.Apply()` after `runGenerator()`:
```go
if defaultKind(opts.Kind) == manifest.KindHertz {
    services, _ := scaffoldtemplate.ParseAllServices(ctx, filepath.Join(dir, idl), opts.Module)
    _, _ = scaffoldtemplate.Apply(scaffoldtemplate.ApplyOptions{
        Root:         dir,
        Module:       opts.Module,
        ServiceName:  opts.Name,
        WithDatabase: opts.WithDatabase,
        Infra:        opts.Infra,
        Services:     services,
    })
}
```

Apply() reads `template/hertz-template/*.yaml` — now populated by writeHertzTemplate(). **No code change required**.

Important: hz must run BEFORE Apply() because:
1. hz creates the directory structure and generates handler/router/pb
2. Apply() overlays hertz-template files on top (some cover hz-generated files, some add new files)

### Change 6: RenderData extension (optional)

**File**: `internal/scaffold/template/types.go`

If hertz-template needs infra-specific conditionals (e.g., `{{if .WithDatabase}}...{{end}}`), the existing `RenderData` struct already supports this. But adding a helper function to `FuncMap()` for infra checks may be needed:

```go
func FuncMap() template.FuncMap {
    return template.FuncMap{
        "ToLower":    strings.ToLower,
        "ToUpper":    strings.ToUpper,
        "LowerFirst": lowerFirst,
        "exportName": exportName,
        "hasInfra":   hasInfra,  // NEW
    }
}

func hasInfra(infra []string, name string) bool {
    for _, kind := range infra {
        if kind == name {
            return true
        }
    }
    return false
}
```

Template usage:
```yaml
{{if hasInfra .Infra "redis"}}
redisClient, _ := redis.NewUniversalClient(ctx, cfg.Redis)
{{end}}
```

## 4. Test Updates

### 4.1 Golden tests

**File**: `internal/scaffold/mono/testdata/` (snapshots)

Run `go test ./internal/scaffold/mono/... -update-golden` to regenerate snapshots after layout.yaml and template changes.

Expected changes in golden output:
- kitex-template: Config types change (ServerConfig → kitexconfig.ServerConfig), new imports
- hertz-template: New files appear in project tree (covered by Apply())
- layout.yaml: Some files have empty body (will be overwritten by Apply())

### 4.2 Integration test

**File**: `internal/scaffold/mono/mono_test.go`

Add test case for hertz-template coverage:
```go
func TestGenerateHertzTemplateApplied(t *testing.T) {
    dir := t.TempDir()
    _, err := Generate(ctx, Options{
        Name:   "testapp",
        Module: "example.com/testapp",
        Kind:   manifest.KindHertz,
        Dir:    dir,
        NoGenerate: true,  // Don't run real hz
        NCGOVersion: "v1.0.0",
        Now: time.Now(),
    })
    if err != nil {
        t.Fatal(err)
    }
    // Verify hertz-template directory exists
    tplDir := filepath.Join(dir, "template", "hertz-template")
    entries, _ := os.ReadDir(tplDir)
    if len(entries) == 0 {
        t.Error("expected hertz-template/*.yaml files")
    }
}
```

### 4.3 Smoke test

Run `./scripts/smoke.sh` after changes to verify end-to-end scaffold.

## 5. Implementation Order

1. **Create empty directory** `internal/assets/_data/hertz/hertz-template/`
2. **Write template files** (12 yaml files) — starting with simplest (errcode, middleware) → complex (conf, server)
3. **Update kitex-template** (6 yaml files) — go-tools integration
4. **Trim layout.yaml** — remove body from files now managed by hertz-template
5. **Update writeHertzTemplate()** — copy yaml files
6. **Add hasInfra to FuncMap()** — conditional infra rendering
7. **Update golden tests** — `-update-golden`
8. **Add integration test** — verify hertz-template copy + Apply
9. **Run full validation** — `go build ./... && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`

## 6. Rollback Plan

If hertz-template causes issues:
1. Revert `writeHertzTemplate()` to not copy hertz-template yaml
2. Revert layout.yaml changes (restore original body content)
3. `template.Apply()` returns empty result (finds no yaml) — graceful degradation

## 7. Success Criteria

1. `ncgo new --mode mono --kind hertz` generates compilable project
2. `ncgo new --mode mono --kind kitex` generates compilable project (updated with go-tools)
3. Generated projects reference go-tools packages
4. Golden tests pass
5. All existing tests pass
6. Smoke script passes

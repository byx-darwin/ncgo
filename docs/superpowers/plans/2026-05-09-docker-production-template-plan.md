# Docker 生产级模板实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 ncgo 生成的下游服务项目提供生产就绪的 Dockerfile、compose.yaml 和 Makefile Docker 入口。

**Architecture:** 修改 container.go 中的 Dockerfile/compose 模板字符串，在 Hertz layout.yaml 和 Kitex makefile.yaml 中添加 Docker Make targets，更新 golden test fixtures。

**Tech Stack:** Go, Docker, Docker Compose, Make

---

### Task 1: 增强 Dockerfile 模板

**Files:**
- Modify: `internal/scaffold/shared/container.go:64-84` (WriteServiceContainerFiles 函数中的 Dockerfile 模板)
- Test: `internal/scaffold/mono/testdata/mono-default/Dockerfile`
- Test: `internal/scaffold/mono/testdata/mono-with-database/Dockerfile`
- Test: `internal/scaffold/mono/testdata/mono-kitex-default/Dockerfile`

- [ ] **Step 1: 更新 container.go 中的 Dockerfile 模板**

在 `WriteServiceContainerFiles` 函数中，将现有的 Dockerfile 模板（第64-84行）替换为增强版本：

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_NAME=app
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

WORKDIR /app
COPY --from=builder --chown=appuser:appgroup /out/app ./app
USER appuser

ENV GO_ENV=docker

EXPOSE %d

ENTRYPOINT ["./app"]
```

关键变化：
- Builder 基础镜像改为 `golang:1.22-alpine`（更小）
- 添加模块缓存层：先 `COPY go.mod go.sum` → `go mod download`
- 添加 `ARG APP_NAME=app` 构建参数
- 运行镜像添加 `tzdata`
- 创建非 root 用户 `appuser`（UID 1000）
- `USER appuser` 切换运行用户

将 `WriteServiceContainerFiles` 函数中的 dockerfile 变量赋值修改为：

```go
dockerfile := fmt.Sprintf(`FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_NAME=app
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

WORKDIR /app
COPY --from=builder --chown=appuser:appgroup /out/app ./app
USER appuser

ENV GO_ENV=docker

EXPOSE %d

ENTRYPOINT ["./app"]
`, port)
```

- [ ] **Step 2: 运行 `go build ./...` 确认编译通过**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go build ./...`
Expected: 无错误

- [ ] **Step 3: 更新 golden test fixtures**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/mono/... -run TestGenerateGolden -update-golden -count=1`
Expected: 三个 golden Dockerfile 更新为新的模板格式

- [ ] **Step 4: 验证 golden 测试通过**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/mono/... -run TestGenerateGolden -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/scaffold/shared/container.go
git add internal/scaffold/mono/testdata/*/Dockerfile
git commit -m "feat(scaffold): enhance Dockerfile with cache layers, non-root user, and tzdata"
```

---

### Task 2: 增强 compose.yaml — 应用服务

**Files:**
- Modify: `internal/scaffold/shared/container.go:276-301` (renderAppCompose 函数)

- [ ] **Step 1: 更新 renderAppCompose 函数**

将 `renderAppCompose` 函数（第276-301行）替换为增强版本：

```go
func renderAppCompose(b *strings.Builder, app composeApp) error {
	containerPort, err := servicePort(app.Kind)
	if err != nil {
		return err
	}
	features := composeFeaturesForApp(app)
	fmt.Fprintf(b, "  %s:\n", app.Name)
	b.WriteString("    build:\n")
	fmt.Fprintf(b, "      context: %s\n", app.Context)
	b.WriteString("      dockerfile: Dockerfile\n")
	b.WriteString("    environment:\n")
	b.WriteString("      GO_ENV: docker\n")
	if app.WithDatabase {
		fmt.Fprintf(b, "      DATABASE_URL: postgres://postgres:${POSTGRES_PASSWORD:-postgres}@postgres:5432/%s?sslmode=disable\n", app.Name)
	}
	deps := dependencyServiceNames(features)
	if len(deps) > 0 {
		b.WriteString("    depends_on:\n")
		for _, dep := range deps {
			fmt.Fprintf(b, "      %s:\n        condition: service_healthy\n", dep)
		}
	}
	b.WriteString("    ports:\n")
	fmt.Fprintf(b, "      - \"%d:%d\"\n", app.HostPort, containerPort)
	b.WriteString("    restart: unless-stopped\n")
	b.WriteString("    logging:\n")
	b.WriteString("      driver: json-file\n")
	b.WriteString("      options:\n")
	b.WriteString("        max-size: \"10m\"\n")
	b.WriteString("        max-file: \"3\"\n")
	return nil
}
```

关键变化：
- `depends_on` 从简单列表改为 `condition: service_healthy` 格式
- `DATABASE_URL` 密码参数化：`${POSTGRES_PASSWORD:-postgres}`
- 添加 `restart: unless-stopped`
- 添加 `logging` 日志限制

- [ ] **Step 2: 运行 `go build ./...` 确认编译通过**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go build ./...`
Expected: 无错误

- [ ] **Step 3: 更新 golden test fixtures**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/mono/... -run TestGenerateGolden -update-golden -count=1`
Expected: 三个 golden compose.yaml 更新为新格式

- [ ] **Step 4: 运行 container_test.go 测试**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/shared/... -count=1 -v`
Expected: 部分测试可能失败（需要更新测试断言），这是预期的

- [ ] **Step 5: 提交**

```bash
git add internal/scaffold/shared/container.go
git add internal/scaffold/mono/testdata/*/compose.yaml
git commit -m "feat(scaffold): enhance compose app service with healthcheck deps, restart policy, and logging"
```

---

### Task 3: 增强 compose.yaml — PostgreSQL 和 Redis 服务

**Files:**
- Modify: `internal/scaffold/shared/container.go:303-324` (renderPostgresCompose 和 renderRedisCompose 函数)
- Modify: `internal/scaffold/shared/container_test.go` (测试断言更新)

- [ ] **Step 1: 更新 renderPostgresCompose**

```go
func renderPostgresCompose(b *strings.Builder) {
	b.WriteString("  postgres:\n")
	b.WriteString("    image: postgres:16-alpine\n")
	b.WriteString("    environment:\n")
	b.WriteString("      POSTGRES_DB: app\n")
	b.WriteString("      POSTGRES_USER: postgres\n")
	b.WriteString("      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres}\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"5432:5432\"\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - postgres-data:/var/lib/postgresql/data\n")
	b.WriteString("    healthcheck:\n")
	b.WriteString("      test: [\"CMD-SHELL\", \"pg_isready -U postgres\"]\n")
	b.WriteString("      interval: 10s\n")
	b.WriteString("      timeout: 5s\n")
	b.WriteString("      retries: 5\n")
	b.WriteString("      start_period: 10s\n")
	b.WriteString("    restart: unless-stopped\n")
}
```

- [ ] **Step 2: 更新 renderRedisCompose**

```go
func renderRedisCompose(b *strings.Builder) {
	b.WriteString("  redis:\n")
	b.WriteString("    image: redis:7-alpine\n")
	b.WriteString("    command: [\"redis-server\", \"--appendonly\", \"yes\"]\n")
	b.WriteString("    ports:\n")
	b.WriteString("      - \"6379:6379\"\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - redis-data:/data\n")
	b.WriteString("    healthcheck:\n")
	b.WriteString("      test: [\"CMD\", \"redis-cli\", \"ping\"]\n")
	b.WriteString("      interval: 10s\n")
	b.WriteString("      timeout: 3s\n")
	b.WriteString("      retries: 3\n")
	b.WriteString("    restart: unless-stopped\n")
}
```

- [ ] **Step 3: 运行测试**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/shared/... -count=1 -v`
Expected: 测试可能需要更新断言（如 `TestRenderComposeProjectIncludesDependenciesAndProfiles`），见 Step 4

- [ ] **Step 4: 更新 container_test.go 断言**

`TestRenderComposeProjectIncludesDependenciesAndProfiles` 需要确认 `healthcheck`、`restart`、`${POSTGRES_PASSWORD:-postgres}` 出现在输出中。将测试中已有的断言列表保持不变（向后兼容），添加新断言：

```go
for _, want := range []string{
	// ... 已有断言 ...
	"healthcheck:",
	"restart: unless-stopped",
	"POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres}",
	"pg_isready",
	"redis-cli",
} {
	if !strings.Contains(body, want) {
		t.Fatalf("compose body missing %q\n---\n%s", want, body)
	}
}
```

- [ ] **Step 5: 更新 golden test fixtures**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/mono/... -run TestGenerateGolden -update-golden -count=1`

- [ ] **Step 6: 运行全部测试确认通过**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/shared/... ./internal/scaffold/mono/... -count=1`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/scaffold/shared/container.go
git add internal/scaffold/shared/container_test.go
git add internal/scaffold/mono/testdata/*/compose.yaml
git commit -m "feat(scaffold): add healthcheck and restart to postgres and redis compose services"
```

---

### Task 4: 为 Hertz Makefile 添加 Docker targets

**Files:**
- Modify: `internal/assets/_data/hertz/layout.yaml` (第8334-8397行的 Makefile body 部分)
- Modify: `internal/scaffold/mono/testdata/mono-default/template/layout.yaml` (golden fixture，需同步更新)
- Modify: `internal/scaffold/mono/testdata/mono-with-database/template/layout.yaml` (golden fixture，需同步更新)

- [ ] **Step 1: 在 Hertz layout.yaml 的 Makefile body 中添加 Docker targets**

在 `.PHONY` 行（8344行）中添加 Docker targets：

**旧：**
```
.PHONY: build run dev i18n-sync i18n-report i18n-check i18n-check-release i18n update swagger generate sqlc migrate-create migrate-up migrate-down migrate-status lint test check tidy clean install-tools
```

**新：**
```
.PHONY: build run dev i18n-sync i18n-report i18n-check i18n-check-release i18n update swagger generate sqlc migrate-create migrate-up migrate-down migrate-status lint test check tidy clean install-tools docker-build docker-up docker-down docker-logs docker-shell docker-start docker-rebuild
```

在 `clean` target 之前（第8396行之前）添加 Docker section：

```makefile
      # ── Docker ─────────────────────────────────────────────────────────────
      IMAGE_TAG ?= latest

      docker-build: ; @docker build -t $(APP_NAME):$(IMAGE_TAG) .

      docker-up: ; @docker compose up -d

      docker-down: ; @docker compose down

      docker-logs: ; @docker compose logs -f $(APP_NAME)

      docker-shell: ; @docker compose exec $(APP_NAME) sh

      docker-start: docker-build docker-up ; @echo "Docker services started"

      docker-rebuild: docker-down docker-build docker-up ; @echo "Docker services rebuilt and restarted"

```

- [ ] **Step 2: 同步更新 golden fixtures 中的 layout.yaml**

由于 golden tests 使用 `golden.Tree` 捕获完整输出树，而 layout.yaml 是嵌入模板文件（由 mono.Generate 写出），需要确认 golden fixture 是否包含 layout.yaml。检查：

```bash
ls /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo/internal/scaffold/mono/testdata/mono-default/template/
```

如果 golden fixture 包含 layout.yaml，则需要更新。运行 `-update-golden` 会自动处理。

- [ ] **Step 3: 运行测试**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go build ./... && go test ./internal/scaffold/mono/... -run TestGenerateGolden -count=1`
Expected: PASS（golden fixture 可能需要 `-update-golden`）

- [ ] **Step 4: 提交**

```bash
git add internal/assets/_data/hertz/layout.yaml
git add internal/scaffold/mono/testdata/*/template/layout.yaml
git commit -m "feat(scaffold): add Docker targets to Hertz Makefile template"
```

---

### Task 5: 为 Kitex Makefile 添加 Docker targets

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/makefile.yaml`
- Modify: `internal/scaffold/mono/testdata/mono-kitex-default/template/kitex-template/makefile.yaml` (golden fixture)

- [ ] **Step 1: 在 Kitex makefile.yaml 中添加 Docker targets**

在 `.PHONY` 行（第17行）中添加 Docker targets：

**旧：**
```
.PHONY: build run dev update generate sqlc migrate-up migrate-down migrate-status migrate-create lint test clean install-tools tidy
```

**新：**
```
.PHONY: build run dev update generate sqlc migrate-up migrate-down migrate-status migrate-create lint test clean install-tools tidy docker-build docker-up docker-down docker-logs docker-shell docker-start docker-rebuild
```

在 `clean` target 之前（第59行之前）添加：

```makefile
  # ── Docker ────────────────────────────────────────────────────────────────
  IMAGE_TAG ?= latest

  docker-build: ; @docker build -t $(APP_NAME):$(IMAGE_TAG) .

  docker-up: ; @docker compose up -d

  docker-down: ; @docker compose down

  docker-logs: ; @docker compose logs -f $(APP_NAME)

  docker-shell: ; @docker compose exec $(APP_NAME) sh

  docker-start: docker-build docker-up ; @echo "Docker services started"

  docker-rebuild: docker-down docker-build docker-up ; @echo "Docker services rebuilt and restarted"

```

- [ ] **Step 2: 更新 golden fixture**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitexDefault -update-golden -count=1`

- [ ] **Step 3: 运行测试**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go test ./internal/scaffold/mono/... -run TestGenerateGolden -count=1`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/assets/_data/kitex/kitex-template/makefile.yaml
git add internal/scaffold/mono/testdata/mono-kitex-default/template/kitex-template/makefile.yaml
git commit -m "feat(scaffold): add Docker targets to Kitex Makefile template"
```

---

### Task 6: 全量验证

- [ ] **Step 1: 构建**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go build ./...`
Expected: 无错误

- [ ] **Step 2: 测试**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && go vet ./... && go test ./... -count=1`
Expected: 全部 PASS

- [ ] **Step 3: 冒烟测试**

Run: `cd /Users/baoyx/Documents/workspace/github.com/byx-darwin/ncgo && ./scripts/smoke.sh`
Expected: 全部 PASS

- [ ] **Step 4: 最终提交（如需要修复）**

```bash
git add -A
git commit -m "chore: final validation fixes"
```

  APP_NAME = demo-http
  MODULE   = github.com/x/demo
  IDL_FILE = idl/demo.proto
  GOOSE_DRIVER ?= postgres
  MIGRATIONS_DIR ?= internal/db/migrations
  DATABASE_URL ?=

  .PHONY: build run dev update generate sqlc migrate-up migrate-down migrate-status migrate-create lint test clean install-tools tidy

  # ── Build ────────────────────────────────────────────────────────────────
  build: ; @echo "Building $(APP_NAME)..."; mkdir -p output/bin; go build -o output/bin/$(APP_NAME) .; echo "Build complete → output/bin/$(APP_NAME)"

  run: build ; @./output/bin/$(APP_NAME)

  dev: ; @which air > /dev/null 2>&1 && air || go run .

  # ── Code generation ──────────────────────────────────────────────────────
  # Re-run after modifying the IDL file.
  update: ; @echo "Generating Hertz HTTP code from IDL..."; hz new --mod=$(MODULE) --idl=$(IDL_FILE) -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml; echo "Hertz code generation complete"

  # Generate type-safe DB query code from SQL files in internal/db/.
  sqlc: ; @echo "Generating sqlc DB code..."; sqlc generate -f internal/db/sqlc.yaml; echo "sqlc generation complete"

  generate: update sqlc ; @echo "All code generation complete"

  # ── Database migrations ──────────────────────────────────────────────────
  migrate-up: ; @test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" && exit 1); goose -dir $(MIGRATIONS_DIR) $(GOOSE_DRIVER) "$(DATABASE_URL)" up

  migrate-down: ; @test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" && exit 1); goose -dir $(MIGRATIONS_DIR) $(GOOSE_DRIVER) "$(DATABASE_URL)" down

  migrate-status: ; @test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" && exit 1); goose -dir $(MIGRATIONS_DIR) $(GOOSE_DRIVER) "$(DATABASE_URL)" status

  migrate-create: ; @test -n "$(name)" || (echo "usage: make migrate-create name=create_users" && exit 1); goose -dir $(MIGRATIONS_DIR) create $(name) sql

  # ── Quality ──────────────────────────────────────────────────────────────
  lint: ; @golangci-lint run ./...

  test: ; @go test -race -count=1 ./...

  check: lint test

  # ── Dependencies ─────────────────────────────────────────────────────────
  tidy: ; @go mod tidy

  # ── Install tools ────────────────────────────────────────────────────────
  install-tools: ; @echo "Installing development tools..."; go install github.com/cloudwego/hertz/cmd/hz@latest; go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest; go install github.com/pressly/goose/v3/cmd/goose@latest; go install github.com/air-verse/air@latest; go install golang.org/x/tools/cmd/goimports@latest; echo "Install complete"

  # ── Cleanup ──────────────────────────────────────────────────────────────
  clean: ; @rm -rf output/

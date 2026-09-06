# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.3] - 2026-09-06

### Fixed

- **Scaffold**: the base Hertz `usecase.go` stub unconditionally imported `context` and go-tools' `goerror` package, both unused until a real method is implemented — every fresh `ncgo new --kind hertz` project failed `go build`/`go vet` immediately. Confirmed fixed by dogfood-generating a project end to end.
- **`ncgo add infra registry_polaris`**: the printed `go get github.com/kitex-contrib/polaris` next-step failed because that module has no tagged releases; pinned to `@main`, which resolves.
- **`ncgo doctor`**: `compose.dockerfile.<svc>` false-positived on hand-customized Dockerfiles that drop the trailing slash or add a `./` prefix on `COPY` sources (both valid Docker forms); now parses the COPY source token instead of matching a literal substring.

## [1.0.2] - 2026-09-06

### Fixed

- **`ncgo version`**: fall back to the Go module's resolved version (`debug.BuildInfo().Main.Version`) when `Version` is still the unset placeholder, so `go install github.com/byx-darwin/ncgo@<tag>` reports the real version instead of `0.1.0-dev`.

## [1.0.1] - 2026-09-06

### Changed

- **Release builds**: `goreleaser` now injects `Version`, `BuildVersion` (short commit), and `BuildTime` via `-ldflags` so released binaries report accurate `ncgo version` output instead of the `dev` placeholder.
- **`.goreleaser.yaml`**: migrated to the v2 configuration schema (`version: 2`, `archives.formats`).

## [1.0.0] - 2026-09-06

### Added

- **Rule Center**: `--rule-center-addr` flag on `ncgo new` and `ncgo add rpc` for generating gRPC rule-center client code, SQLC query templates, and configuration stubs for Hertz and Kitex services.
- **`ncgo add rule-center`**: Subcommand to add rule-center support to an existing project.
- **`ncgo add infra canary`**: Optional infra add-on for canary release support (`release_canary`), including unified rule engine, context helpers, Herts/Kitex middleware, and SDK-neutral registry/discovery adapters.
- **`ncgo add infra logging`**: Optional infra add-on for unified observability logging (`observability_logging`), including category-based file routing, oops structured parsing, rotation and compression.
- **Wire mode**: `--wire` flag on `ncgo add infra` commands to auto-insert middleware, interceptors, and imports into server/client wiring code. Supports `--dry-run` and `--plan` for preview.
- **Standalone doc generation**: `ncgo ai sync` now generates standalone `docs/ncgo/` reference files alongside agent context files.
- **Code template export**: `ncgo export templates` command to export Hertz/Kitex code templates with variable substitution.
- **Template engine**: YAML template types, proto parser for ServiceInfo extraction, and render engine with variable substitution.
- **Hertz template apply**: Apply exported templates after `hz new` for post-generation customization of Hertz services.
- **Rate limit E2E test**: Comprehensive end-to-end test command covering all source/backend scenarios with readiness probe support.
- **`--preset` flag**: Template preset override on `ncgo new` for rule-center modules.
- **AI init architecture awareness**: `ncgo ai init claude` now detects project architecture and auto-updates context on `ncgo add`.
- **MCP rule-center support**: MCP tools for rule-center configuration and management.

### Fixed

- **Rate limit**: Critical bugs in rule-center client timeout handling, method wildcard support in SQLC queries, and e2e script reliability.
- **Kitex templates**: Skip rate-limit templates in default Kitex projects (only included when `--preset` rule-center is used).
- **AI error wording**: Corrected error message text in `writeStandaloneDocs`.

### Changed

- **Golden snapshots**: Updated for rule-center integration, rate-limit template changes, and logging/canary optional wiring.
- **Scaffold**: Rule-center config added to Docker configuration and generated project layouts.

### Documentation

- Rate-limit E2E test plans and MCP scaffold examples.
- Rule-center examples and usage guide.
- Code template export and apply implementation plan.
- Design specs for canary release, observability logging, i18n system, Proto I/O system, and v1.0 architecture plan.
- Architecture documentation and `.claude/` reorganization.

### Chore

- Add `.worktrees/` to `.gitignore`.
- Template clarifications for rule-center middleware as pass-through placeholders.

# Contributing

Thanks for contributing to `ncgo`.

中文版本： [CONTRIBUTING.zh-CN.md](CONTRIBUTING.zh-CN.md)

This document is the contributor entry point for local checks, pull requests,
release labels, and release-related docs.

## Development prerequisites / 开发前提

- Go `1.25+`
- `hz >= v0.9.7` when working on Hertz generator flows
- `kitex >= v0.16.1` when working on Kitex generator flows

If you only need to inspect scaffold inputs, prefer `--no-generate` workflows.

## Local checks / 本地检查

Run the same core checks as CI before opening or updating a PR:

```bash
go build ./...
go build .
go vet ./...
go test ./... -count=1
./scripts/smoke.sh
```

Use the smallest useful scope first when iterating, but make sure the final PR
state passes the full checks above.

## Optional pre-commit workflow / 可选 pre-commit 工作流

This repository now includes `.pre-commit-config.yaml` for contributors who use
[`pre-commit`](https://pre-commit.com/).

Recommended setup:

1. install `pre-commit` with your preferred package manager
2. run `pre-commit install --install-hooks --hook-type pre-commit --hook-type pre-push`
3. run `pre-commit run --all-files` once after bootstrapping or after changing hook config

Hook split:

- `pre-commit`: file hygiene checks plus `gofmt` for staged Go files
- `pre-push`: `go vet ./...`, `go test ./... -count=1`, `go build .`, and `./scripts/smoke.sh`

The pre-push hooks intentionally mirror the repository's CI checks, so they may
take longer than the pre-commit stage.

## Making changes / 改动约定

- Keep changes minimal and consistent with the current project structure.
- Update tests when code behavior changes.
- Update docs/examples when commands, install flow, release flow, or generated
  behavior changes.
- If a change is docs-only, call that out clearly in the PR.

## Pull requests / 提交 PR

- Use `.github/PULL_REQUEST_TEMPLATE.md`.
- Describe user-visible impact.
- Record validation steps.
- Call out breaking changes explicitly.
- Choose release labels for the PR.

## Issues / 提交 Issue

- Use the GitHub issue forms for bug reports, feature requests, and docs improvements.
- Include the smallest useful reproduction for bugs.
- Point to the affected README/docs section for documentation issues when possible.

## Release labels / 发布标签

Release notes are generated from PR labels.

Primary reference:

- `docs/release-labels.zh-CN.md`

Common labels:

- `feature` / `enhancement`
- `fix` / `bug`
- `docs`
- `chore`, `ci`, `refactor`, `test`
- `breaking-change` or `semver-major` for compatibility breaks
- `skip-release-notes` for noise that should stay out of release notes

## Release docs / 发布资料

- `docs/release.zh-CN.md` — release workflow and manual release steps
- `docs/release-notes-template.zh-CN.md` — human-edited release notes template
- `.github/release.yml` — GitHub generated release notes categories

## Useful docs / 相关文档

- `README.md`
- `README.zh-CN.md`
- `docs/examples.md`
- `docs/examples.zh-CN.md`
- `docs/context-handoff.zh-CN.md`

If you are unsure where a change belongs, open a draft PR early and explain the
intended user impact plus the validation plan.
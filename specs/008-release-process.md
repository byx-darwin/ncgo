# ncgo CI / Release Engineering

## CI

`.github/workflows/ci.yml` runs on `push` and `pull_request`:

- `gofmt` check;
- `go vet ./...`;
- `go test ./... -count=1`;
- `go build .`;
- `./scripts/smoke.sh`.

Before local commits, run the checks documented in `CONTRIBUTING.md`.

## Release Workflow

`.github/workflows/release.yml` supports two entry points:

- `workflow_dispatch`: builds snapshot artifacts only, does not create a GitHub Release;
- Push `v*.*.*` tag: builds official artifacts, generates `checksums.txt`, and creates a GitHub Release.

Official releases build for:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`

Release binaries inject version info via `-ldflags`:

- `internal/cli.Version`: tag version, or `snapshot-<sha7>` for non-tag builds;
- `internal/cli.BuildVersion`: current commit short SHA;
- `internal/cli.BuildTime`: UTC build time in RFC3339 format.

When running `go install .` locally (without `-ldflags`), `ncgo version` falls back to Go build info's `vcs.revision` / `vcs.time`, appending `-dirty` for uncommitted changes.

GitHub Release uses `gh release create --generate-notes` for auto-generated release notes; categorization is driven by `.github/release.yml`.

For manual polishing before release, see `docs/release-notes-template.md`. For PR label / release note classification conventions, see `docs/release-labels.md`.

After release, users install via: `go install github.com/byx-darwin/ncgo@latest`.

## Manual Release Steps

Deployment, releasing, pushing, and tagging all require human confirmation. Recommended flow:

1. Confirm clean working tree, changes reviewed.
2. Run full local validation: `go build ./...`, `go build .`, `go vet ./...`, `go test ./... -count=1`, `./scripts/smoke.sh`.
3. `git tag -a vX.Y.Z -m "vX.Y.Z: <summary>"`
4. `git push origin main --tags`
5. Verify GitHub Actions release workflow succeeds.
6. Verify `go install github.com/byx-darwin/ncgo@latest` installs the correct version.

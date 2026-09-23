# ncgo CI / Release Engineering

Chinese version: [008-release-process.zh-CN.md](008-release-process.zh-CN.md)

## Verification layers

`.github/workflows/ci.yml` is both the normal CI workflow and the reusable
release gate. It separates deterministic repository checks from tests that
intentionally resolve generated-project dependencies:

- **Hermetic unit gate**: after one explicit `go mod download`,
  `scripts/test-unit.sh` disables the module proxy and automatic toolchain
  download, then checks generated reference drift, formatting, vet, short unit
  tests, all-package build, and the CLI smoke suite.
- **Generated-project integration**: runs on Go `1.26.5` with pinned
  `protoc 28.3`, `hz v0.9.7`, `kitex v0.16.1`, and `sqlc v1.30.0`. It generates,
  tidies, builds, and race-tests Hertz and Kitex projects, including database
  profiles, and validates the pinned Polaris SDK fixture. Reviewed Hertz and
  Kitex dependency lock fixtures are prefetched first; the generated projects
  then tidy, build, and test offline with `-mod=readonly`. The cache key includes
  those lockfiles, compatibility constants, and templates.
- **Race and coverage**: runs every Monday and whenever the reusable workflow
  is called as a release gate. It uses the hermetic/short test surface.

The network-dependent tests require `NCGO_INTEGRATION=1`; otherwise they skip.
When integration mode is enabled, a missing generator is a hard failure rather
than a silent skip.

## Required release gate

`.github/workflows/release.yml` calls CI with `release_gate: true`. No package
build starts until all of these complete successfully:

1. generated-reference and formatting checks;
2. `go vet`, hermetic unit tests, and all-package build;
3. CLI smoke tests;
4. real generated-project and pinned SDK verification;
5. race and coverage checks.

`workflow_dispatch` uses the same gate and produces snapshot workflow
artifacts. A `v*.*.*` tag additionally publishes a GitHub Release.

## Reproducible artifacts

The release workflow uses `scripts/build-release.sh` and
`scripts/package-release.py` as the single publishing implementation. The old,
unused GoReleaser configuration was removed to prevent two drifting release
paths.

Each target is built and packaged twice, and the archives must compare byte for
byte before upload. Builds use `-trimpath`; `BuildTime` and archive timestamps
come from the source commit (`SOURCE_DATE_EPOCH`); archive order, ownership, and
gzip/zip metadata are normalized.

Published targets are:

- `linux/amd64` and `linux/arm64` (`tar.gz`)
- `darwin/amd64` and `darwin/arm64` (`tar.gz`)
- `windows/amd64` (`zip`)

The metadata job requires exactly those five archives, creates a sorted
`checksums.txt`, immediately verifies it, and publishes `provenance.json` with
the repository, full revision, ref, workflow run, and artifact digests.
GitHub's `actions/attest@v4` also creates signed artifact attestations before
the release job may run. Every Action in the required release path is pinned to
a reviewed full commit SHA, and release jobs use the explicit `ubuntu-24.04`
runner label.

## Verify a downloaded artifact

Download the archive, `checksums.txt`, and `provenance.json` from the same
release. On Linux:

```bash
asset=ncgo_vX.Y.Z_linux_amd64.tar.gz
test "$(awk -v a="$asset" '$2 == a {n++} END {print n+0}' checksums.txt)" -eq 1 && \
  sha256sum -c <(awk -v a="$asset" '$2 == a' checksums.txt) && \
  gh attestation verify "$asset" -R byx-darwin/ncgo
```

On macOS, verify the selected digest directly:

```bash
asset=ncgo_vX.Y.Z_darwin_arm64.tar.gz
test "$(awk -v a="$asset" '$2 == a {n++} END {print n+0}' checksums.txt)" -eq 1 && \
  shasum -a 256 -c <(awk -v a="$asset" '$2 == a' checksums.txt) && \
  gh attestation verify "$asset" -R byx-darwin/ncgo
```

Confirm that `provenance.json.source.revision` is the intended tag commit and
that its artifact digest matches `checksums.txt`.

## Manual release steps

Publishing, tagging, and pushing require human confirmation.

1. Confirm the working tree is clean and the intended commit is on `main`.
2. Run `go mod download && ./scripts/test-unit.sh`. When changing generators or
   templates, also install the exact versions above and run
   `./scripts/test-generated.sh`.
3. Review [008-release-notes-template.md](008-release-notes-template.md) and the
   [release-label conventions](008-release-labels.md).
4. Create an annotated, never-reused tag: `git tag -a vX.Y.Z -m "vX.Y.Z"`.
5. Push that tag after confirmation: `git push origin vX.Y.Z`.
6. Verify the release gate, five assets, checksums, provenance manifest, and
   attestations before announcing the version.
7. Confirm `go install github.com/byx-darwin/ncgo@vX.Y.Z` and `ncgo version`.

## Recovery

- **Gate or build failure:** nothing is published. Fix the cause on a new
  commit and create a new patch tag. Do not move or reuse the failed tag; Go
  module proxies may already have observed it.
- **Transient rerun before publication:** a run proves within-run reproducibility
  by building every target twice. Before reusing an incomplete immutable
  tag/SHA run, compare its new digests with the earlier run. Hosted runner or
  compression-runtime maintenance can still change bytes across runs; if any
  digest changes, do not replace assets—review the cause and publish a new patch.
- **Incomplete GitHub Release:** stop announcements, record the workflow URL,
  remove only the incomplete GitHub Release after human confirmation, and
  rerun the same immutable tag. Never overwrite individual assets silently.
- **Bad version already published:** mark the release as affected, publish a
  fixed patch version, and communicate upgrade/remediation steps. Deleting a
  Release or tag does not recall a version cached by Go proxies, so never
  retarget an existing version.
- **Checksum or attestation mismatch:** treat the artifact as untrusted, do not
  install it, retain the evidence/run URL, and publish a new patch only after
  identifying the cause.

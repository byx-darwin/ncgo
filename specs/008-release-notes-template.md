# ncgo Release Notes Template

For polishing GitHub auto-generated release notes before and after tag publishing.

Recommended structure:

## One-Line Summary

- The most important change in this release
- Keep it to 1-3 sentences

## Highlights

- New capabilities:
- Important fixes:
- Documentation / release improvements:

## Installation / Upgrade

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo version
```

If version needs emphasis:

- Upgrade to `vX.Y.Z`
- Rerun `ncgo version` to confirm

## Compatibility Notes

- Whether there are breaking changes
- Whether generator manual upgrades are needed (e.g. `hz` / `kitex`)
- Whether only newly generated projects are affected, not existing ones

## Verification

- `go test ./... -count=1`
- `./scripts/smoke.sh`
- `go install .`

## Auto-generated Change List

Keep the GitHub auto-generated categorized change list (from `.github/release.yml`) at the bottom as a detailed diff.

# Run Validation

Use this workflow after editing code or user-facing docs.

## Order

1. focused unit tests or `go test -run` for the exact target
2. relevant file/package tests
3. `go test -race` when concurrency, shutdown, or timing behavior is involved
4. targeted integration tests
5. small CLI or end-to-end smoke checks
6. broader repository checks only when needed

## Notes

- prefer the cheapest reliable signal first
- if a check fails, make the smallest plausible fix and rerun the closest check
- for docs edits, include markdown diagnostics in the report
- do not jump to full-repository validation before the nearest failing check passes
- for template or generated-output changes, prefer targeted golden, fixture, or regeneration checks before broad test runs

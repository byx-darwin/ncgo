# Run Validation

Use this workflow after editing code or user-facing docs.

## Order

1. focused unit tests
2. relevant file/package tests
3. targeted integration tests
4. small CLI or end-to-end smoke checks
5. broader repository checks only when needed

## Notes

- prefer the cheapest reliable signal first
- if a check fails, make the smallest plausible fix and rerun the closest check
- for docs edits, include markdown diagnostics in the report

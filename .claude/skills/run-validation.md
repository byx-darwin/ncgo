# Run Validation

Use this workflow after editing code or user-facing docs.

## Validation Order

Run from smallest and fastest to broadest:

1. `go test ./internal/<pkg>/... -run TestTargetFunction -count=1`
2. `go test ./internal/<pkg>/... -count=1`
3. `go test ./... -count=1`
4. `go build ./...` and `go vet ./...`
5. `./scripts/smoke.sh`

## Surface-Specific Minimum Checks

| Surface changed | Minimum check |
| --- | --- |
| MCP tool schema or output shape | `go test ./internal/mcp/... -count=1` |
| CLI command, flag, or JSON output | targeted command test + `./scripts/smoke.sh` |
| scaffold template or golden fixture | `go test ./internal/scaffold/... -count=1` |
| doctor or protolint logic | `go test ./internal/doctor/... -count=1` or `./internal/protolint/...` |
| `ai sync` or context generation | `go test ./internal/ai/... -count=1` |
| shared helper or builder | package-level tests + direct callers |
| docs only | markdown diagnostics; no Go checks required |

## Notes

- prefer the cheapest reliable signal first
- if a check fails, make the smallest plausible fix and rerun the closest failing check
- do not skip or suppress a failing test to proceed; stop and ask instead
- for docs edits, run markdown diagnostics and include the result in the report

# Write Tests

Use this workflow whenever behavior changes. Tests are part of the change.

## Choose the Smallest Useful Level

| Change type | Minimum test |
| --- | --- |
| Pure logic, helper, formatter, mapper | unit test |
| Handler, transport, CLI command, endpoint wiring | integration test |
| Template, codegen input, generated output | golden/snapshot test if the repo uses one |
| Entrypoint, top-level workflow, smoke path | smoke check |

## Mono vs Micro Scope

- In a mono service repo, start with the affected package or service entrypoint.
- In a micro workspace, test the narrowest affected service first.
- Expand to workspace-level checks only when shared root behavior or multiple services are affected.

## Checklist

1. cover the happy path
2. cover at least one important error or conflict path
3. verify structured output or stable contract fields when they matter
4. avoid broad end-to-end tests when a smaller test gives the same signal
5. if generated output changes intentionally, update snapshots or fixtures together
6. prefer table-driven tests with clearly named cases when multiple scenarios exist
7. test observable behavior, not private implementation details

## Validation Order

Run from smallest and fastest to broadest:

1. focused test function
2. relevant test file
3. affected package or service tests
4. related integration tests
5. smoke checks

## Notes

- keep tests near the code they validate
- preserve existing contract wording when assertions depend on it
- do not hand-edit generated output instead of fixing the source input or template
- if the repository already has a test helper or fixture pattern, reuse it
- for flaky, concurrent, or shutdown-related behavior, consider `go test -race` before widening scope
- if the repository uses golden or snapshot tests, review output diffs before accepting fixture updates
- if the repository uses `testify`, prefer `require` for setup/preconditions and `assert` for behavior checks

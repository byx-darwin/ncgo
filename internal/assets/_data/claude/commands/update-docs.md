# Update docs

Use the `doc-writer` agent together with the `doc-sync` skill.

For user-facing behavior changes:

1. update the nearest English source-of-truth doc first
2. sync the Chinese pair where one exists
3. update worked examples together with command/output changes
4. if stable machine-consumed fields or API contracts changed, update the nearest contract doc and any Swagger/OpenAPI docs the repository owns
5. run markdown diagnostics if available

Confirm which docs were updated and include the diagnostics command and result in the summary.

# Documentation Sync

Use this workflow when a change affects commands, outputs, flags, or stable behavior.

## Rules

- update the closest source-of-truth doc first
- keep English and Chinese docs aligned for the same user-facing behavior
- update examples when the contract is likely to be copied by users or agents
- run markdown diagnostics after doc edits
- when commands, flags, outputs, or stable machine-consumed fields change, treat docs as part of the same delivery
- update worked examples together with the command or output they demonstrate; do not leave stale examples behind
- if the repository owns Swagger or OpenAPI docs, update those annotations or generated docs when the API contract changes
- do not document behavior that was not actually implemented

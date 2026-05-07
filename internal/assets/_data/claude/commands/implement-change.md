# Implement a planned change

Use the `implementer` agent for this workflow.

Apply the `write-tests` skill when behavior changes.

If there is no approved plan and the task is non-trivial, run `/plan` first.

When implementing:

1. read the target files and nearby tests first
2. make the smallest correct edit
3. update tests when behavior changes
4. run focused validation using the smallest useful checks first
5. if user-facing behavior changed, either update docs or request a handoff to `doc-writer`

Prepare a reviewer handoff with:

- one-line summary of what changed
- contract-sensitive surfaces touched
- tests or checks that were run and their outcome

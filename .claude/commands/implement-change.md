# Implement a planned change

Use the `implementer` agent for this workflow.

Apply the [write-tests](../skills/write-tests.md) skill when behavior changes.
Apply the [run-validation](../skills/run-validation.md) skill when deciding which checks to run.

If there is no approved plan and the task is non-trivial, run `/plan` first.

When implementing:

1. read the target files and nearby tests first
2. use `.claude/generated/project-context.md` as a fact sheet when available
3. preserve layering, context propagation, and generated-input ownership unless the task explicitly changes them
4. make the smallest correct edit
5. update tests when behavior changes
6. run focused validation using the smallest useful checks first
7. if user-facing behavior changed, either update docs or request a handoff to `doc-writer`

Prepare a reviewer handoff with:

- one-line summary of what changed
- contract-sensitive surfaces touched
- tests or checks that were run and their outcome
- whether docs were updated directly or still need a `doc-writer` follow-up

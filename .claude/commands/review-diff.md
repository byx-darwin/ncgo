# Review diff

Use the `reviewer` agent after `implementer` has completed focused validation.

## Checklist

- summarize what changed
- identify contract-sensitive files touched
- confirm tests/checks that were run
- note whether docs were updated
- check for layering drift, context mistakes, or generated-output ownership mistakes
- call out obvious data-access, observability, or sensitive-data handling risks when present in the diff
- call out any remaining risk or follow-up items

End with exactly one review outcome:

- `PASS`: safe to hand off or merge
- `NEEDS REVISION`: specific gap plus the smallest fix required

If user-facing behavior changed but docs were not updated, explicitly request a handoff to `doc-writer`.

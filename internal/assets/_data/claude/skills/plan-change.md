# Plan Change

Use this workflow before making a non-trivial change.

## Goals

- identify the smallest safe edit target
- confirm the relevant tests or validation path
- avoid speculative edits across unrelated packages

## Checklist

1. find the command, function, or contract that actually owns the behavior
2. read nearby tests before editing
3. state the planned files to touch
4. make the smallest diff that solves the task
5. run the smallest useful validation before broad checks

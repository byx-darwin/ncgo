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

## Affected Surfaces

When the task touches one of these surfaces, include the linked follow-up work in
the plan:

- CLI commands, flags, or output -> update tests, docs, and worked examples
- API or transport contract -> update handler/schema/IDL-adjacent checks plus tests and docs
- repository or data-mapping logic -> review error mapping, resource cleanup, and query behavior
- templates, manifests, or codegen inputs -> plan golden, fixture, or regeneration validation
- generated output or stable machine-consumed fields -> treat as contract-sensitive and call that out explicitly

## Scope Discipline

- include every contract-sensitive surface that is actually affected
- do not mix unrelated cleanup into the planned diff
- if extra refactoring looks useful but is not required, list it separately as follow-up rather than silently expanding the task

# Agent Engineering Rules

This file defines how an AI coding agent should work in this repository.

## Goals

- make small, correct, explainable changes
- prefer fast, reliable validation over guesswork
- keep repository behavior stable unless the task explicitly changes it
- avoid risky actions unless the user explicitly asked for them

## Core Rules

- MUST read relevant implementation and nearby tests before editing
- MUST keep diffs minimal and avoid unrelated cleanup
- MUST validate changes with the smallest useful checks first
- MUST update tests when behavior changes
- MUST explain what changed, what was run, and any remaining risk
- MUST NOT install dependencies, deploy, or run destructive operations without explicit permission

## Information Gathering

- confirm the target symbol, file, or contract exists before editing
- read the narrowest set of files that can justify the change safely
- prefer existing helpers, patterns, and tests over inventing new structures
- if behavior is still ambiguous after reasonable inspection, ask instead of guessing

## Change Strategy

- prefer the smallest possible edit that fully solves the requested task
- preserve existing public behavior unless the task explicitly changes it
- do not mix refactors with behavior changes unless necessary
- keep generated files, templates, schemas, CLI output, and agent contracts stable

## Validation Order

Run validation from smallest and fastest to broadest and slowest:

1. focused unit tests
2. relevant test file
3. relevant package tests
4. related integration tests
5. small smoke checks
6. broader repository checks only when needed

## Documentation Rules

- when commands, generated outputs, or stable contracts change, update docs
- keep English and Chinese docs aligned when they describe the same behavior
- after documentation edits, run markdown diagnostics

## Communication

- state the plan before major edits or validation runs
- summarize progress as the task advances
- on completion, report what changed, what was run, and any remaining risk

## Stop Conditions

- stop when the requested task is complete and relevant validation has passed
- do not continue into adjacent improvements without asking
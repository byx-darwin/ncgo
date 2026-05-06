# Go Service Rules

This file defines Go-specific coding rules for a repository bootstrapped with
`ncgo`.

## Goals

- keep Go changes small, readable, and consistent with the current codebase
- preserve stable CLI, API, template, and generated-output contracts unless the task explicitly changes them
- prefer existing repository patterns over introducing new abstractions

## General Style

- follow standard Go style and keep files `gofmt`-clean
- prefer small, focused functions over large rewrites
- prefer explicit, readable code over clever abstractions
- reuse existing helpers before adding new utility layers

## Typical Service Boundaries

- keep handlers or transport shells thin
- keep business logic in usecase/service-layer code
- avoid pulling transport-specific concerns deep into core business logic
- keep repository and adapter code isolated from request/response transport details

Respect the generated project layout and the current repository structure rather
than forcing a new architecture during routine tasks.

## Contract-Sensitive Surfaces

Be especially conservative when editing:

- CLI flags, command text, and JSON output
- protobuf/IDL files and generated transport bindings
- generated configuration, templates, and codegen inputs
- response/error shapes that callers may already depend on

When these surfaces change, update tests and docs together.

## Errors and Control Flow

- return clear errors; do not swallow errors silently
- wrap errors with useful context when crossing package or filesystem boundaries
- preserve existing error wording when tests or contracts depend on it
- prefer early returns to deeply nested control flow

## Tests and Generated Files

- put tests close to the code they validate
- add helper-level tests for pure logic and formatting behavior
- use integration tests for CLI, transport, or multi-package wiring behavior
- do not hand-edit generated outputs as a substitute for fixing templates or source inputs

## Documentation Coupling

- when Go changes affect user-facing behavior, update the relevant docs
- keep English and Chinese variants aligned for the same behavior
- if code introduces a new stable output field, flag, or workflow step, document it near the existing contract docs
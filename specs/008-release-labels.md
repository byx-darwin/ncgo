# ncgo Release Labels Convention

This document defines label conventions for GitHub Release auto-generated notes. Classification rules are driven by `.github/release.yml`; this explains how teams should label PRs.

## Goals

- Make GitHub auto-generated release notes more stable
- Ensure correct categorization for similar changes
- Reduce manual curation effort per release

## Core Rules

1. Every PR that appears in release notes must have at least one release label.
2. If multiple categories apply, prefer the one with "greatest user impact".
3. Pure internal noise changes may use `skip-release-notes` to exclude.
4. Breaking changes must additionally carry `breaking-change` or `semver-major`.

## Label to Category Mapping

| Label | Release Category | When to Use |
|---|---|---|
| `breaking-change` | Breaking Changes | Compatibility breaks, command behavior changes |
| `semver-major` | Breaking Changes | Changes requiring a major version bump |
| `feature` | Features | New user-facing capabilities |
| `enhancement` | Features | Notable enhancements to existing capabilities |
| `fix` | Fixes | Fixes for incorrect behavior, regressions, or wrong output |
| `bug` | Fixes | Like `fix`, for clear bug fixes |
| `docs` | Documentation | Docs, README, examples, release notes improvements |
| `chore` | Internal | Maintenance work not worth highlighting |
| `ci` | Internal | CI / workflow / automation changes |
| `refactor` | Internal | Refactoring, structural changes, no observable behavior change |
| `test` | Internal | Test additions, test refactoring |
| `skip-release-notes` | Excluded | Changes that should not appear in release notes |

## Common Judgment Examples

### Suitable for `feature`

- New `ncgo add ...` subcommands
- New optional infra capabilities
- New MCP-exposed capabilities

### Suitable for `enhancement`

- Adding `--plan` / `--dry-run` enhancement to existing commands
- README homepage, example doc, usability improvements

### Suitable for `fix` / `bug`

- Fix regression in command output
- Fix incorrect golden test data
- Fix MCP response structure mismatch with schema

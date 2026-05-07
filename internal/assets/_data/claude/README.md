# .claude Layout

This repository uses `.claude/` for Claude-facing repository rules,
generated project facts, and private local notes.

## Recommended Layout

```text
.claude/
  README.md
  rules/
  skills/
  hooks/
  agents/
  commands/
  generated/
  local/
```

## Ownership

### Hand-authored, repo-owned

These files define shared repository policy and workflow.
They should be reviewed and edited like normal source files.

- `.claude/README.md`
- `.claude/rules/*`
- `.claude/skills/*`
- `.claude/hooks/*`
- `.claude/agents/*`
- `.claude/commands/*`

### Generated, tool-owned

These files are safe to refresh with `ncgo ai sync`.
They should contain project facts rather than repository policy.

- `.claude/generated/*`

### Local, user-owned

These files are private overlays for personal notes or machine-specific prompts.
They should normally be gitignored.

- `.claude/local/*`

## Recommended Workflow

1. Run `ncgo ai init claude --root .` once to bootstrap the minimal starter files.
2. Optionally use `ncgo ai init claude --root . --preset team` for workflow starter docs and subagents such as `planner`, `implementer`, `reviewer`, `debugger`, and `doc-writer`.
3. Edit `.claude/rules/*` to match your repository policy.
4. Run `ncgo ai sync --root . --lang en` to refresh generated project facts, especially `.claude/generated/project-context.md`.

## Using External Go Skills

Starter files under `.claude/` should stay short and repository-specific.
Use external Go skills for deeper, on-demand guidance rather than copying full
skill docs into this repository.

Typical examples:

- use a testing skill when designing table-driven tests, race checks, or golden updates
- use a troubleshooting skill when a failure is flaky, timing-sensitive, or hard to reproduce
- use a database or security skill when a diff touches repository, data-access, or user-boundary code

Treat `.claude/rules/*`, `.claude/skills/*`, and `.claude/agents/*` as the
repository policy layer, and external skills as the deep reference layer.

## Mono vs Micro Repositories

Starter files under `.claude/` are intentionally project-generic.

{{PROJECT_SHAPE_GUIDANCE}}

## Agent Files

Files under `.claude/agents/*.md` are hand-authored Claude Code custom
subagents.

To be dispatchable, each agent file should begin with YAML frontmatter that
defines:

- `name`
- `description`
- `tools`

Write `description` in task language such as `Use when ...` so Claude Code can
match the agent to a request.

## Precedence

When instructions overlap, use this precedence order:

1. `.claude/rules/*`
2. `.claude/skills/*`, `.claude/hooks/*`, `.claude/agents/*`, `.claude/commands/*`
3. `.claude/generated/*`
4. `.claude/local/*`

Generated files provide facts and summaries. They must not redefine shared
repository policy.

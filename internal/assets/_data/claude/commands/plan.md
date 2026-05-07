# Plan before editing

Use the `planner` agent together with the `plan-change` skill.

Before planning, read `.claude/generated/project-context.md` when present so the
plan starts from repository facts rather than assumptions.

Output the plan before any edits using these sections:

- `Task`
- `Files to change`
- `Contract-sensitive surfaces touched`
- `Tests to write or update`
- `Risk notes`

If templates, generated outputs, docs, or machine-consumed fields may be
affected, call that out explicitly in the plan instead of leaving it implicit.

Do not begin implementation until the plan has been stated and is ready to hand off to `implementer`.

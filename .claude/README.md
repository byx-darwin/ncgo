# .claude Layout and Ownership

This directory defines the recommended Claude-facing repository structure for `ncgo`.
It separates generated project facts from hand-authored policy, workflows, and local overrides.

## 1. Design Goals

- Keep `ncgo ai sync` deterministic and conservative.
- Generate only content that can be derived safely from repository state.
- Keep rules, workflows, hooks, and agent roles hand-authored unless a future command explicitly scaffolds them.
- Preserve user-owned local notes without overwrite risk.

## 2. Recommended Layout

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

## 3. Ownership Rules

### Hand-authored, repo-owned

These files define policy and workflow. `ncgo ai sync` SHOULD NOT overwrite them.

- `.claude/rules/*`
- `.claude/skills/*`
- `.claude/hooks/*`
- `.claude/agents/*`
- `.claude/commands/*`
- `.claude/README.md`

These files should be reviewed like normal source files.

### Generated, tool-owned

These files may be written by `ncgo ai sync` and should carry the managed marker.

- `.claude/generated/project-context.md`
- `.claude/generated/manifest-summary.md`
- `.claude/generated/architecture.md` (future, once AST-derived project scanning exists)

Generated files should contain facts, summaries, and derived project context, not policy.

### Local, user-owned

These files are private overlays and MUST NOT be overwritten by `ncgo ai sync`.

- `.claude/local/*`

They are good places for personal notes, local prompts, and machine-specific preferences.
They should normally be gitignored.

## 4. What `ncgo ai sync` SHOULD Generate

`ncgo ai sync` is best suited to deterministic project facts, for example:

- manifest summary
- service kind / module / infra / domains
- embedded design-doc summary
- future scanned architecture facts
- links to stable repo rules

Safe starting point for future `.claude` support:

- `.claude/generated/project-context.md`

This keeps generation low-risk and avoids conflicts with hand-authored workflow files.

## 5. What `ncgo ai sync` SHOULD NOT Generate

`ncgo ai sync` SHOULD NOT directly generate or overwrite:

- `.claude/rules/go.md`
- `.claude/rules/agent-engineering.md`
- `.claude/skills/*`
- `.claude/hooks/*`
- `.claude/agents/*`
- `.claude/commands/*`

Reason: these files express policy, workflow, role boundaries, or tool-specific behavior.
They are not purely derivable from the manifest and embedded design docs.

If the repository wants bootstrapped starter files later, that should be a separate explicit scaffold command, not `ai sync`.

## 6. Precedence Model

Use this precedence model when generated and hand-authored content overlap:

1. repo policy and safety rules in `.claude/rules/*`
2. repo workflows in `.claude/skills/*`, `.claude/hooks/*`, `.claude/agents/*`, `.claude/commands/*`
3. generated project facts in `.claude/generated/*`
4. local personal notes in `.claude/local/*`

Generated files provide facts and context. They MUST NOT redefine repository policy.
Local files may add personal guidance, but they SHOULD NOT weaken shared repository safety rules.

## 7. Relationship to Current `ai sync`

Today `ncgo ai sync` writes:

- `AGENTS.md`
- `CLAUDE.md`
- `.cursor/rules/ncgo.mdc`

Those outputs remain valid. A future `.claude` integration should start by adding `.claude/generated/*`, not by replacing hand-authored rules or workflow files.

For the first-step implementation plan, see:

- `docs/ai-sync-claude-generated-plan.md`
- `docs/ai-init-claude-plan.md`

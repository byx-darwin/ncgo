# Agent Quickstart

This guide connects Codex, Claude Code, or Cursor to ncgo and gives coding
agents a predictable workflow for inspecting and changing generated projects.

Chinese version: [agent-quickstart.zh-CN.md](agent-quickstart.zh-CN.md).

## 1. Know Which Repository You Opened

- The ncgo source repository contains `main.go`, `internal/cli/`, and
  `internal/scaffold/`. Follow its root `AGENTS.md`.
- A mono service contains `.ncgo/manifest.yaml`.
- A micro workspace contains `ncgo.workspace`; member services have their own
  `.ncgo/manifest.yaml` files.

Do not infer project type from directory names. Read the metadata file.

## 2. Install and Verify ncgo

```bash
go install github.com/byx-darwin/ncgo@latest
command -v ncgo
ncgo version
```

Use the absolute path returned by `command -v ncgo` in MCP configuration when
the desktop client may not inherit your shell `PATH`.

In a generated project, render every supported Agent target once:

```bash
ncgo ai sync --root . --target all
ncgo check --root . --output json
```

This writes the universal `AGENTS.md` plus Claude and Cursor context files.

## 3. Connect an MCP Client

`ncgo mcp serve` is a local stdio server. It inherits the client's current
working directory, and MCP filesystem operations are restricted to that
workspace. Start the client from the intended repository root.

All MCP `root`, `dir`, `templateDir`, `to`, and explicit file inputs must be
relative. `..`, absolute paths, symlinks that resolve outside the workspace,
and dangling symlinks are rejected before a tool reads or writes files. A
symlink is allowed when its resolved target remains inside the workspace;
targets that do not exist yet are checked through their nearest existing
parent. Registry templates selected by name come from ncgo's managed cache.

The server also inherits the client's environment. Read-only tools need only
the `ncgo` binary, while generation workflows may invoke `go`, `hz`, `kitex`,
`protoc`, or `sqlc`. Make those tools available on the inherited `PATH`, or use
the client's MCP environment setting when required. Do not place secrets in a
project-shared MCP configuration.

### Codex

The installed Codex CLI supports stdio MCP servers through `codex mcp add`:

```bash
codex mcp add ncgo -- /absolute/path/to/ncgo mcp serve
codex mcp get ncgo
```

Remove the entry with `codex mcp remove ncgo`.

### Claude Code

```bash
claude mcp add ncgo -- /absolute/path/to/ncgo mcp serve
claude mcp get ncgo
```

For a project-shared configuration, Claude Code also accepts `.mcp.json`:

```json
{
  "mcpServers": {
    "ncgo": {
      "command": "/absolute/path/to/ncgo",
      "args": ["mcp", "serve"]
    }
  }
}
```

Review and approve project-scoped MCP servers before using them.

### Cursor

Create `.cursor/mcp.json` in the generated project:

```json
{
  "mcpServers": {
    "ncgo": {
      "command": "/absolute/path/to/ncgo",
      "args": ["mcp", "serve"]
    }
  }
}
```

Restart or refresh Cursor's MCP server list after changing the file.

## 4. Verify the Connection

Ask the Agent to list its ncgo tools and call `ncgo_version`. For a raw stdio
check without an MCP client:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | ncgo mcp serve
```

The second response should contain tools such as `ncgo_doctor`, `ncgo_check`,
and `ncgo_ai_context`.

## 5. Recommended Agent Workflow

Start with facts, not guesses:

1. Read `AGENTS.md` and `.ncgo/manifest.yaml` or `ncgo.workspace`.
2. Call `ncgo_version`.
3. Run `ncgo doctor --root . --output json` or call `ncgo_doctor`.
4. Run `ncgo check --root . --output json` or call `ncgo_check`.
5. Call `ncgo_ai_context` to inspect domains, methods, anchors, and issues.
6. Select the narrowest command for the requested change.
7. Use `--plan` or `dryRun` before a write when supported.
8. Execute the change and consume returned `nextSteps` instead of inventing
   follow-up commands.
9. Run focused build/tests and `ncgo check`.
10. Refresh context with `ncgo ai sync --root . --target all`, then check again.

Prefer structured top-level MCP fields for decisions. Treat
`content[0].text` as the human-readable presentation.

## 6. Safety and Recovery

- Review paths and side effects before approving a mutating tool.
- Do not use `--force` as the first recovery step; inspect the existing file.
- Do not edit generated handler, router, or `kitex_gen` files when the IDL or
  template is the source of truth.
- If `doctor`, `check`, or `protolint` reports a blocking error, resolve it
  before continuing with unrelated mutations.
- If a command fails after invoking `hz`, `kitex`, `go mod tidy`, or a registry
  operation, inspect the working tree before retrying.

## 7. Reusable Agent Prompt

```text
You are operating the ncgo project at <root>. First identify whether it is the
ncgo source repository, a mono service, or a micro workspace. Read AGENTS.md
and the manifest/workspace metadata. Establish facts with ncgo_version,
ncgo_doctor, ncgo_check, and ncgo_ai_context. Before writing, use plan/dry-run
when available and summarize affected paths and side effects. Prefer structured
fields and returned nextSteps. After the change, run focused tests, ncgo check,
and ncgo ai sync --target all. Do not guess around a failed tool call.
```

For individual command contracts and payload examples, continue with
[examples.md](examples.md#0-mcp-contract-first-reference).

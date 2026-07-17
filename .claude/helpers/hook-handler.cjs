#!/usr/bin/env node
/**
 * Minimal hook handler stub for claude-flow hooks.
 *
 * This is a placeholder handler that prevents SessionStart and other hook
 * errors caused by the handler file being missing. It accepts all hook
 * command types and exits successfully.
 *
 * To replace with a real implementation, see the claude-flow installer.
 */

const VALID_COMMANDS = new Set([
  'session-restore',
  'session-end',
  'route',
  'pre-bash',
  'post-bash',
  'pre-edit',
  'post-edit',
  'status',
  'post-task',
  'notify',
  'compact-manual',
  'compact-auto',
]);

const cmd = process.argv[2];

if (!cmd) {
  console.error('Usage: hook-handler.cjs <command>');
  process.exit(0);
}

if (!VALID_COMMANDS.has(cmd)) {
  console.error(`Unknown hook command: ${cmd}`);
  process.exit(0);
}

// All commands pass through silently for now.
// Hook input is available as JSON on stdin (per Claude Code hook spec),
// but this stub intentionally ignores it.
process.exit(0);

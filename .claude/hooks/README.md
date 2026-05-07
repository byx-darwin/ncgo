# Hooks

This directory is reserved for optional Claude hook configuration.

**Current status: no active hooks.** The `team` preset intentionally ships with hooks disabled.

## Guidance

- keep hooks opt-in and conservative
- avoid hooks that mutate repository state unexpectedly
- document any environment assumptions clearly

## How to enable a hook

Add a hook configuration file here and reference it in your Claude settings.
Document the trigger, the command it runs, and any environment prerequisites.

# Hooks

This directory is reserved for optional Claude hook configuration.

## Guidance

- keep hooks opt-in and conservative
- avoid hooks that mutate repository state unexpectedly
- document any environment assumptions clearly

The default `team` preset intentionally documents hooks without enabling active
hook behavior.

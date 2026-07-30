---
sidebar_position: 5
title: FAQ
---

# FAQ

### Do I need `hz` / `kitex` installed?

For generation flows, yes — `hz >= v0.9.7` for Hertz, `kitex >= v0.16.1` for
Kitex. Use `--no-generate` to skip the generator and only write the manifest,
template inputs, and IDL placeholder. Run `ncgo doctor` to check your toolchain.

### Is ncgo a framework?

No. `ncgo` is a scaffold and lifecycle CLI. It orchestrates the `hz` / `kitex`
generators and keeps manifests, templates, and AI context files under version
control.

### Can I use it on an existing project?

Yes. `ncgo import` generates a `.ncgo/manifest.yaml` for an existing Hertz or
Kitex project so subsequent `ncgo` commands work.

### Does it work with AI coding agents?

That is the point. `ncgo` renders `AGENTS.md`, `CLAUDE.md`, Cursor rules, and
exposes an MCP stdio server so agents can operate on generated projects.

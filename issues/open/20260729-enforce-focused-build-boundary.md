# Enforce the Claudex product boundary at build and test time

Status: open
Model: GPT-5
Created: 2026-07-29
Updated: 2026-07-29
Branch: fix/20260729-focused-build-boundary

## Summary

Make the repository's supported Claudex surface mechanically enforceable instead of relying on documentation while the full upstream proxy implementation remains buildable and test-visible.

## Problem

README describes a loopback-only, Codex-only Anthropic Messages gateway with management UI, plugins, and other providers disabled. The repository still contains broad upstream packages for management, plugins, Amp, Redis, WebSocket relay, remote model updates, and other providers. Configuration validation blocks unsupported features at runtime, but future upstream changes can accidentally reconnect those packages to the Claudex binary or expose routes without a failing build or test.

## Agent Prompt

Implement a narrow build boundary for `cmd/claudex` without deleting upstream code needed for synchronization.

1. Trace the complete dependency graph and route registration path reachable from `cmd/claudex`.
2. Introduce a dedicated Claudex composition root that imports only required authentication, Anthropic translation, Codex execution, usage accounting, and configuration packages.
3. Ensure generic server, management, plugin, Amp, non-Codex provider, remote-management, and public WebSocket relay modules cannot be registered by the Claudex binary.
4. Add architecture tests that fail when forbidden packages or routes become reachable.
5. Keep upstream packages intact and independently compilable where practical.
6. Document the enforced boundary in `AGENTS.md`.

Do not broaden the supported product surface. Do not solve this only with runtime configuration checks.

## Acceptance Criteria

- [ ] `cmd/claudex` has an explicit, reviewable composition root.
- [ ] Forbidden routes and modules cannot be registered in the Claudex process.
- [ ] Tests enumerate the allowed HTTP routes and reject all generic proxy routes.
- [ ] A dependency/architecture test prevents accidental imports of management, plugin, Amp, and non-Codex provider composition code.
- [ ] Upstream synchronization remains possible without deleting the retained upstream packages.
- [ ] `go test ./...` and the focused Claudex build pass.

## Test Plan

- Run `go test ./internal/claudex ./cmd/claudex`.
- Run `go test ./...`.
- Build `./cmd/claudex` and inspect registered routes in an integration test.
- Add a negative fixture proving a forbidden import or route causes a test failure.

## Risks

- Over-isolation may duplicate upstream wiring and make synchronization harder.
- Package-level import checks alone are insufficient if forbidden behavior is reachable through shared generic packages.

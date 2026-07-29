# Harden ClaudexDesktop session ownership and crash recovery

Status: open
Model: GPT-5
Created: 2026-07-29
Updated: 2026-07-29
Branch: fix/20260729-desktop-session-recovery

## Summary

Make temporary Claude Desktop preference changes transactional, single-owner, and recoverable after crashes, forced termination, concurrent launches, and stale gateway processes.

## Problem

ClaudexDesktop temporarily changes Claude Desktop's Third-Party Inference Gateway preferences and restores them when the session ends. A backup-based flow can corrupt or overwrite user preferences when two launchers run concurrently, the process dies between write steps, the backup is stale, or Claude Desktop changes the same settings during the session.

## Agent Prompt

Implement a transaction-oriented session manager for macOS and Linux launchers.

1. Define an explicit state machine covering idle, preparing, active, restoring, recovered, and failed states.
2. Use an atomic lock containing owner PID, start time, executable identity, and transaction ID.
3. Snapshot only the keys Claudex changes, with a checksum and original file metadata.
4. Write preferences and transaction state atomically using temporary files and rename/fsync where supported.
5. On startup, distinguish a live owner from a stale transaction and recover only changes owned by Claudex.
6. Detect user or external modifications and avoid blindly restoring over them; report a precise recovery action.
7. Verify gateway ownership before reusing or terminating a process.
8. Add fault-injection tests for every write boundary and concurrent-launch tests.

Do not modify the Claude Desktop application bundle.

## Acceptance Criteria

- [ ] Only one ClaudexDesktop session can own preferences at a time.
- [ ] Killing the launcher at any persistence step leaves a recoverable state.
- [ ] Recovery does not overwrite unrelated user preference changes.
- [ ] Stale lock detection does not treat PID reuse as the original owner.
- [ ] The launcher reuses only a verified Claudex gateway bound to the expected loopback address and configuration.
- [ ] Errors explain the transaction state and safe manual recovery path without printing secrets.
- [ ] macOS and Linux behavior is covered by platform-specific tests.

## Test Plan

- Add table-driven state-machine tests.
- Inject failures before and after each atomic write/rename.
- Launch two instances concurrently and assert one cleanly refuses ownership.
- Simulate PID reuse, stale files, edited preferences, and an unrelated process on the gateway port.

## Risks

- Preference formats and file-lock semantics differ across platforms.
- Restoring a full preference file is unsafe; restoration must be key-scoped and conflict-aware.

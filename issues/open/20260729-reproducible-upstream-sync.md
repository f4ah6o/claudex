# Make upstream synchronization reproducible and auditable

Status: open
Model: GPT-5
Created: 2026-07-29
Updated: 2026-07-29
Branch: chore/20260729-upstream-sync

## Summary

Define and automate a repeatable process for importing CLIProxyAPI updates while preserving Claudex-owned product boundaries and producing an auditable change report.

## Problem

Claudex intentionally retains the upstream module path and a large portion of the upstream codebase. The README names files that should survive synchronization, but there is no machine-readable ownership boundary, recorded upstream base, or automated check showing which upstream changes affect the supported Claudex surface.

## Agent Prompt

Implement a synchronization workflow that never merges upstream blindly.

1. Record the exact upstream repository and commit currently incorporated.
2. Add a machine-readable manifest classifying paths as upstream-owned, Claudex-owned, or intentionally patched.
3. Create a script that fetches a selected upstream ref into a temporary worktree, computes changes from the recorded base, and applies only upstream-owned paths automatically.
4. Produce a report for conflicts, deleted paths, dependency changes, new route/provider/plugin code, security-sensitive changes, and modifications touching Claudex patches.
5. Require focused policy, route, E2E, secret scanning, vulnerability, and full test suites before updating the recorded base.
6. Add documentation for resolving conflicts without overwriting Claudex-owned files.
7. Add CI validation that the manifest covers all tracked source paths and that the recorded upstream commit exists.

Do not add an unattended scheduled merge or automatically publish synchronized code.

## Acceptance Criteria

- [ ] The repository records a verifiable upstream base commit.
- [ ] Every relevant tracked path has an ownership classification or an explicit exclusion.
- [ ] Dry-run synchronization produces a deterministic Markdown or JSON report without changing the working tree.
- [ ] Apply mode refuses unresolved Claudex-owned or patched-path conflicts.
- [ ] Dependency and route-surface changes are highlighted for manual review.
- [ ] The recorded base advances only after all required checks pass.
- [ ] The workflow is documented in `AGENTS.md` and a maintainer document.

## Test Plan

- Test no-op sync against the recorded base.
- Test a fixture upstream commit that changes owned, patched, and Claudex-owned paths.
- Test upstream deletion, rename, new dependency, and new route/provider detection.
- Verify deterministic reports and clean-worktree enforcement.

## Risks

- Path ownership alone cannot detect semantic coupling through shared packages; architecture and E2E tests remain mandatory.
- Upstream history rewrites or unavailable commits require an explicit recovery process.

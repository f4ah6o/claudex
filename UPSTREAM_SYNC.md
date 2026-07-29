# Upstream synchronization

Claudex retains the upstream module path so protocol translation and Codex authentication can be synchronized without copying the upstream project into a second module. Synchronization is explicit and review-driven.

The ownership manifest is [.claudex/upstream-manifest.json](.claudex/upstream-manifest.json). Claudex-owned and intentionally excluded paths are never changed by the sync script. The recorded upstream base is a verifiable commit and must be advanced only after the required checks pass.

Generate a deterministic report for the recorded base:

```sh
python3 scripts/claudex-upstream-sync.py --json
```

Inspect a selected upstream commit or branch:

```sh
python3 scripts/claudex-upstream-sync.py --ref <commit-or-ref> --json
```

Apply changes only to upstream-owned paths after reviewing the report:

```sh
python3 scripts/claudex-upstream-sync.py --ref <commit-or-ref> --apply
```

The command requires a clean worktree in apply mode. It does not update the recorded base, create a commit, push a branch, or publish artifacts. After applying, review dependency, route/provider/plugin, and security-sensitive findings, run the focused policy and E2E tests, secret scanning, vulnerability checks, and the full Go test suite. Update `base_commit` in the manifest only after those checks pass.

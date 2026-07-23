# Changes

## Unreleased

### Added

- Added the macOS `ClaudexDesktop.app` launcher for Claude Code Desktop, including loopback Gateway setup, fixed `gpt-5.6-luna` / `xhigh` model discovery, and standard-provider restoration. (`issues/open/20260723-claude-desktop-launcher.md`)

### Changed

- Exposed an authenticated Anthropic-format `/v1/models` catalog containing the single fixed ClaudexDesktop model while preserving the focused proxy route policy.

### Fixed

### Deprecated

### Removed

### Security

- Updated the Go toolchain requirement and vulnerable Go dependencies; `govulncheck` now reports no reachable vulnerabilities.

### Migration

- On macOS, build `ClaudexDesktop.app`, run its bundled Codex login command once, and launch the app from Finder for the Desktop Gateway workflow.
- Builds now require Go 1.26.5 or newer.

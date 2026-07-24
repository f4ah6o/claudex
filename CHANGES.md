# Changes

## Unreleased

### Added

- Added the macOS `ClaudexDesktop.app` launcher for Claude Desktop, including loopback Gateway setup, Sol/Terra/Luna model discovery, and standard-provider restoration. (`issues/closed/20260723-claude-desktop-launcher.md`)
- Added full-history secret scanning and reachable Go vulnerability checks for public repository changes. (`issues/closed/20260724-public-release-readiness.md`)
- Added a non-launching `--build-only` mode for reproducible macOS Desktop bundle checks.

### Changed

- Exposed an authenticated Anthropic-format `/v1/models` catalog containing the three Codex-backed profiles: Opus/Sol, Sonnet/Terra, and Haiku/Luna.
- Documented Claudex as an independent local single-user project, including the name origin, credential boundaries, supported installation method, and retained upstream Go module path.

### Fixed

- Ignored repository-local Claudex configuration and Desktop preference backup files so generated credentials are not committed during normal use.
- Aligned legacy, versioned, Desktop, and launcher model mappings with Opus/Sol, Sonnet/Terra, and Haiku/Luna.

### Deprecated

### Removed

### Security

- Updated the Go toolchain requirement and vulnerable Go dependencies; `govulncheck` reports no reachable vulnerabilities.
- Added CI checks that scan the complete Git history for secrets and run `govulncheck ./...`.

### Migration

- On macOS, build `ClaudexDesktop.app`, run its bundled Codex login command once, and launch the app from Finder for the Desktop Gateway workflow.
- Builds require Go 1.26.5 or newer.
- Install from a repository clone with `just setup` or by building `./cmd/claudex`; the retained upstream module path does not support `go install github.com/f4ah6o/claudex/...`.

# Add Linux support for ClaudexDesktop

Status: open
Model: unknown
Created: 2026-07-24
Updated: 2026-07-24
Branch: feat/linux-claudexdesktop

## 概要

Extend `cmd/claudexdesktop` so it can launch a compatible Claude Desktop installation on Linux while preserving the existing macOS behavior. Linux mode must use a dedicated launch contract and must not modify the normal Desktop configuration files.

## 背景

The existing ClaudexDesktop implementation starts the loopback Claudex server, configures Claude Desktop's Third-Party Inference Gateway settings, launches Claude Desktop, and restores the previous settings. The implementation currently uses macOS-specific `defaults`, AppleScript, `open`, process names, and Library paths.

The public Claudex repository must remain independently buildable. Linux support should expose only a generic compatibility contract and must not contain identifiers, URLs, or implementation details for any particular non-public distribution.

## 問題

`cmd/claudexdesktop/main.go` rejects every non-Darwin platform. As a result, Linux users cannot start the local gateway and a compatible Claude Desktop installation through the same managed workflow. Directly editing the normal Linux Desktop profile would also risk corrupting or leaking the user's standard configuration.

## 目標

- Support Linux x86_64 and aarch64 builds of ClaudexDesktop.
- Start or reuse the loopback Claudex server and launch the configured Desktop command in a dedicated Claudex mode.
- Pass gateway settings to the child process through environment variables, never command-line arguments.
- Keep the standard Desktop configuration untouched on Linux.
- Preserve the current macOS preference backup, restore, and launch behavior.
- Provide generic documentation and tests without depending on a particular external distribution.

## 対象外

- Windows Desktop launcher support.
- Modifying or rebuilding the Claude Desktop application itself.
- Adding generic proxy routes or non-Codex providers.
- Packaging ClaudexDesktop for a particular external Linux distribution.
- Publishing or documenting private packaging infrastructure in this repository.

## 提案する方針

1. Split platform-specific Desktop operations behind a small internal interface or platform-specific implementation while leaving the current macOS path behaviorally unchanged.
2. Add a Linux implementation with a configurable Desktop command, defaulting to `claude-desktop`, and configurable process detection suitable for packaged and user-local installations.
3. Use a child-process environment contract including `CLAUDEX_DESKTOP_MODE`, `CLAUDEX_GATEWAY_BASE_URL`, `CLAUDEX_GATEWAY_API_KEY`, `CLAUDEX_GATEWAY_AUTH_SCHEME`, and `CLAUDEX_INFERENCE_MODELS`. Do not place the local API key in process arguments or logs.
4. Detect an already-running Desktop process. Use `zenity` or `kdialog` when available and a terminal fallback when interactive; cancel without changing the existing process.
5. Reuse an already-ready Claudex server, start one when needed, wait for readiness, and leave the server running after the Desktop process exits. A server started by another process must not be stopped.
6. Keep Linux configuration handling separate from the macOS `defaults` and config-library implementation. Linux mode must not write the normal Desktop profile.
7. Add focused tests for Linux command construction, environment propagation, process lifecycle, first-run configuration, and failure cleanup. Keep public README wording generic and omit non-public distribution identifiers and URLs.

## 受け入れ条件

- [ ] `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/claudexdesktop` succeeds.
- [ ] `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/claudexdesktop` succeeds.
- [ ] Linux ClaudexDesktop creates or validates the Claudex configuration and reports the existing Codex login requirement without displaying secret values.
- [ ] Linux ClaudexDesktop starts or reuses a loopback server, waits for readiness, launches the configured Desktop command, and leaves the server running after Desktop exits.
- [ ] Linux gateway settings are passed through the dedicated environment contract and the API key is absent from command-line arguments and launcher logs.
- [ ] Linux mode does not modify the normal Desktop configuration, including when startup is cancelled or fails.
- [ ] An already-running Desktop process triggers an explicit restart confirmation; cancellation leaves it unchanged.
- [ ] Existing macOS launcher tests and behavior continue to pass.
- [ ] Tracked public files contain no particular non-public distribution identifiers or URLs.

## テスト計画

- Run `gofmt -w .` after Go changes.
- Run focused tests for `cmd/claudexdesktop` and `internal/claudex`.
- Run `go test ./...`.
- Run `go build -o test-output ./cmd/server && rm test-output`.
- Cross-build the Linux amd64 and arm64 ClaudexDesktop binaries.
- Use fake Desktop commands and temporary XDG directories to verify environment propagation, cancellation, process restart, and unchanged configuration.
- Run the existing macOS bundle verification where the required platform is available.
- Review public documentation and repository search results for unintended non-public integration references.

## リスク

- Linux Desktop implementations may differ in process names, command paths, single-instance behavior, and environment handling; configurable command and process detection are required.
- Same-user processes may inspect environment variables. The local key is already user-scoped, but it must not be exposed in arguments or logs.
- A Desktop process that ignores the dedicated environment contract will not enter Claudex mode; the Linux integration issue must provide the corresponding consumer separately.
- Interruptions must not require profile restoration on Linux because Linux mode must avoid modifying the normal profile.

## 変更履歴

`CHANGES.md` impact: yes

項目案：

- Add Linux support to ClaudexDesktop through a generic dedicated launch mode for compatible Claude Desktop installations.

## 注記

- The existing macOS launcher issue is recorded at `issues/closed/20260723-claude-desktop-launcher.md`.
- The Linux package integration is tracked separately in the packaging repository.

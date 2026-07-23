# Add ClaudexDesktop launcher for Claude Code Desktop

Status: open
Model: unknown
Created: 2026-07-23
Updated: 2026-07-23
Branch: feat/20260723-claude-desktop-launcher

## 概要

Add a macOS `ClaudexDesktop.app` launcher that starts the bundled Claudex server, configures Claude Desktop to use the local Anthropic-compatible gateway, and restores the standard Claude Desktop configuration when the Claudex session ends.

## 背景

Claudex currently provides a focused Anthropic Messages gateway for Claude Code CLI. The shell launcher in `scripts/claudex.sh` starts the local server and passes `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` to the CLI process, but Claude Desktop does not use those environment variables for third-party inference routing.

Claude Desktop supports a Gateway provider through its Third-Party Inference configuration. The documented gateway surface requires `POST /v1/messages` with streaming and tool use; `GET /v1/models` is optional but is used for model discovery when available. See [Claude Code gateway configuration](https://code.claude.com/docs/en/llm-gateway-connect) and [Claude Desktop third-party inference](https://claude.com/docs/third-party/claude-desktop/gateway).

## 問題

There is no Finder-launched workflow for using Claude Code Desktop through Claudex. Users must manually manage the local server and Claude Desktop's provider configuration, and the current Claudex middleware blocks the model discovery route needed for a natural Desktop model picker.

## 目標

Provide a macOS-only `ClaudexDesktop.app` that makes the fixed Codex-backed Claude Code Desktop workflow easy to start and safe to return to the normal Anthropic Claude Desktop workflow.

## 対象外

- Changing the standard Claude Desktop application bundle.
- Supporting Windows or Linux Desktop launchers in this issue.
- Exposing Claudex beyond loopback.
- Adding non-Codex providers or general-purpose proxy routes.
- User-selectable Codex model switching; the first version remains fixed to `gpt-5.6-luna` with `xhigh` effort.
- Changing the existing Claude Code CLI launcher behavior.

## 提案する方針

1. Add a small Go launcher under a new macOS-specific command/package and package it as `ClaudexDesktop.app`.
2. Bundle the native `claudex-server` binary inside the app while keeping the user configuration at `~/.config/claudex/claudex.yaml` and Codex auth material under the configured Claudex auth directory.
3. On first launch, create the configuration template when it is missing, explain the required local client key and Codex login steps, and stop without printing secret values.
4. Detect an already-running Claude Desktop process, ask for confirmation, and relaunch it after the provider configuration has changed.
5. Start Claudex automatically when it is not already listening, wait for readiness, and fail with an actionable error without opening Claude Desktop when startup fails. Keep the local server running after the Desktop process exits.
6. Configure the Claude Desktop third-party Gateway provider with the loopback URL, the configured local API key, and Bearer authentication. Preserve and restore the prior standard configuration, with a recovery path when the launcher is interrupted or the Desktop configuration format is not recognized.
7. Extend the focused Claudex API surface with an authenticated Anthropic-compatible `GET /v1/models` response containing one fixed Claude-compatible entry whose requests route to `gpt-5.6-luna`. Keep `/v1/messages` and `/v1/messages/count_tokens` restrictions and the existing GPT-5.6 policy.
8. Set the fixed model and effort behavior so Desktop requests use `gpt-5.6-luna` and `xhigh`, regardless of the single displayed model's Claude-compatible identifier.
9. Add macOS packaging/build instructions and document the normal-mode recovery and fallback behavior.

## 受け入れ条件

- [ ] Building the project produces a launchable `ClaudexDesktop.app` containing the matching `claudex-server` binary.
- [ ] Double-clicking `ClaudexDesktop.app` creates or validates the Claudex configuration without displaying secret values.
- [ ] The launcher starts Claudex on `127.0.0.1:8317`, waits for readiness, and reports actionable errors for missing configuration, missing Codex auth, occupied ports, or server failure.
- [ ] If Claude Desktop is running, the launcher asks before restarting it; after confirmation, the Desktop session uses the local Gateway provider.
- [ ] Normal Claude Desktop startup remains usable without routing requests through Claudex after the Claudex session has ended normally.
- [ ] Interrupted or incompatible configuration changes have a documented recovery path that does not require modifying the Claude Desktop application bundle.
- [ ] Authenticated `GET /v1/models` returns exactly one selectable fixed model entry, and its `POST /v1/messages` requests are accepted and routed to `gpt-5.6-luna`.
- [ ] The fixed request path preserves `xhigh` effort behavior and does not expose non-GPT-5.6 models or generic proxy routes.
- [ ] The server remains loopback-only and does not log API keys, OAuth tokens, or other credentials.
- [ ] Existing CLI compatibility tests continue to pass, and the new launcher/API behavior has focused automated or manual verification.

## テスト計画

- Run `gofmt -w .` after Go changes.
- Run focused tests for the new launcher and Claudex middleware/model listing.
- Run `go test ./...`.
- Run `go build -o test-output ./cmd/server && rm test-output` as required by the repository instructions.
- Build the macOS app and verify it launches on the current macOS environment.
- Manually verify normal Claude Desktop launch, ClaudexDesktop launch, model discovery, streaming/tool use, clean exit restoration, and recovery after forced termination.
- Verify that requests to non-loopback or non-allowed generic routes remain rejected.

## リスク

- Claude Desktop's local third-party inference settings are documented through the app and managed configuration surfaces, but a general-purpose user CLI toggle is not documented. Preference-key changes may require a version check and a manual fallback after a Desktop update.
- Storing the local gateway key in the Desktop provider configuration may expose it to local user-level application preferences; the implementation must not print it or place it in repository files.
- Claude Desktop may require recognizable Claude-family model IDs for discovery, so the fixed entry must retain a Claude-compatible ID while its display name explains the Codex-backed model.
- The server is intentionally left running after the Desktop session to make mode switching reliable; it remains loopback-only and must not interfere with normal Anthropic Desktop traffic.

## 変更履歴

`CHANGES.md` impact: yes

Proposed item:

- Add the macOS `ClaudexDesktop` launcher and Claude Desktop Gateway integration for the fixed Codex `gpt-5.6-luna` / `xhigh` workflow.

## 注記

- The current example configuration maps Claude-compatible aliases to `gpt-5.6-luna` in `claudex.example.yaml`.
- The existing Claudex policy accepts direct `gpt-5.6` / `gpt-5.6-*` models and configured Codex aliases while restricting the public API surface.

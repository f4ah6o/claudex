# Claudex

[日本語](README.ja.md)

Claudex is a focused local gateway that exposes OpenAI Codex models through 
the Anthropic Messages API used by Claude Code.

It is an independent open-source project and is not an official OpenAI or Anthropic product. 
Claude, Claude Code, Codex, and OpenAI are names and trademarks of their respective owners.

The supported product surface is intentionally small:

- client: Claude Code and Claude Desktop's Third-Party Inference Gateway
- inbound protocol: Anthropic Messages API
- upstream provider: OpenAI Codex OAuth or Codex-compatible API keys
- allowed models: `gpt-5.6` and `gpt-5.6-*`
- network exposure: loopback only
- usage model: local, single-user operation
- management UI, plugins, and other providers: disabled

## Model mapping

The example configuration maps Claude-compatible IDs to three Codex model profiles:

| Claude profile | Codex model | Desktop label |
| --- | --- | --- |
| Opus | `gpt-5.6-sol` | Codex GPT-5.6 Sol |
| Sonnet | `gpt-5.6-terra` | Codex GPT-5.6 Terra |
| Haiku | `gpt-5.6-luna` | Codex GPT-5.6 Luna |

Claude Code may send versioned or built-in Claude model IDs. `claudex.example.yaml` contains the supported aliases for those IDs. Direct requests to `gpt-5.6` and any `gpt-5.6-*` model are also accepted. Models outside that family are rejected before provider routing.

Unless the client disables thinking or supplies its own effort setting, Claudex applies `xhigh` as the default effort.

## Structure

| Path | Responsibility |
| --- | --- |
| `cmd/claudex` | Focused CLI: `login`, `serve`, and `version` |
| `cmd/claudexdesktop` | macOS launcher for Claude Desktop |
| `internal/claudex` | Configuration validation, route restriction, and GPT-5.6 model policy |
| `claudex.example.yaml` | Minimal supported configuration |
| remaining upstream packages | Shared Codex OAuth, Anthropic↔Responses translation, streaming, tools, and auth rotation |

## Quick start

Clone the repository, build the gateway, and create a configuration:

```bash
git clone https://github.com/f4ah6o/claudex.git
cd claudex
go build -o claudex ./cmd/claudex
cp claudex.example.yaml claudex.yaml
```

Replace `replace-with-a-local-random-key` in `claudex.yaml`. This key authenticates local Claude Code clients; it is not an upstream provider credential. Claudex refuses to serve with the placeholder value.

Authenticate Codex and start the proxy:

```bash
./claudex login
./claudex serve
```

For a device-code login, use `./claudex login --device`. Credentials are stored under `~/.claudex` by default, separate from a generic CLIProxyAPI installation. Running `./claudex` without a command is equivalent to `./claudex serve`.

Use `--config <path>` or set `CLAUDEX_CONFIG` to select another configuration file.

## Use with Claude Code

Point Claude Code at the local gateway and select a supported model and effort level:

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8317"
export ANTHROPIC_AUTH_TOKEN="the-local-key-from-claudex.yaml"

claude --model opus --effort xhigh
```

Claude-compatible model IDs are routed according to the Sol, Terra, and Luna mapping above. A direct `gpt-5.6-*` model name bypasses the Claude-profile alias while remaining inside the allowed model family.

To keep the normal Anthropic Claude command unchanged, remove the local gateway variables before using it:

```bash
unset ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN
claude --model opus
```

The included native launchers automate this separation. `claude` remains the normal Anthropic command, while `claudex` starts or reuses the local gateway and launches Claude Code with the Opus/Sol profile by default. The launcher also defines Sonnet/Terra and Haiku/Luna defaults for Claude Code's tier switching. The `./claudex` binary built from this repository is the gateway server itself.

## Claude Desktop on macOS

Build the Finder-launchable `ClaudexDesktop.app` without opening it:

```sh
./script/build_and_run.sh --build-only
```

Build and verify that the application starts:

```sh
./script/build_and_run.sh --verify
```

Copy `dist/ClaudexDesktop.app` to `~/Applications` to launch it from Finder. On first launch, the app creates `~/.config/claudex/claudex.yaml` and shows the bundled Codex login command. Run that command once, then launch `ClaudexDesktop` again.

`ClaudexDesktop` starts the loopback gateway, configures Claude Desktop's Third-Party Inference Gateway settings, and opens Claude Desktop. Its model catalog contains three entries: Codex GPT-5.6 Sol, Terra, and Luna. The launcher restores the previous Claude Desktop settings when the session ends. If it is interrupted, open `ClaudexDesktop` again to restore the pending settings backup before starting another session.

The standard `Claude Desktop` app bundle is not modified. The provider preference is changed only while `ClaudexDesktop` owns the session; the gateway remains loopback-only and can stay running after Claude Desktop exits.

## Cross-platform setup

The repository includes a `justfile` and native launchers for Windows, macOS, and Linux. Install `just` once with Cargo, then run the setup task from the repository root:

```sh
cargo install just --locked
just setup
```

`just setup` creates the configuration, generates the local client key, builds the native server, installs the launcher, and adds the launcher directory to the user `PATH` where the shell profile can be detected. On Windows ARM64 it uses `$HOME\\.config\\claudex\\claudex.yaml` and `$HOME\\bin`; on macOS/Linux it uses `${XDG_CONFIG_HOME:-$HOME/.config}/claudex/claudex.yaml` and `${XDG_BIN_HOME:-$HOME/.local/bin}`. Open a new terminal after setup.

Authenticate Codex with the browser OAuth flow and start Claude Code through the local gateway:

```sh
just login
just run
```

Use `just serve` to run only the gateway, `just build` to rebuild the native launcher, and `just verify` to run targeted tests and rebuild.

## Security and usage boundaries

Claudex is supported only as a local gateway for one user.

- Use only your own OpenAI account, OAuth session, or API key.
- Do not share or redistribute OAuth tokens, API keys, account access, or generated authentication files.
- Do not operate Claudex as a hosted service, shared gateway, or multi-user credential broker.
- Do not use Claudex to bypass usage limits, plan restrictions, access controls, or provider policy enforcement.
- Follow the applicable OpenAI and Anthropic terms and policies.
- Do not remove the loopback-only restriction. It is a security boundary, not a deployment default.

At startup, Claudex rejects configurations that enable non-Codex providers, plugins, remote management, non-loopback binding, or aliases targeting models outside `gpt-5.6` / `gpt-5.6-*`.

At request time, Anthropic clients may use `/v1/models`, `/v1/messages`, and `/v1/messages/count_tokens`. The model catalog contains the three Codex-backed profiles described above. Other generic proxy routes return an Anthropic-compatible 404 response.

## Installation and module path

Claudex intentionally retains the upstream Go module path, `github.com/router-for-me/CLIProxyAPI/v7`, to keep upstream synchronization practical. Therefore this command is not supported:

```sh
go install github.com/f4ah6o/claudex/cmd/claudex@latest
```

The supported source installation is to clone this repository and use `just setup` or build `./cmd/claudex` from the clone. A binary is an official distribution only when it is attached to a tagged release in this repository.

## Docker

Because Claudex enforces a loopback-only listener, use host networking on Linux:

```bash
docker build -t claudex .
docker run --rm --network host \
  -v "$PWD/claudex.yaml:/app/claudex.yaml:ro" \
  -v "$HOME/.claudex:/root/.claudex" \
  claudex
```

## Development

```bash
go test ./internal/claudex ./cmd/claudex
go build -o claudex ./cmd/claudex
govulncheck ./...
gitleaks detect --log-opts="--all"
```

On macOS, verify the Desktop bundle with `./script/build_and_run.sh --build-only` or `--verify`.

Keep upstream changes isolated from the focused product layer. Normal synchronization should preserve `cmd/claudex`, `cmd/claudexdesktop`, `internal/claudex`, `claudex.example.yaml`, and the Claudex Docker target.

## Acknowledgements

Claudex is based on [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). We acknowledge and thank the upstream maintainers and contributors. Claudex keeps the upstream core available so protocol translation and Codex authentication can continue to benefit from upstream work while exposing a deliberately smaller product surface.

## License

MIT. See [LICENSE](LICENSE). Third-party dependencies remain subject to their respective licenses.

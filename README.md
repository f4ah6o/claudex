# Claudex

Claudex runs OpenAI Codex OAuth models from the `gpt-5.6` family behind the Anthropic Messages API used by Claude Code.

This repository is a focused fork of `router-for-me/CLIProxyAPI`. The upstream proxy core remains in the tree so protocol translation and Codex authentication can continue to be synchronized, while the supported product entrypoint is intentionally narrow:

- client: Claude Code
- inbound protocol: Anthropic Messages API
- upstream provider: OpenAI Codex OAuth or Codex-compatible API keys
- allowed models: `gpt-5.6` and `gpt-5.6-*`
- network exposure: loopback only
- management UI, plugins, and other providers: disabled

## Structure

| Path | Responsibility |
| --- | --- |
| `cmd/claudex` | Focused CLI: `login`, `serve`, and `version` |
| `internal/claudex` | Configuration validation, route restriction, and GPT-5.6 model policy |
| `claudex.example.yaml` | Minimal supported configuration |
| remaining upstream packages | Shared Codex OAuth, Anthropic↔Responses translation, streaming, tools, and auth rotation |

`cmd/server` is retained as upstream implementation material. It is not the supported Claudex executable.

## Build

````bash
go build -o claudex ./cmd/claudex
````

## Configure

````bash
cp claudex.example.yaml claudex.yaml
````

Replace `replace-with-a-local-random-key` in `claudex.yaml`. Claudex refuses to serve with the placeholder value.

The example maps Claude Code's built-in Opus, Sonnet, and Haiku model IDs to `gpt-5.6-sol`. Direct requests to `gpt-5.6` and any `gpt-5.6-*` model are also accepted. Models outside that family are rejected before provider routing.

## Authenticate Codex

Browser login:

````bash
./claudex login
````

Device-code login:

````bash
./claudex login --device
````

Credentials are stored under `~/.claudex` by default, separate from a generic CLIProxyAPI installation.

## Start the proxy

````bash
./claudex serve
````

Running `./claudex` without a command is equivalent to `./claudex serve`.

## Configure Claude Code

````bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8317"
export ANTHROPIC_AUTH_TOKEN="the-local-key-from-claudex.yaml"
export ANTHROPIC_MODEL="gpt-5.6-sol"
export ANTHROPIC_DEFAULT_OPUS_MODEL="gpt-5.6-sol"
export ANTHROPIC_DEFAULT_SONNET_MODEL="gpt-5.6-sol"
export ANTHROPIC_DEFAULT_HAIKU_MODEL="gpt-5.6-sol"

claude
````

The Claude model aliases in `claudex.yaml` provide a fallback when Claude Code sends one of its built-in model IDs.

## Docker

````bash
docker build -t claudex .
docker run --rm \
  -p 127.0.0.1:8317:8317 \
  -v "$PWD/claudex.yaml:/app/claudex.yaml:ro" \
  -v "$HOME/.claudex:/root/.claudex" \
  claudex
````

## Enforced boundaries

At startup, Claudex rejects configurations that enable non-Codex providers, plugins, remote management, non-loopback binding, or aliases targeting models outside `gpt-5.6` / `gpt-5.6-*`.

At request time, only `/v1/messages` and `/v1/messages/count_tokens` are exposed. Other generic proxy routes return an Anthropic-compatible 404 response.

## Test

````bash
go test ./internal/claudex ./cmd/claudex
````

## Upstream synchronization

Keep upstream changes isolated from the focused product layer. Normal synchronization should update the inherited core while preserving:

- `cmd/claudex`
- `internal/claudex`
- `claudex.example.yaml`
- the Claudex Docker target

## License

MIT. See [LICENSE](LICENSE).

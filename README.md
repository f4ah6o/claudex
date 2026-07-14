# Claudex

[日本語](README.ja.md)

Claudex is a focused local gateway that exposes OpenAI Codex models through the Anthropic Messages API used by Claude Code.

The supported product surface is intentionally small:

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

## Quick start

Build and create a configuration:

```bash
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

Point Claude Code at the local gateway and select the supported model and effort level:

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8317"
export ANTHROPIC_AUTH_TOKEN="the-local-key-from-claudex.yaml"

claude --model gpt-5.6-luna --effort xhigh
```

`xhigh` is passed through Claude Code's effort setting and does not require a model-name suffix. The example configuration maps Claude Code's built-in Opus, Sonnet, and Haiku IDs to `gpt-5.6-luna`; direct requests to `gpt-5.6` and any `gpt-5.6-*` model are also accepted. Models outside that family are rejected before provider routing.

To keep the normal Anthropic Claude command unchanged, remove the local gateway variables before using it:

```bash
unset ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN
claude --model opus
```

An optional shell launcher can automate this separation: `claude` invokes the normal Anthropic client, while a separate `claudex` launcher starts the local gateway and invokes Claude Code with `gpt-5.6-luna` and `xhigh`. The `./claudex` binary built from this repository is the gateway server itself.

## Configuration boundaries

At startup, Claudex rejects configurations that enable non-Codex providers, plugins, remote management, non-loopback binding, or aliases targeting models outside `gpt-5.6` / `gpt-5.6-*`.

At request time, only `/v1/messages` and `/v1/messages/count_tokens` are exposed. Other generic proxy routes return an Anthropic-compatible 404 response.

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
```

Keep upstream changes isolated from the focused product layer. Normal synchronization should preserve `cmd/claudex`, `internal/claudex`, `claudex.example.yaml`, and the Claudex Docker target.

## Acknowledgements

Claudex is based on [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). We acknowledge and thank the upstream maintainers and contributors. Claudex keeps the upstream core available so protocol translation and Codex authentication can continue to benefit from upstream work while exposing a deliberately smaller product surface.

## License

MIT. See [LICENSE](LICENSE).

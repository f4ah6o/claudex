# Claude Desktop on Linux

Claudex can launch the Linux build from [`f4ah6o/claude-desktop-linux`](https://github.com/f4ah6o/claude-desktop-linux) in Claude Desktop's third-party inference mode.

## Prerequisites

- `claude-desktop` is installed and available on `PATH`
- Claudex has been set up with `just setup`
- Codex OAuth has been completed with `just login`

## Run

```sh
just desktop
```

The Linux launcher:

1. restores any configuration backup left by an interrupted previous session
2. stops a running Claude Desktop process so the app reloads its inference profile
3. backs up the normal and third-party Claude Desktop configuration files
4. writes `deploymentMode: "3p"` under the XDG configuration root
5. writes a Claudex gateway profile under `Claude-3p/configLibrary`
6. starts or reuses the loopback Claudex server
7. launches `/usr/bin/claude-desktop`
8. restores the previous Claude Desktop configuration after the app exits

The profile exposes Claude-compatible Opus, Sonnet, and Haiku route IDs. Claudex maps those routes to Sol, Terra, and Luna respectively. The Linux profile stores `inferenceModels` as a JSON array rather than a JSON-encoded preference string.

## Paths and overrides

The launcher follows XDG paths:

- Claudex configuration: `${XDG_CONFIG_HOME:-$HOME/.config}/claudex/claudex.yaml`
- Claude Desktop configuration: `${XDG_CONFIG_HOME:-$HOME/.config}/Claude*`
- installed launcher: `${XDG_BIN_HOME:-$HOME/.local/bin}/claudex-desktop-linux`

Environment overrides:

- `CLAUDEX_CONFIG`
- `CLAUDEX_SERVER_PATH`
- `CLAUDEX_CLAUDE_DESKTOP_COMMAND`

Gateway logs are written to:

- `~/.claudex/desktop-linux-serve.stdout.log`
- `~/.claudex/desktop-linux-serve.stderr.log`

The Claude Desktop Linux launcher keeps its own log under `${XDG_CACHE_HOME:-$HOME/.cache}/claude-desktop/launcher.log`.

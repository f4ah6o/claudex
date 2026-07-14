#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
config_path=${CLAUDEX_CONFIG:-"$HOME/.config/claudex/claudex.yaml"}
server_path=${CLAUDEX_SERVER_PATH:-"$script_dir/claudex-server"}
base_url=${CLAUDEX_BASE_URL:-http://127.0.0.1:8317}
base_url=${base_url%/}

if [ ! -f "$config_path" ]; then
    printf 'Claudex configuration not found: %s\n' "$config_path" >&2
    exit 1
fi
if [ ! -x "$server_path" ]; then
    printf 'Claudex server binary not found: %s\n' "$server_path" >&2
    exit 1
fi

local_key=$(awk '
    /^[[:space:]]*api-keys:[[:space:]]*$/ { in_keys=1; next }
    in_keys && /^[[:space:]]*-/ {
        value=$0
        sub(/^[[:space:]]*-[[:space:]]*/, "", value)
        print value
        exit
    }
    in_keys && $0 !~ /^[[:space:]]/ { exit }
' "$config_path" | sed -e 's/^"//' -e 's/"[[:space:]]*$//' -e "s/^'//" -e "s/'[[:space:]]*$//")
case "$local_key" in
    ''|replace-*|your-api-key*)
        printf 'claudex.yaml still contains a placeholder API key\n' >&2
        exit 1
        ;;
esac

is_ready() {
    command -v curl >/dev/null 2>&1 && curl -sS --max-time 2 "$base_url/" >/dev/null 2>&1
}

if ! is_ready; then
    if [ "${CLAUDEX_NO_START:-0}" = "1" ]; then
        printf 'Claudex server is not running at %s\n' "$base_url" >&2
        exit 1
    fi

    log_dir=${CLAUDEX_LOG_DIR:-"$HOME/.claudex"}
    mkdir -p "$log_dir"
    "$server_path" serve --config "$config_path" >"$log_dir/serve.stdout.log" 2>"$log_dir/serve.stderr.log" &
    server_pid=$!
    ready=0
    attempt=0
    while [ "$attempt" -lt 60 ]; do
        if is_ready; then
            ready=1
            break
        fi
        if ! kill -0 "$server_pid" 2>/dev/null; then
            break
        fi
        attempt=$((attempt + 1))
        sleep 0.25
    done
    if [ "$ready" -ne 1 ]; then
        printf 'Claudex server did not become ready at %s\n' "$base_url" >&2
        if [ -f "$log_dir/serve.stderr.log" ]; then
            tail -n 20 "$log_dir/serve.stderr.log" >&2 || true
        fi
        exit 1
    fi
fi

claude_command=${CLAUDEX_CLAUDE_COMMAND:-claude}
command_path=$(command -v "$claude_command" 2>/dev/null || true)
if [ -z "$command_path" ]; then
    printf 'Claude Code command not found: %s\n' "$claude_command" >&2
    exit 1
fi

ANTHROPIC_BASE_URL="$base_url" \
ANTHROPIC_AUTH_TOKEN="$local_key" \
ANTHROPIC_MODEL="gpt-5.6-luna" \
ANTHROPIC_DEFAULT_OPUS_MODEL="gpt-5.6-luna" \
ANTHROPIC_DEFAULT_SONNET_MODEL="gpt-5.6-luna" \
ANTHROPIC_DEFAULT_HAIKU_MODEL="gpt-5.6-luna" \
CLAUDE_CODE_EFFORT_LEVEL="xhigh" \
CLAUDE_CODE_ALWAYS_ENABLE_EFFORT="1" \
exec "$command_path" --model gpt-5.6-luna --effort xhigh "$@"

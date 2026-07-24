#!/bin/sh

set -eu

task=${1:-}
case "$task" in
    setup|build|login|serve|run|desktop|verify|version)
        ;;
    *)
        printf '%s\n' 'usage: claudex-tasks.sh {setup|build|login|serve|run|desktop|verify|version}' >&2
        exit 2
        ;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
config_dir=${XDG_CONFIG_HOME:-"$HOME/.config"}/claudex
config_path=$config_dir/claudex.yaml
install_dir=${XDG_BIN_HOME:-"$HOME/.local/bin"}
server_path=$install_dir/claudex-server
launcher_path=$install_dir/claudex
desktop_path=$install_dir/claudex-desktop-linux
template_path=$repo_root/claudex.example.yaml

new_local_api_key() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 32
    else
        od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
    fi
}

ensure_config() {
    mkdir -p "$config_dir"
    if [ ! -f "$config_path" ]; then
        cp "$template_path" "$config_path"
        key=$(new_local_api_key)
        sed "s/replace-with-a-local-random-key/$key/g" "$config_path" > "$config_path.tmp"
        mv "$config_path.tmp" "$config_path"
        printf 'Created Claudex configuration: %s\n' "$config_path"
    elif grep -Fq 'replace-with-a-local-random-key' "$config_path"; then
        key=$(new_local_api_key)
        sed "s/replace-with-a-local-random-key/$key/g" "$config_path" > "$config_path.tmp"
        mv "$config_path.tmp" "$config_path"
        printf 'Replaced the placeholder local API key in: %s\n' "$config_path"
    fi
}

ensure_install() {
    mkdir -p "$install_dir"
    native_goos=$(go env GOOS)
    native_goarch=$(go env GOARCH)
    CGO_ENABLED=0 GOOS="$native_goos" GOARCH="$native_goarch" go build -o "$server_path" ./cmd/claudex
    if [ "$native_goos" = linux ]; then
        CGO_ENABLED=0 GOOS="$native_goos" GOARCH="$native_goarch" go build -o "$desktop_path" ./cmd/claudexdesktoplinux
    fi
    cp "$repo_root/scripts/claudex.sh" "$launcher_path"
    chmod 755 "$launcher_path"
    printf 'Installed Claudex launcher and server in: %s\n' "$install_dir"
}

ensure_user_path() {
    case ":${PATH:-}:" in
        *:"$install_dir":*)
            return
            ;;
    esac

    profile=
    shell_name=${SHELL##*/}
    case "$shell_name" in
        bash)
            profile=$HOME/.profile
            ;;
        zsh)
            profile=$HOME/.zprofile
            ;;
    esac

    if [ -n "$profile" ]; then
        path_line="export PATH=\"$install_dir:\$PATH\""
        if [ ! -f "$profile" ] || ! grep -Fqx "$path_line" "$profile"; then
            {
                printf '\n# Added by Claudex\n'
                printf '%s\n' "$path_line"
            } >> "$profile"
        fi
        printf 'Added %s to %s. Open a new terminal to use claudex directly.\n' "$install_dir" "$profile"
    else
        printf 'Add %s to PATH to use claudex directly.\n' "$install_dir"
    fi
}

invoke_server() {
    if [ ! -x "$server_path" ]; then
        ensure_install
    fi
    exec "$server_path" "$1" --config "$config_path"
}

case "$task" in
    setup)
        ensure_config
        ensure_install
        ensure_user_path
        ;;
    build)
        ensure_install
        ;;
    login)
        ensure_config
        invoke_server login
        ;;
    serve)
        ensure_config
        invoke_server serve
        ;;
    run)
        ensure_config
        [ -x "$launcher_path" ] || ensure_install
        exec "$launcher_path"
        ;;
    desktop)
        if [ "$(go env GOOS)" != linux ]; then
            printf '%s\n' 'The native Desktop launcher is available on Linux only. Use ClaudexDesktop.app on macOS.' >&2
            exit 2
        fi
        ensure_config
        [ -x "$desktop_path" ] || ensure_install
        exec "$desktop_path"
        ;;
    verify)
        go test ./internal/claudex ./cmd/claudex ./cmd/claudexdesktoplinux
        ensure_install
        printf '%s\n' 'Claudex verification completed.'
        ;;
    version)
        [ -x "$server_path" ] || ensure_install
        exec "$server_path" version
        ;;
esac

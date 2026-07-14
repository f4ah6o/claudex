set windows-shell := ["pwsh.exe", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command"]

default:
    @just --list

setup:
    @{{ if os() == "windows" { "pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File scripts\\claudex-tasks.ps1 setup" } else { "sh scripts/claudex-tasks.sh setup" } }}

build:
    @{{ if os() == "windows" { "pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File scripts\\claudex-tasks.ps1 build" } else { "sh scripts/claudex-tasks.sh build" } }}

login: setup
    @{{ if os() == "windows" { "pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File scripts\\claudex-tasks.ps1 login" } else { "sh scripts/claudex-tasks.sh login" } }}

serve: setup
    @{{ if os() == "windows" { "pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File scripts\\claudex-tasks.ps1 serve" } else { "sh scripts/claudex-tasks.sh serve" } }}

run: setup
    @{{ if os() == "windows" { "pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File scripts\\claudex-tasks.ps1 run" } else { "sh scripts/claudex-tasks.sh run" } }}

verify:
    @{{ if os() == "windows" { "pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File scripts\\claudex-tasks.ps1 verify" } else { "sh scripts/claudex-tasks.sh verify" } }}

version: build
    @{{ if os() == "windows" { "pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File scripts\\claudex-tasks.ps1 version" } else { "sh scripts/claudex-tasks.sh version" } }}

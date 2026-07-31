#!/bin/sh
set -eu

uid="${CLAUDEX_UID:-}"
gid="${CLAUDEX_GID:-}"

if [ -z "$uid" ] || [ -z "$gid" ]; then
  if [ -e /app/claudex.yaml ]; then
    uid="$(stat -c %u /app/claudex.yaml)"
    gid="$(stat -c %g /app/claudex.yaml)"
  elif [ -d /home/claudex/.claudex ]; then
    uid="$(stat -c %u /home/claudex/.claudex)"
    gid="$(stat -c %g /home/claudex/.claudex)"
  else
    uid="$(id -u claudex)"
    gid="$(id -g claudex)"
  fi
fi

export HOME=/home/claudex
exec setpriv --reuid="$uid" --regid="$gid" --clear-groups /app/claudex "$@"

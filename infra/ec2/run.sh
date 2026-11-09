#!/usr/bin/env bash
# Run a one-off command with the server env loaded, e.g.:
#   ~/peoplvance/run.sh migrate status
#   ADMIN_PASSWORD='…' ~/peoplvance/run.sh create-admin --email you@peoplvance.org --role owner
# BIN=/path/to/binary overrides which binary runs (deploy.sh uses this for the new build).
# Reads .env literally (like systemd does): no shell expansion, so values with <, $ or spaces are safe.
set -euo pipefail
APP=/home/ubuntu/peoplvance
BIN="${BIN:-$APP/peoplvance-api}"
while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in ''|\#*) continue ;; esac
    key="${line%%=*}"; val="${line#*=}"
    [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
    if [[ "$val" == \"*\" || "$val" == \'*\' ]]; then val="${val:1:${#val}-2}"; fi
    # Variables already set in the caller's environment (e.g. ADMIN_PASSWORD) win.
    [ -n "${!key+x}" ] || export "$key=$val"
done < "$APP/.env"
exec "$BIN" "$@"

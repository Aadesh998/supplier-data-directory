#!/usr/bin/env bash
# Go back to the previous build: `rollback.sh api` (default) or `rollback.sh web`.
# Database migrations are not rolled back.
set -euo pipefail
APP=/home/ubuntu/peoplvance
case "${1:-api}" in
api)
    [ -f "$APP/peoplvance-api.prev" ] || { echo "No previous API binary" >&2; exit 1; }
    mv -f "$APP/peoplvance-api.prev" "$APP/peoplvance-api"
    sudo systemctl restart peoplvance-api peoplvance-gmaps-worker ;;
web)
    [ -d "$APP/web.prev" ] || { echo "No previous website build" >&2; exit 1; }
    rm -rf "$APP/web" && mv "$APP/web.prev" "$APP/web"
    sudo systemctl restart peoplvance-web ;;
*) echo "usage: rollback.sh [api|web]" >&2; exit 2 ;;
esac
echo "Rolled back $1."

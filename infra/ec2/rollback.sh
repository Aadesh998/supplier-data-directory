#!/usr/bin/env bash
# Go back to the previous binary (migrations are not rolled back).
set -euo pipefail
APP=/home/ubuntu/peoplvance
[ -f "$APP/peoplvance-api.prev" ] || { echo "No previous binary" >&2; exit 1; }
mv -f "$APP/peoplvance-api.prev" "$APP/peoplvance-api"
sudo systemctl restart peoplvance-api peoplvance-gmaps-worker
echo "Rolled back."

#!/usr/bin/env bash
# Put the uploaded binary (~/peoplvance-api.new) in place, run migrations, restart both services.
set -euo pipefail
APP=/home/ubuntu/peoplvance
NEW=/home/ubuntu/peoplvance-api.new

[ -f "$NEW" ] || { echo "No new build at $NEW" >&2; exit 1; }
[ -f "$APP/.env" ] || { echo "$APP/.env is missing - run 'make ec2-env'" >&2; exit 1; }
chmod +x "$NEW"

# Migrate with the NEW binary before switching, so a failed migration leaves the old version running.
echo "==> Migrating"
( set -a; . "$APP/.env"; set +a; "$NEW" migrate up )

[ -f "$APP/peoplvance-api" ] && cp -f "$APP/peoplvance-api" "$APP/peoplvance-api.prev"
# mv over the running binary is safe: the old process keeps its inode until restart.
mv "$NEW" "$APP/peoplvance-api"

echo "==> Restarting"
sudo systemctl restart peoplvance-api peoplvance-gmaps-worker
sleep 2
for u in peoplvance-api peoplvance-gmaps-worker; do echo "    $u: $(systemctl is-active "$u")"; done
curl -fsS -m 5 http://127.0.0.1:8090/readyz >/dev/null && echo "    API ready" || { echo "!! /readyz failed - see $APP/api.log" >&2; exit 1; }

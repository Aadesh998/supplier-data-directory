#!/usr/bin/env bash
# Unpack the uploaded website build (~/peoplvance-web.tgz) and restart the web service.
set -euo pipefail
APP=/home/ubuntu/peoplvance
TGZ=/home/ubuntu/peoplvance-web.tgz

[ -f "$TGZ" ] || { echo "No new build at $TGZ" >&2; exit 1; }
rm -rf "$APP/web.new" && mkdir -p "$APP/web.new"
tar -xzf "$TGZ" -C "$APP/web.new"
[ -f "$APP/web.new/server.js" ] || { echo "Build has no server.js" >&2; exit 1; }

rm -rf "$APP/web.prev"
[ -d "$APP/web" ] && mv "$APP/web" "$APP/web.prev"
mv "$APP/web.new" "$APP/web"
rm -f "$TGZ"

sudo systemctl restart peoplvance-web
for i in $(seq 1 20); do
    curl -fsS -m 3 -o /dev/null http://127.0.0.1:3100/robots.txt && { echo "    web: $(systemctl is-active peoplvance-web), responding"; exit 0; }
    sleep 1
done
echo "!! website not responding - see $APP/web.log (rollback: ~/peoplvance/rollback.sh web)" >&2
exit 1

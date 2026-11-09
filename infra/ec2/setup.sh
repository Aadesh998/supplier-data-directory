#!/usr/bin/env bash
# One-time box setup (safe to re-run): app folder, log rotation, systemd units, nginx site.
# Run on the server from ~/peoplvance-setup (make ec2-setup copies this folder there).
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
APP=/home/ubuntu/peoplvance

mkdir -p "$APP"
[ -f "$APP/.env" ] || echo "!! $APP/.env is missing - run 'make ec2-env' from your machine before starting the services"

sudo tee /etc/logrotate.d/peoplvance >/dev/null <<ROT
$APP/api.log $APP/gmaps-worker.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
    copytruncate
}
ROT

sudo cp "$HERE"/peoplvance-*.service "$HERE"/peoplvance-*.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable peoplvance-api peoplvance-gmaps-worker >/dev/null
sudo systemctl enable --now peoplvance-maintenance.timer >/dev/null

# Don't overwrite the site once certbot has added HTTPS to it.
SITE=/etc/nginx/sites-available/api.peoplvance.org
if sudo grep -q "managed by Certbot" "$SITE" 2>/dev/null; then
    echo "nginx: $SITE already has HTTPS, left as is"
else
    sudo cp "$HERE/nginx-api.conf" "$SITE"
    sudo ln -sf "$SITE" /etc/nginx/sites-enabled/api.peoplvance.org
    sudo nginx -t && sudo systemctl reload nginx
    echo "nginx: api.peoplvance.org written. For HTTPS: sudo certbot --nginx -d api.peoplvance.org"
fi
cp "$HERE/deploy.sh" "$HERE/rollback.sh" "$APP/" && chmod +x "$APP/deploy.sh" "$APP/rollback.sh"
echo "Setup done."

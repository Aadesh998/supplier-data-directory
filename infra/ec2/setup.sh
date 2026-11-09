#!/usr/bin/env bash
# One-time box setup (safe to re-run): app folder, log rotation, systemd units, nginx site.
# Everything runs on this box: API, gmaps worker, website, daily maintenance.
# Run on the server from ~/peoplvance-setup (make ec2-setup copies this folder there).
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
APP=/home/ubuntu/peoplvance

mkdir -p "$APP"
[ -f "$APP/.env" ] || echo "!! $APP/.env is missing - run 'make ec2-env' from your machine before starting the services"

sudo tee /etc/logrotate.d/peoplvance >/dev/null <<ROT
$APP/api.log $APP/gmaps-worker.log $APP/web.log {
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
sudo systemctl enable peoplvance-api peoplvance-gmaps-worker peoplvance-web >/dev/null
sudo systemctl enable --now peoplvance-maintenance.timer >/dev/null

# Don't overwrite the site once certbot has added HTTPS to it.
SITE=/etc/nginx/sites-available/peoplvance.org
if sudo grep -q "managed by Certbot" "$SITE" 2>/dev/null; then
    echo "nginx: $SITE already has HTTPS, left as is"
else
    sudo cp "$HERE/nginx-site.conf" "$SITE"
    sudo ln -sf "$SITE" /etc/nginx/sites-enabled/peoplvance.org
    sudo nginx -t && sudo systemctl reload nginx
    echo "nginx: peoplvance.org written. For HTTPS: sudo certbot --nginx -d peoplvance.org -d www.peoplvance.org"
fi
cp "$HERE/deploy.sh" "$HERE/deploy-web.sh" "$HERE/rollback.sh" "$HERE/run.sh" "$APP/"
chmod +x "$APP/deploy.sh" "$APP/deploy-web.sh" "$APP/rollback.sh" "$APP/run.sh"
echo "Setup done."

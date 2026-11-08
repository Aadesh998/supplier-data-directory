#!/usr/bin/env bash
# Nightly PostgreSQL backup. Cron example (02:30 daily):
#   30 2 * * * /opt/bharatmfg/scripts/backup.sh >> /var/log/bharatmfg-backup.log 2>&1
# The dump contains encrypted personal fields. DATA_ENCRYPTION_KEY is NOT in the dump, so keep
# the key somewhere separate (password manager); a backup is useless without it.
set -euo pipefail
cd "$(dirname "$0")/.."
ENV_FILE=${ENV_FILE:-infra/deploy/.env}
COMPOSE=(docker compose -f infra/deploy/compose.prod.yaml --env-file "$ENV_FILE")
# Read only the keys we need (the env file is not valid shell: values like MAIL_FROM contain "<").
envval() { { grep -E "^$1=" "$ENV_FILE" || true; } | tail -1 | cut -d= -f2- | sed -e 's/^"//' -e 's/"$//'; }
POSTGRES_USER=$(envval POSTGRES_USER)
POSTGRES_DB=$(envval POSTGRES_DB)

BACKUP_DIR=$(envval BACKUP_DIR); BACKUP_DIR=${BACKUP_DIR:-/var/backups/bharatmfg}
KEEP=$(envval BACKUP_KEEP_DAYS); KEEP=${KEEP:-14}
BACKUP_REMOTE=$(envval BACKUP_REMOTE)
case "$BACKUP_DIR" in
  /var/backups/*|/srv/*|/opt/*|/mnt/*|/data/*) ;;
  *) echo "refusing BACKUP_DIR=$BACKUP_DIR (expected under /var/backups, /srv, /opt, /mnt or /data)"; exit 1 ;;
esac
mkdir -p "$BACKUP_DIR"

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT="$BACKUP_DIR/bharatmfg-$STAMP.dump"
"${COMPOSE[@]}" exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner > "$OUT.partial"
mv "$OUT.partial" "$OUT"

# Verify the archive is readable and contains the core table.
if ! "${COMPOSE[@]}" exec -T postgres pg_restore --list < "$OUT" | grep -q "TABLE public companies"; then
  echo "backup verification FAILED for $OUT"
  exit 1
fi
echo "$(date -u +%FT%TZ) backup ok: $OUT ($(du -h "$OUT" | cut -f1))"

if [[ -n "${BACKUP_REMOTE:-}" ]]; then
  rclone copy "$OUT" "$BACKUP_REMOTE/"
  echo "uploaded to $BACKUP_REMOTE"
fi

# Retention: only files matching our own naming pattern inside the validated directory.
find "$BACKUP_DIR" -maxdepth 1 -type f -name 'bharatmfg-*.dump' -mtime +"$KEEP" -print -delete

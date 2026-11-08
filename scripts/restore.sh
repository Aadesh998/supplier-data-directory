#!/usr/bin/env bash
# Restore a backup. By default this is a DRILL: it restores into a temporary database, prints
# row counts, and drops it again. With --live it replaces the production database.
#   scripts/restore.sh /var/backups/bharatmfg/bharatmfg-20261107T023000Z.dump      # drill
#   scripts/restore.sh <file> --live                                               # real restore
set -euo pipefail
cd "$(dirname "$0")/.."
FILE=${1:?usage: restore.sh <dump-file> [--live]}
ENV_FILE=${ENV_FILE:-infra/deploy/.env}
COMPOSE=(docker compose -f infra/deploy/compose.prod.yaml --env-file "$ENV_FILE")
# Read only the keys we need (the env file is not valid shell: values like MAIL_FROM contain "<").
envval() { { grep -E "^$1=" "$ENV_FILE" || true; } | tail -1 | cut -d= -f2- | sed -e 's/^"//' -e 's/"$//'; }
POSTGRES_USER=$(envval POSTGRES_USER)
POSTGRES_DB=$(envval POSTGRES_DB)

if [[ "${2:-}" == "--live" ]]; then
  read -r -p "This REPLACES the live database '$POSTGRES_DB'. Type its name to continue: " ok
  [[ "$ok" == "$POSTGRES_DB" ]] || { echo "aborted"; exit 1; }
  "${COMPOSE[@]}" stop api web
  "${COMPOSE[@]}" exec -T postgres pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner < "$FILE"
  "${COMPOSE[@]}" start api web
  echo "live restore complete"
  exit 0
fi

DRILL=restore_drill
"${COMPOSE[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d postgres -c "DROP DATABASE IF EXISTS $DRILL" -c "CREATE DATABASE $DRILL"
"${COMPOSE[@]}" exec -T postgres pg_restore -U "$POSTGRES_USER" -d "$DRILL" --no-owner < "$FILE"
"${COMPOSE[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$DRILL" -c \
  "SELECT (SELECT count(*) FROM companies) AS companies,
          (SELECT count(*) FROM supplier_submissions) AS submissions,
          (SELECT max(created_at) FROM audit_logs) AS last_audit_entry"
"${COMPOSE[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d postgres -c "DROP DATABASE $DRILL"
echo "restore drill passed. Record the date in docs/runbooks/operations.md"

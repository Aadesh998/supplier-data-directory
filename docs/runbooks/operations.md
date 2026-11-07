# Operations runbook

`DC` below means `docker compose -f infra/deploy/compose.prod.yaml --env-file infra/deploy/.env` (production) or plain `docker compose` (local).

## Daily / automatic

| Job | How | What it does |
|---|---|---|
| Maintenance | Inside `api serve`, every 24 h (also `DC exec api /api maintenance`) | Marks overdue verifications `stale`; unpublishes companies whose permissions lapsed (source revoked or retention expired); purges old sessions; deletes spam (30 d) and rejected (180 d) submissions that never became companies |
| Backup | cron → `scripts/backup.sh` | `pg_dump` (custom format), verifies archive, optional off-site copy, prunes after `BACKUP_KEEP_DAYS` |

## Weekly

- Review Admin → **Submissions** (pending), **Privacy requests** (aim to respond within 7 days), **Duplicates**.
- Dashboard "due for review" count: re-verify those companies.
- Check `/var/log/bharatmfg-backup.log` for `backup ok` lines.

## Monthly

- Restore drill: `scripts/restore.sh $(ls -t /var/backups/bharatmfg/*.dump | head -1)` and record the date here.
- Update images and dependencies: `git pull`, rebuild, run the test suite locally first.
- Review admin users (`/api/v1/admin/users`) and remove anyone who no longer needs access.

| Restore drill date | Dump file | Result | By |
|---|---|---|---|
| | | | |

## Monitoring (lightweight)

- Uptime: an external checker (e.g. UptimeRobot or Better Stack free tier) on `https://<domain>/healthz` (web proxy → API) every 5 minutes, with alerts to email.
- Logs: `DC logs -f api` (JSON, one line per request with `request_id`, status and duration; no bodies or query strings). Errors are logged at `level=ERROR` with the request id shown to users in error responses.
- Disk: `df -h` and `docker system df`. Alert if free space falls below 20%.
- Optional: forward logs with a lightweight agent (Vector or Promtail) to a hosted log service if volume grows.

## Common tasks

```bash
DC exec api /api migrate status
DC exec -e ADMIN_PASSWORD='…' api /api create-admin --email reviewer@… --role reviewer
DC exec postgres psql -U "$POSTGRES_USER" "$POSTGRES_DB"     # careful: production data
```

## Incidents

| Situation | Immediate action |
|---|---|
| Suspected data exposure | Unpublish the affected companies (Admin → company → Unpublish), or revoke the source's public permission (owner) to unpublish everything it backs. Preserve logs. Note the time. Assess DPDP breach-notification duties with counsel. |
| Admin account compromise | Deactivate the user (`UPDATE admin_users SET active=false WHERE email=…`): sessions stop working immediately because lookups require `active`. Rotate passwords. Review `audit_logs` for that `actor_id`. |
| Encryption key leaked | Treat it as a data breach. Key rotation needs a re-encryption script (not yet written): decrypt with the old key, encrypt with the new one, in one transaction per table. |
| Spam wave | Lower the submission rate limit in `internal/api/server.go` or add Cloudflare Turnstile. Bulk-mark spam in the admin console. |
| Site down | `DC ps`; `DC logs --tail=200 api web caddy`; check `/readyz` (database reachability) and free disk space. |

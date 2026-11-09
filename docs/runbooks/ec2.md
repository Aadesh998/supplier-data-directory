# Deploying: everything on EC2

```
browser ──► nginx peoplvance.org (EC2: santimentos dev box, 3.225.67.242)
              ├─ /api/* ──────────► peoplvance-api          127.0.0.1:8090  ┐ one Go binary,
              └─ everything else ─► peoplvance-web (Next.js) 127.0.0.1:3100  │ /home/ubuntu/peoplvance
            peoplvance-gmaps-worker ──► scraper 127.0.0.1:8080 (google-scrape)┘
            peoplvance-maintenance.timer (daily 03:17)
            API, worker, maintenance ──► PostgreSQL (DATABASE_URL, schema peoplvance_dir)
```

Why: see [ADR 0010](../adr/0010-everything-on-ec2.md). Files: `infra/ec2/`.

## First time

1. **DNS:** point `peoplvance.org` and `www.peoplvance.org` to `3.225.67.242` (A records).
2. **Env file:** `cp infra/ec2/env.example .env.production` and fill it in. Use the **same `DATA_ENCRYPTION_KEY`** as today. `.env.production` is git-ignored. One file serves the API, the worker and the website.
3. From the repo root:
   ```bash
   make ec2-setup        # systemd units, nginx site, log rotation, maintenance timer
   make ec2-env          # uploads .env.production to /home/ubuntu/peoplvance/.env
   make ec2-deploy-all   # API: build → scp → migrate → restart; website: build → scp → restart
   ```
4. **HTTPS:** `make ec2-ssh`, then `sudo certbot --nginx -d peoplvance.org -d www.peoplvance.org`.
5. **Admin account** (once), on the box:
   ```bash
   ADMIN_PASSWORD='…' ~/peoplvance/run.sh create-admin --email you@peoplvance.org --role owner
   ```
6. **Jobs:** Admin → Discovery jobs → upload `scripts/jobs/jobs.csv`. The gmaps worker picks them up within 5 minutes.

## Releases

| Change | Command | Manual alternative |
|---|---|---|
| API / worker | `make ec2-deploy` | `make ec2-scp`, then on the box `~/peoplvance/deploy.sh` |
| Website | `make ec2-web-deploy` | `make ec2-web-scp`, then on the box `~/peoplvance/deploy-web.sh` |
| Both | `make ec2-deploy-all` | |
| Env | edit `.env.production`, `make ec2-env`, `make ec2-restart` | |

`deploy.sh` migrates the database with the new binary **before** switching, so a failed migration leaves the old version running.

Check and fix: `make ec2-status`, `make ec2-logs`, `make ec2-rollback` (API), `make ec2-web-rollback` (website). Migrations are never rolled back.

## Memory

The box has about 900 MB RAM, shared with the scraper and santimentos. Limits: API 300 MB, worker 200 MB, website 350 MB (Node heap 256 MB; about 135 MB in use when idle). Watch `free -m` after the first deploy.

## Backups
Use your Postgres provider's snapshots. For an independent copy, run `pg_dump "$DATABASE_URL" --format=custom` from a trusted machine on a schedule (`scripts/backup.sh`).

# Deploying: API on EC2, website on Cloudflare Workers

```
browser ──► Cloudflare ──► peoplvance-web Worker (Next.js via OpenNext, apps/web/wrangler.jsonc)
                               └─ /api/* and server-side page data ──► https://api.peoplvance.org
EC2 (santimentos dev box, 3.225.67.242)
  nginx api.peoplvance.org ──► peoplvance-api (127.0.0.1:8090)         ┐ one binary,
  peoplvance-gmaps-worker ──► scraper (127.0.0.1:8080, google-scrape)  ┘ /home/ubuntu/peoplvance
  peoplvance-maintenance.timer (daily 03:17)
  all three ──► PostgreSQL (DATABASE_URL, schema peoplvance_dir)
```

Why this split: see [ADR 0010](../adr/0010-ec2-api-cloudflare-workers-web.md).

## A. API + worker on EC2 (first time)

1. **DNS:** add `api.peoplvance.org` → `3.225.67.242` (A record, **DNS only / grey cloud**, so certbot can verify it).
2. **Env file:** `cp infra/ec2/env.example .env.production` and fill it in. Use the **same `DATA_ENCRYPTION_KEY`** as today. `.env.production` is git-ignored.
3. From the repo root:
   ```bash
   make ec2-setup     # systemd units, nginx site, log rotation, maintenance timer
   make ec2-env       # uploads .env.production to /home/ubuntu/peoplvance/.env
   make ec2-deploy    # build → scp → migrate → restart → /readyz check
   ```
4. **HTTPS:** `make ec2-ssh`, then `sudo certbot --nginx -d api.peoplvance.org`.
5. **Admin account** (once): on the box,
   ```bash
   cd ~/peoplvance && set -a && . ./.env && set +a && ADMIN_PASSWORD='…' ./peoplvance-api create-admin --email you@peoplvance.org --role owner
   ```

### Every API release
```bash
make ec2-deploy        # or by hand: make ec2-scp, then on the box ~/peoplvance/deploy.sh
make ec2-status        # services + readiness
make ec2-logs          # follow api.log and gmaps-worker.log
make ec2-rollback      # previous binary (migrations are not rolled back)
```
Env change: edit `.env.production`, `make ec2-env`, `make ec2-restart`.

## B. Website on Cloudflare Workers

Needs the Workers **Free** plan or above (Paid if traffic grows past the free request limit), and `peoplvance.org` in your Cloudflare account. No Docker.

1. `cd apps/web && npx wrangler login`
2. Check `vars` in `apps/web/wrangler.jsonc` (`API_INTERNAL_URL`, site URL, emails, `NEXT_PUBLIC_ALLOW_INDEXING`).
3. `npx wrangler secret put INTERNAL_API_TOKEN` (the same value as in `.env.production`).
4. From the repo root: `make web-deploy`.
5. Check: `curl -sI https://peoplvance.org/ | head -1` and `curl -fsS https://peoplvance.org/api/v1/states | head -c 100`.

Try it locally first with `make web-preview` (runs the real Workers runtime on http://localhost:8787; put `API_INTERNAL_URL=…` in `apps/web/.dev.vars`).

## C. Discovery jobs
Admin → Discovery jobs → upload `scripts/jobs/jobs.csv`. The gmaps worker picks them up within `GMAPS_IDLE_SECONDS` (5 min). Watch with `make ec2-logs`.

## Backups
Use your Postgres provider's snapshots. For an independent copy, run `pg_dump "$DATABASE_URL" --format=custom` from a trusted machine on a schedule (`scripts/backup.sh`).

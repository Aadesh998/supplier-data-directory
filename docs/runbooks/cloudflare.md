# Deploying on Cloudflare (Workers + Containers)

Everything in `infra/cloudflare/` deploys as **one Worker** in front of **two containers**:

```
browser ──► Cloudflare edge ──► Worker (infra/cloudflare/src/index.ts)
                                  ├─ /api/*, /healthz, /readyz ─► ApiContainer  (Go API, infra/api.Dockerfile)
                                  ├─ everything else ──────────► WebContainer  (Next.js, infra/web.Dockerfile)
                                  ├─ cron */5 * * * * ─────────► discovery cycle (claims 4 jobs → Overpass / Places IDs → API)
                                  └─ cron 17 3 * * * ──────────► API maintenance (re-verification, retention)
                                                    ApiContainer ──TCP──► external PostgreSQL (Neon, Supabase, RDS, your VM…)
```

The Worker replaces any client-supplied `X-Forwarded-For` with `CF-Connecting-IP`, so the API's per-IP limits can't be dodged with spoofed headers. The web container's server-side calls go back through the Worker with `INTERNAL_API_TOKEN`, so normal page traffic isn't throttled as one IP.

## Requirements and limits (checked against Cloudflare docs, Nov 2026)

- **Workers Paid plan** (Containers are only available there). Check current Containers pricing before deploying.
- **Docker running locally.** `wrangler deploy` builds both images (`linux/amd64`) and pushes them to Cloudflare's registry.
- **Disk is ephemeral.** The database must be external PostgreSQL 16+ with `pg_trgm`, `unaccent` and `citext`. Neon and Supabase support these.
- **Cold starts** are typically 1–3 s after a container sleeps (`sleepAfter = "15m"`).
- **Instances:** both containers use the `basic` instance type (¼ vCPU, 1 GiB) and one named instance each. The API's in-memory rate limiter and in-process maintenance assume a single instance.
- ⚠️ **Verify first:** containers have outbound internet by default, but the docs don't explicitly confirm raw TCP to an external PostgreSQL port. Step 4 below checks it before anything else. If it fails, use a Postgres provider/port reachable over the allowed path, or run the API on a VM (see `deployment.md`) and keep only the web and cron on Cloudflare.

## 1. Database

Create a PostgreSQL database (e.g. Neon, Mumbai region `ap-south-1` for India latency) and copy its connection string with `sslmode=require`. The API runs migrations automatically on start.

## 2. Install and log in

```bash
cd infra/cloudflare
npm install
npx wrangler login
```

## 3. Configure

Edit `wrangler.jsonc` → `vars`:
- `PUBLIC_SITE_URL`: your custom domain, or `https://bharatmfg.<your-subdomain>.workers.dev` for the first deploy.
- `PRIVACY_EMAIL`, `MAIL_FROM`.
- `DISCOVERY_ENABLED`: leave `"false"` until you have uploaded jobs and checked one manual run.

Set secrets (prompted, never stored in git):

```bash
npx wrangler secret put DATABASE_URL          # postgres://…?sslmode=require
npx wrangler secret put DATA_ENCRYPTION_KEY   # openssl rand -base64 32   (store it in your password manager too!)
npx wrangler secret put INTERNAL_API_TOKEN    # openssl rand -hex 32
npx wrangler secret put WORKER_TOKEN          # openssl rand -hex 32
# optional
npx wrangler secret put SMTP_HOST ; npx wrangler secret put SMTP_PORT
npx wrangler secret put SMTP_USERNAME ; npx wrangler secret put SMTP_PASSWORD
npx wrangler secret put ADMIN_NOTIFY_EMAIL
npx wrangler secret put GOOGLE_PLACES_API_KEY # only for google_places_ids jobs (IDs-only field mask)
```

## 4. Deploy and verify

```bash
npm run typecheck && npm test
npx wrangler deploy
curl -fsS https://<your-site>/healthz        # API container up
curl -fsS https://<your-site>/readyz         # API reached PostgreSQL ← the TCP check
curl -sI https://<your-site>/ | head -1      # web container up
npx wrangler tail                             # live logs (Worker + cron summaries)
```

Create your owner account. Containers have no shell access, so run the one-off command from your machine against the same database:

```bash
DATABASE_URL='postgres://…?sslmode=require' DATA_ENCRYPTION_KEY='…' APP_ENV=production COOKIE_SECURE=true \
  ADMIN_PASSWORD='…' go run ./apps/api create-admin --email you@yourdomain --role owner
```

(`set-password --email …` works the same way if you ever get locked out.)

## 5. Custom domain

Cloudflare dashboard → Workers & Pages → `bharatmfg` → Settings → Domains & Routes → add your domain. Then set `PUBLIC_SITE_URL` to it and `npx wrangler deploy` again.

## 6. Discovery jobs

1. Admin → **Discovery jobs** → upload `scripts/jobs/jobs.csv` (14,419 jobs; re-upload any time, rows match by `job_id`).
2. Set `DISCOVERY_ENABLED` to `"true"` in `wrangler.jsonc` and deploy. Every 5 minutes the cron claims `DISCOVERY_JOBS_PER_RUN` (default 4) jobs, runs them one after another, and reports results.
3. Watch Admin → **Discovery jobs** (status counts, errors) and **Discovery results** (review, promote, mark contacted).

Throughput at 4 jobs / 5 min: priority-1 clusters in ~5.5 h, a full sweep in ~12.5 days. Completed jobs repeat after `refresh_interval_days` (default 30).

**Overpass fair use:** the public `overpass-api.de` instance is shared and often slow. Keep `DISCOVERY_JOBS_PER_RUN` low. Failed jobs retry with backoff (5 min × 2ⁿ, max 3 attempts, then `failed` → "Retry failed" in the admin). For heavier use, point `OVERPASS_URL` at another public instance or your own.

## Updating

```bash
git pull && cd infra/cloudflare && npm install && npx wrangler deploy
```

Migrations run on API start. Take a database snapshot with your provider before deploying a release that changes `db/migrations/`.

## Backups

Use your Postgres provider's point-in-time recovery or snapshots. For an independent copy, run `pg_dump "$DATABASE_URL" --format=custom` from a trusted machine on a schedule (see `scripts/backup.sh` for the verification pattern).

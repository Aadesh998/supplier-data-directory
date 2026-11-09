# ADR 0010: API on EC2, website on Cloudflare Workers

**Status:** accepted · 2026-11-09 · supersedes [ADR 0009](0009-cloudflare-containers.md)

## Context
The Google Maps worker must run for hours next to the scraper, and Cloudflare Containers sleep after 15 minutes. The scraper already runs on an EC2 box (the santimentos dev server, `leads.peoplvance.org`). Containers also cost more and need Docker for every deploy.

## Decision
- **API + gmaps worker on EC2.** One static Go binary (`make build-linux`), copied with scp and run by systemd as two services: `peoplvance-api` (`serve`, on 127.0.0.1:8090 behind nginx at `api.peoplvance.org`) and `peoplvance-gmaps-worker`. A systemd timer runs `maintenance` daily. Files: `infra/ec2/`. Migrations run in `deploy.sh` with the new binary before the switch.
- **Website on Cloudflare Workers** with OpenNext (`@opennextjs/cloudflare`), not containers. Pages still render on request, so removed profiles disappear immediately (410) and SEO metadata stays live. `/api/*` on the site is proxied to the EC2 API, keeping cookies same-origin.
- The website sends the visitor IP (`CF-Connecting-IP`) as `X-Forwarded-For`. nginx trusts that header only from Cloudflare's IP ranges (`real_ip`).

## Consequences
- The EC2 box is small (1 GB, shared with the scraper and santimentos), so the services have `MemoryMax` limits.
- Next.js 16 `proxy.ts` runs as Node middleware, which OpenNext marks experimental on Cloudflare. It was tested in `wrangler` preview (410 page works). If it breaks after an upgrade, the supplier page already falls back to a noindex "removed" page with status 200.
- The OSM discovery cron from ADR 0009 is gone. OSM jobs can still be run through the worker API with `WORKER_TOKEN`.

# ADR 0010: Everything on one EC2 box

**Status:** accepted · 2026-11-09 · supersedes [ADR 0009](0009-cloudflare-containers.md)

## Context
The Google Maps worker must run for hours next to the scraper, and Cloudflare Containers sleep after 15 minutes. The scraper already runs on an EC2 box (the santimentos dev server). Splitting the website onto Cloudflare added a second platform, a second deploy path and IP-forwarding rules, without a real need at current traffic.

## Decision
- **One box, no Docker for the app.** nginx serves `peoplvance.org`: `/api/*` goes to the Go API, everything else to the Next.js website.
- **API + gmaps worker:** one static Go binary (`make build-linux`), copied with scp, run by systemd as `peoplvance-api` (`serve`, 127.0.0.1:8090) and `peoplvance-gmaps-worker`. A systemd timer runs `maintenance` daily. `deploy.sh` migrates with the new binary before switching.
- **Website:** the Next.js standalone build (`make build-web`, a tarball), run by systemd as `peoplvance-web` with Node on 127.0.0.1:3100. Pages still render on request, so removed profiles return 410 immediately.
- nginx overwrites `X-Forwarded-For` with the client address; the API trusts it (`TRUST_PROXY_HEADERS=true`) because it only listens on localhost.

## Consequences
- Single point of failure, and a small box (1 GB) shared with the scraper and santimentos. Every service has a `MemoryMax`.
- Scaling later means moving the website or the database off this box. The Docker Compose setup (`infra/deploy`) stays as an alternative.
- The Cloudflare Worker, its OSM discovery cron and the OpenNext build were removed. OSM jobs can still be run through the worker API with `WORKER_TOKEN`.

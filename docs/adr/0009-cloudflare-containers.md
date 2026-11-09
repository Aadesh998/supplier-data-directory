# ADR 0009: Cloudflare Workers + Containers as a deployment target

**Status:** superseded by [ADR 0010](0010-ec2-api-cloudflare-workers-web.md) · 2026-11-09 (was accepted 2026-11-08)

## Context
The founder wants to run the platform, including the backend and the scheduled discovery jobs, on Cloudflare rather than a VM. The Go API and Next.js app are already container images. Cloudflare Workers cannot run a Go binary or a Node server directly, but Cloudflare Containers can run Docker images behind a Worker.

## Decision
- One Worker (`infra/cloudflare`) routes `/api/*` and health checks to an `ApiContainer` and everything else to a `WebContainer`. Both use the existing Dockerfiles, so there is no separate code path.
- The Worker overwrites `X-Forwarded-For` with `CF-Connecting-IP`, so the API can trust it.
- Server-side page renders call back through the Worker with a shared `INTERNAL_API_TOKEN` that exempts them from per-IP limits. This also fixes the same issue in the Compose/Caddy deployment.
- Cron Triggers replace the in-process timer: every 5 minutes for the discovery cycle, daily for maintenance. The discovery runner is plain TypeScript (`fetch` to Overpass / Places), talking to the API's token-protected worker endpoints over the container binding.
- PostgreSQL stays external (container disk is ephemeral).
- One named instance per container, because the API's rate limiter is in-memory.

## Consequences
- The VPS deployment remains fully supported. Both targets build from the same images.
- Cold starts of 1–3 s after idle. Workers Paid plan required.
- Open item: raw TCP from a container to an external Postgres port isn't explicitly documented. The runbook verifies it via `/readyz` immediately after the first deploy and lists fallbacks.
- Scaling beyond one API instance needs a shared rate-limit store (Durable Object or Postgres-based).

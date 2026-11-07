# ADR 0003: Next.js App Router for the public site and admin console

**Status:** accepted · 2026-11-07

## Context
Public pages need server rendering for SEO. The admin console needs rich client interactions. We want one frontend codebase.

## Decision
Next.js 16 (App Router, TypeScript strict, Tailwind 4):
- Public pages are server components that fetch the API with no caching, so unpublished data disappears immediately.
- Admin pages are client components using TanStack Query.
- A runtime route handler proxies `/api/*` to the Go API. The browser sees one origin, so the session cookie is first-party. `API_INTERNAL_URL` is read at runtime, so one Docker image works everywhere. Build-time rewrites would freeze the URL into the image.

## Consequences
- The build never contacts the API, and no data is baked into static output.
- The proxy forwards `X-Forwarded-For` as received. Production must sit behind a proxy that sets it (Caddy, see ADR 0004 and the deployment runbook).

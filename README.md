# Bharat Manufacturing Directory

A database of Indian manufacturers and suppliers, plus a public website and admin console, built so that **every field knows where it came from and what you are allowed to do with it**. It is for a solo founder: one Go API, one Next.js app, one PostgreSQL database.

> Status: MVP / private beta. Nothing is published unless a person approves it **and** the data's source allows public display. The paid external API is deliberately not switched on yet (see [ADR 0007](docs/adr/0007-defer-external-api-billing.md)).

## What it does

| Area | What you get |
|---|---|
| **Data model** | Companies, facilities, products, capabilities, certifications and contacts. Uses an arbitrary-depth category tree (14 sectors, ~90 categories) and 15 business roles. Provenance is tracked per field. |
| **Imports** | Upload a CSV (e.g. your existing 1,200 records). Declare where the data came from and its reuse rights, map columns, review normalisation and duplicate candidates, dry-run, commit, roll back, download a validation report. |
| **Supplier self-registration** | A 6-step mobile form with three separate, unticked permissions: privacy acknowledgement, public listing, and future commercial/API use. Has a honeypot, a timing check, rate limits and idempotency. |
| **Admin console** | Review submissions, edit companies, verify, publish or unpublish, merge duplicates, manage taxonomy and sources, handle privacy requests, run permission-filtered exports, read the audit log. |
| **Public site** | Industry, location and supplier pages, plus search. SEO metadata, sitemap and robots are built only from approved, permitted data. The site stays `noindex` until you switch indexing on. |
| **Safety rails in the database** | Triggers and constraints refuse to publish without public-display permission. Personal contacts can never be public. Permissions default to `false`. The audit log is append-only. |
| **Discovery** | A Google Places adapter that is off by default. Results are transient; only place IDs may be stored as leads. |

## Run it locally

Prerequisites: Docker with Compose v2. For development outside Docker you also need Go 1.27+ and Node 22+.

```bash
cp .env.example .env              # defaults work for local use; change API_PORT if 8080 is taken
docker compose up -d --build      # postgres, mailpit, api (runs migrations), web
docker compose exec api /api seed                                    # 20 clearly-synthetic private records
docker compose exec -e ADMIN_PASSWORD='choose-a-long-password' api \
  /api create-admin --email you@example.com --role owner             # your owner login
```

Then open:

- **http://localhost:3000**: public site
- **http://localhost:3000/register-supplier**: supplier form
- **http://localhost:3000/login**: admin console
- **http://localhost:8025**: Mailpit (every email the app sends lands here)

Optional: `docker compose exec api /api seed --demo` publishes the synthetic records so you can preview public pages. It is refused in production, and the pages carry a "Synthetic demo record" banner.

### Importing your 1,200 records

1. Admin → **Sources**: create a source describing where the file came from. Tick only the rights you can evidence. If you are unsure, leave everything unticked: rows will stage but cannot be committed.
2. Admin → **Imports** → upload the CSV (UTF-8; export from Excel as "CSV UTF-8").
3. Check the column mapping, review rows with errors or duplicate candidates, then **Dry run** → **Commit**.
4. Imported companies are `approved_private`. To publish one, its data must be backed by a source that allows public display.

## Developing

```bash
make db-up        # PostgreSQL only (localhost:5434)
make run          # API on :8080 (reads .env)
make web          # Next.js dev server on :3000 (cd apps/web && npm install first)
make test         # Go tests (needs db-up) + web typecheck/lint/unit tests
make test-perf    # 100,000-record performance test
make e2e          # Playwright against a running stack (see apps/web/.env.example)
make help         # everything else
```

| Command | Purpose |
|---|---|
| `go run ./apps/api migrate up\|down\|reset\|status` | Schema migrations (Goose, embedded in the binary) |
| `go run ./apps/api maintenance` | Re-verification, permission-expiry and retention jobs (also runs daily inside `serve`) |
| `go run ./apps/api create-admin --email E --role owner\|admin\|reviewer` | Create an admin user |

## Repository layout

```
apps/api/            Go entrypoint (serve, migrate, seed, create-admin, maintenance, healthcheck)
apps/web/            Next.js App Router site + admin console (TypeScript, Tailwind)
internal/            Go packages: api (HTTP), store (SQL + domain operations), importer, normalize,
                     secure (encryption, Argon2id), httpx, notify (emails), discovery, config
db/migrations/       PostgreSQL schema (Goose)
db/seeds/            Synthetic seed data (private) + dev-only demo publish
packages/openapi/    OpenAPI 3.1 contract
infra/               Dockerfiles, local Postgres init, production compose + Caddy
scripts/             backup.sh / restore.sh, jobs/jobs.csv (discovery job list)
docs/                Architecture, ERD, data dictionary, policies, ADRs, runbooks
```

## Documentation

- [Architecture](docs/architecture.md): components, request flow, how permissions gate everything
- [ERD](docs/erd.md) and [data dictionary](docs/data-dictionary.md)
- [How the schema fits rugs, namkeen, tiles, bricks, CNC and sourcing houses](docs/taxonomy-examples.md)
- [Data source policy](docs/data-source-policy.md): what may and may not be collected, and the Google Places boundary
- [Privacy and security](docs/privacy-and-security.md): threat model, controls, retention schedule
- [API examples](docs/api-examples.md): curl walkthrough
- [Supplier-facing copy](docs/supplier-copy.md): registration text and email templates
- [Deployment](docs/runbooks/deployment.md), [operations](docs/runbooks/operations.md), [launch checklist](docs/launch-checklist.md)
- [Architecture decision records](docs/adr/)

## Testing at a glance

- **Go:** migrations up/down/up, schema constraints (publication gate, contact protection, permission defaults, append-only audit), normalisation, importer, encryption and passwords, and full HTTP integration tests. The integration tests cover auth, CSRF, RBAC, lockout, submission → approval → publication, publish denial, import/dedupe/commit/rollback, permission-filtered exports, merges with provenance, consent withdrawal and erasure, and source revocation.
- **Web:** Vitest component tests (registration form, search filters, submission review) and Playwright end-to-end flows against the real stack.
- **Performance:** 100k companies. Public search ~170 ms, filtered search ~400 ms, profile ~7 ms, admin search ~15 ms on a laptop.

## Important

This software helps you keep records honest; it does not make your data collection lawful. Before launch, get qualified Indian legal advice on the privacy notice, supplier permissions, India's DPDP Act obligations and any data licensing. See the [launch checklist](docs/launch-checklist.md).

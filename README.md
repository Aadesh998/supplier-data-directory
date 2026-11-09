# Peoplvance

**peoplvance.org**: a database of Indian manufacturers and suppliers, plus a public website and admin console, built so that **every field knows where it came from and what you are allowed to do with it**. It is for a solo founder: one Go API, one Next.js app, one PostgreSQL database.

> Status: MVP / private beta. Nothing is published unless a person approves it **and** the data's source allows public display. The paid external API is deliberately not switched on yet (see [ADR 0007](docs/adr/0007-defer-external-api-billing.md)).

## What it does

| Area | What you get |
|---|---|
| **Data model** | Companies, facilities, products, capabilities, certifications and contacts. Uses an arbitrary-depth category tree (14 sectors, ~90 categories) and 15 business roles. Provenance is tracked per field. |
| **Imports** | Upload a CSV (e.g. your existing 1,200 records). Declare where the data came from and its reuse rights, map columns, review normalisation and duplicate candidates, dry-run, commit, roll back, download a validation report. |
| **Supplier self-registration** | A 6-step mobile form with three separate, unticked permissions: privacy acknowledgement, public listing, and future commercial/API use. Has a honeypot, a timing check, rate limits and idempotency. |
| **Admin console** | Review submissions, edit companies, verify, publish or unpublish, merge duplicates, manage taxonomy and sources, handle privacy requests, run permission-filtered exports, read the audit log. |
| **Public site** | Content-rich landing page, plus industry, location and supplier pages and search. SEO metadata, Open Graph image, sitemap and robots are built only from approved, permitted data. The site stays `noindex` until you switch indexing on. See the [keyword plan](docs/seo/keyword-research.md). |
| **Teaser profiles** | Public supplier pages show what a business makes and where it is based. Contacts, website, legal name, exact locality, certificate issuers and the full product list are withheld by the API (never sent to the browser). Visitors see locked placeholders and a "Request full profile" link. |
| **Users** | Admin → Users (owner-only): create users, change roles, deactivate, reset passwords. Or use `scripts/create-user.sh`. |
| **Safety rails in the database** | Triggers and constraints refuse to publish without public-display permission. Personal contacts can never be public. Permissions default to `false`. The audit log is append-only. |
| **Discovery** | A Google Places lookup that is off by default. Results are transient; only place IDs may be stored as leads. |
| **Discovery job queue** | Upload a jobs CSV (e.g. `scripts/jobs/jobs.csv`: 14,419 sector × location searches across India) whenever you like. Workers claim jobs on a schedule (default 4 every 5 minutes) with leases, retries and backoff. Finished jobs repeat after a refresh interval, so results stay current. Executors: OpenStreetMap (Overpass) and Google Places ID-only. The database refuses to store Google content beyond place IDs. Results are leads for review; nothing is published automatically. |
| **Deployment targets** | Docker Compose locally or on a VPS with Caddy, or the default: **everything on one EC2 box** (Go API + Google Maps worker as one binary, Next.js website, nginx, systemd; `make ec2-deploy-all`). |

## Run it locally

Prerequisites: Docker with Compose v2. For development outside Docker you also need Go 1.27+ and Node 22+.

```bash
cp .env.example .env              # defaults work for local use; change API_PORT if 8080 is taken
docker compose up -d --build      # postgres, mailpit, api (runs migrations), web
docker compose exec api /api seed                                    # 20 clearly-synthetic private records
scripts/create-user.sh --email you@example.com --role owner          # your owner login (prompts for a 12+ char password)
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

### Using a hosted / shared PostgreSQL (e.g. Railway)

Put the connection string in `DATABASE_URL` and set `DATABASE_SCHEMA=peoplvance_dir` (any lowercase name). Every table, and the migration history, then lives in that schema, so other applications in the same database are untouched, and `migrate reset` is refused. To run the Docker API container against it too, set `API_DATABASE_URL=${DATABASE_URL}`. The local Postgres container and the test suite use `LOCAL_PG_*` and never touch `DATABASE_URL`.

### Bulk-importing your supplier list

Admin → **Imports** → upload the CSV (e.g. `suppliers_import.csv`, which is git-ignored because it contains phone numbers). Pick or create a data source with internal use allowed, check the column mapping (GSTIN, phone, categories and so on map automatically), then run **Preview (dry run)** → **Commit import**. Row checking, the preview and the commit all run in the background on the server, so large files aren't cut off by request timeouts. The page shows progress, and you can leave it and come back. Records are created private, phones are encrypted, and GSTINs are used for duplicate checks. Roll back from the same page. The CLI equivalent is `api import-csv --file F --as ADMIN_EMAIL --source-name NAME --commit`.

### Running discovery jobs

1. Admin → **Discovery jobs** → upload `scripts/jobs/jobs.csv`. Re-upload any time: rows match by `job_id`, and run history is kept unless you tick "reset".
2. A worker processes the queue. `gmaps_scraper` jobs are run by the `gmaps-worker` service on EC2, one at a time (see [EC2 runbook](docs/runbooks/ec2.md)). Any other process can use the worker API with `WORKER_TOKEN`: `POST /api/v1/worker/jobs/claim` → run → `…/complete` or `…/fail` (see `packages/openapi/openapi.yaml`).
3. Review leads in Admin → **Discovery results**. Promote OpenStreetMap results to private draft companies (internal-only source; duplicates are flagged), or mark them contacted after inviting the business to register.

Expect thin coverage from OpenStreetMap; it is a lead source, not a complete register. On public Overpass servers, expect slow responses. Timeouts are retried automatically.

## Developing

```bash
make db-up        # PostgreSQL only (localhost:5434)
make run          # API on :8080 (reads .env)
make web          # Next.js dev server on :3000 (cd apps/web && npm install first)
make test         # Go tests (needs db-up) + web typecheck/lint/unit tests
make test-perf    # 100,000-record performance test
make e2e          # Playwright against a running stack (see apps/web/.env.example). The submission rate
                  # limit (5/hour per IP) means back-to-back runs need `docker compose restart api` in between.
make help         # everything else
```

| Command | Purpose |
|---|---|
| `go run ./apps/api migrate up\|down\|reset\|status` | Schema migrations (Goose, embedded in the binary) |
| `go run ./apps/api maintenance` | Re-verification, permission-expiry and retention jobs (also runs daily inside `serve`) |
| `go run ./apps/api create-admin --email E --role owner\|admin\|reviewer` | Create an admin user |
| `go run ./apps/api import-csv --file F --as ADMIN_EMAIL --source-name NAME [--commit]` | Stage (and optionally commit) a CSV through the importer, the same as Admin → Imports |
| `scripts/create-user.sh --email E [--role R] [--reset]` | Create an admin user or reset a password (Docker or host) |
| `go run ./apps/api set-password --email E` | Reset a forgotten admin password (revokes that admin's sessions). With Docker: `docker compose exec -e ADMIN_PASSWORD='…' api /api set-password --email E` |

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
infra/ec2/          systemd units, nginx site and deploy scripts for the EC2 box (API, gmaps worker, website)
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
- [Deployment on a VPS](docs/runbooks/deployment.md), [deployment on EC2](docs/runbooks/ec2.md), [operations](docs/runbooks/operations.md), [launch checklist](docs/launch-checklist.md)
- [Architecture decision records](docs/adr/)

## Testing at a glance

- **Go:** migrations up/down/up, schema constraints (publication gate, contact protection, permission defaults, append-only audit), normalisation, importer, encryption and passwords, and full HTTP integration tests. The integration tests cover auth, CSRF, RBAC, lockout, submission → approval → publication, publish denial, import/dedupe/commit/rollback, permission-filtered exports, merges with provenance, consent withdrawal and erasure, and source revocation.
- **Web:** Vitest component tests (registration form, search filters, submission review) and Playwright end-to-end flows against the real stack.
- **Performance:** 100k companies. Public search ~170 ms, filtered search ~400 ms, profile ~7 ms, admin search ~15 ms on a laptop.

## Important

This software helps you keep records honest; it does not make your data collection lawful. Before launch, get qualified Indian legal advice on the privacy notice, supplier permissions, India's DPDP Act obligations and any data licensing. See the [launch checklist](docs/launch-checklist.md).

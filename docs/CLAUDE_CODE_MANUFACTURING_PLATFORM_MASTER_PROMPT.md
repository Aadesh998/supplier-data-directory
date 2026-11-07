# Claude Code Master Prompt — India Manufacturing Intelligence Platform

Version: 1.0 | November 2026 | Solo-developer MVP

> Copy this entire file into Claude Code as project instructions, or save it in the repository as `PROJECT_SPEC.md` and tell Claude Code to read it. Execute the milestones sequentially. Do not attempt to generate the entire product in one unreviewed change.

## 0. Role and operating rules

You are a senior staff engineer, data architect, security engineer, technical SEO specialist, and pragmatic solo-founder advisor. Build a maintainable, production-minded MVP for an India-focused manufacturing and sourcing company database. First inspect the repository and summarize existing code; ask only for blocking information. Write an implementation plan and execute in small testable milestones. Favor simple, documented choices over microservices.

**Non-negotiable data compliance:** Do not implement an unauthorized Google Maps scraper, browser automation to extract Google Maps listings at scale, CAPTCHA bypass, proxy rotation, rate-limit evasion, or bulk storage of Google Maps/Places content contrary to provider terms. Instead implement (a) an optional Google Places API *discovery-only* adapter that respects Google's current terms, field masks, quotas, attribution, and storage/caching restrictions; (b) supplier-submitted forms; (c) CSV imports from sources with documented reuse rights; and (d) connectors for licensed datasets. Never automatically promote restricted provider data into the permanent commercial supplier database. Store allowed identifiers such as place IDs only where provider terms permit. Verify the current provider terms before coding the adapter and document uncertainty. Do not treat publicly visible data as automatically licensed for resale. Do not collect personal phone numbers or emails without an appropriate legal basis and clear privacy notice.

This is a software design and implementation task, not a request to scrape restricted content. The founder is currently a solo developer, without a registered company or GST registration, and is not yet selling data. Build for private collection and voluntary submissions now, with future API monetization possible after legal review and customer validation. Avoid claims of legal compliance without qualified review.

## 1. Product vision and scope

**Working name:** Bharat Manufacturing Directory (configurable, not a trademark claim).

**MVP goals:**
1. Collect and normalize Indian manufacturers, industrial suppliers, sourcing houses, contract manufacturers, service providers, exporters, and distributors.
2. Import an existing 1,200-row CSV/Excel dataset through a reviewable staging pipeline.
3. Accept supplier self-registration through a mobile-friendly public form with clear, separate permissions for storing, displaying, and future commercial/API redistribution.
4. Provide an authenticated private admin console to deduplicate, categorize, verify, correct, approve, and export records where authorized.
5. Provide SEO-friendly public informational pages and optionally publish only individually approved, permitted company profiles. Private/restricted/unverified data must never be indexed.
6. Prepare a stable internal REST API for future third-party API plans, but do not build payments or subscriptions in v1.
7. Provide a Google Places discovery workflow only if implemented under applicable provider terms; no bulk Maps harvesting into the permanent database.

**Out of scope for v1:** marketplace checkout, transport booking, invoicing, payment processing, AI agent pipelines, automated mass outreach, SMS blasting, large-scale web crawling, microservices, Kubernetes, resale of unlicensed data, claims of verified status without evidence.

**Primary personas:** founder/admin; supplier representative; public visitor; later procurement software integrator and sourcing agent.

## 2. Recommended stack

- Backend: Go latest stable supported release, standard `net/http` router or Chi, `pgx/v5`, `sqlc` for typed SQL, Goose migrations, `slog` structured logging, `go-playground/validator`, OpenAPI 3.1, `httptest` tests.
- Database: PostgreSQL 16+, `pg_trgm`, `unaccent` where available, JSONB only for category-specific attributes, ordinary normalized relational tables for identities and relations.
- Frontend: Next.js App Router, TypeScript strict, React, Tailwind CSS, accessible UI primitives (shadcn/ui optional), React Hook Form + Zod, TanStack Query where useful, server components for SEO pages.
- Auth: secure admin session with HttpOnly Secure SameSite cookies, Argon2id for passwords if self-hosted, CSRF protections for cookie-authenticated mutations; do not invent custom JWT auth when sessions suffice.
- Storage: local development storage abstraction; production S3-compatible private object storage for verification documents; never publicly expose uploads by default.
- Infrastructure: Docker Compose for PostgreSQL, backend, frontend; `.env.example`; Makefile; GitHub Actions CI; optional deployment guides for a small VPS and managed PostgreSQL.
- Prefer permissively licensed libraries and document licenses. Pin dependency versions and verify compatibility at implementation time.

Repository structure:
```
/apps/api/             # Go HTTP API
/apps/web/             # Next.js
/internal/             # Go domain, service, repository, adapters
/db/migrations/        # SQL schema migrations
/db/queries/           # sqlc queries
/packages/openapi/     # API contract and generated client types
/scripts/              # import, seed, maintenance
/docs/                 # architecture, privacy, source policy, runbooks
/infra/                # compose and deployment samples
```
Adjust if the repo already has a coherent structure.

## 3. Industry taxonomy and entity model

Do NOT put every industry into one flat enum. Support an arbitrary-depth category tree and multiple business roles per company. Seed a practical taxonomy with examples, and optionally map to official NIC 2008/NIC 2025 codes only after verifying official code mappings. Never fabricate NIC codes.

Initial top-level commercial categories:
- Food and beverages: namkeen, snacks, bakery, spices, dairy, packaged foods, grain milling, edible oils, beverages.
- Textiles and apparel: rugs, carpets, yarn, fabric, garment manufacturing, home textiles, dyeing, finishing.
- Building and construction materials: ceramic/vitrified/porcelain tiles, bricks (clay/fly-ash), AAC/concrete blocks, cement, pavers, precast concrete, sanitaryware, stone, glass, pipes.
- Metals and fabrication: iron/steel, aluminium, brass, foundries, sheet metal, fabrication, fasteners.
- Industrial machinery and components: CNC machining, gears, shafts, bearings, pumps, motors, conveyors, compressors, tools, dies, molds.
- Plastics, rubber and packaging: injection molding, extrusion, rubber seals, flexible packaging, corrugated boxes, labels.
- Electrical and electronics: transformers, wiring, PCBs, control panels, sensors, electronic assembly.
- Automotive and transport: auto parts, railway components, aerospace components.
- Chemicals, pharma, cosmetics and medical devices.
- Furniture, wood, paper, printing, leather, footwear, handicrafts, jewellery, toys, renewables, recycling.
- Industrial services: testing, certification, quality inspection, job work, maintenance, tooling, logistics support.
- Sourcing and trading: buying houses, sourcing agents, exporters, importers, wholesalers, distributors.

Business roles (many-to-many): manufacturer, contract_manufacturer, oem, odm, raw_material_supplier, component_supplier, job_work_provider, sourcing_agent, buying_house, exporter, importer, wholesaler, distributor, testing_lab, industrial_service_provider.

Industry classification, business role, product category, product, manufacturing process, facility, and capability must be separate concepts. A company may have many of each.

## 4. Database design — implement this FIRST

Use UUID primary keys, `created_at`, `updated_at`, optional `deleted_at`, constraints, foreign keys, indexes, and auditability. Avoid leaking internal IDs in public pages if not needed. Include a data dictionary and an ERD (Mermaid) in docs.

Required tables:
1. `companies`: id, legal_name, display_name, slug, normalized_name, description, website_url, establishment_year, business_status, publication_status, verification_status, primary_category_id nullable, created_at, updated_at, deleted_at.
2. `company_roles`: company_id, role_code, composite PK.
3. `facilities`: id, company_id, type (factory/office/warehouse/etc), address fields, locality, city, district, state_code, postal_code, country_code, geo fields optional, is_primary, source_id, verification status. Do not store Google-sourced coordinates beyond provider permissions.
4. `categories`: id, parent_id nullable, name, slug, category_type, description, active, sort_order; unique sibling slug; recursive category queries.
5. `company_categories`: company_id, category_id, evidence/source id, confidence, verified_at.
6. `products`: id, company_id, category_id, name, normalized_name, description, unit, attributes JSONB, evidence/source id.
7. `capabilities`: id, slug, name, type, description.
8. `company_capabilities`: company_id, capability_id, evidence/source id, optional attributes JSONB.
9. `certifications`: id, company_id, certification_type, issuer, certificate_number nullable, issued_at, expires_at, status, evidence_document_id; never default to verified.
10. `contacts`: id, company_id, contact_type (public_business_email, switchboard, sales_department, individual), value encrypted if sensitive, is_public, lawful_basis_or_permission_ref, source_id, verified_at. Exclude personal contacts from public/API responses by default.
11. `data_sources`: id, source_type (self_submission, authorized_import, official_site, licensed_feed, permitted_api_lookup, manual_verification), provider, source_url, license_reference, collected_at, allowed_internal_use, allowed_public_display, allowed_commercial_api, retention_expires_at nullable, restrictions JSONB, notes. **Default all permission flags to false until evidenced.**
12. `field_provenance`: id, company_id, entity_type, entity_id, field_name, source_id, observed_at, permission_status, confidence; track field-level provenance where practical.
13. `supplier_submissions`: id, status, payload JSONB, submitter_email (protected), privacy_notice_version, storage_consent_at, public_listing_permission_at nullable, future_commercial_api_permission_at nullable, submitted_at, reviewed_at, reviewer_id. Consent/authorization must be specific, separate, revocable where applicable, and documented; do not use prechecked boxes.
14. `verification_events`: id, company_id, field/entity, method, result, evidence_id nullable, checked_at, checked_by, next_review_at.
15. `import_jobs`: id, filename, file_hash, declared_source_id, status, totals, errors, initiated_by, created_at, finished_at.
16. `import_rows`: id, import_job_id, row_number, raw_data JSONB, normalized_data JSONB, duplicate_candidates JSONB, validation_errors JSONB, disposition (pending/accepted/rejected/merged), matched_company_id.
17. `documents`: id, company_id, private_object_key, mime_type, size, checksum, access_level, retention_policy, uploaded_at.
18. `admin_users`: id, email unique, password_hash or external auth id, role, active, created_at, last_login_at.
19. `audit_logs`: id, actor_id, action, entity_type, entity_id, before JSONB, after JSONB, request_id, created_at; protect secrets and personal information in logs.
20. `deletion_requests`: id, requester, company_id nullable, scope, status, received_at, resolved_at, resolution_notes.
21. `api_clients` and `api_keys`: future-ready, disabled for external consumers in v1; store only hashes of API keys.
22. `api_usage_daily`: client_id, date, endpoint_group, request_count, error_count (future-ready).

Recommended indexes: company normalized_name trigram; company slug unique; category parent_id; facility state/city; composite company-category; import job/row status; verification date; source permissions; publication status. Build duplicate candidate logic using normalized name + website domain + address/phone where lawfully available; never auto-merge uncertain matches.

Model constraints:
- A public company page requires `publication_status=published` AND field-level public permission.
- Commercial API exposure requires explicit compatible source rights for every returned field.
- Do not confuse `unverified` with `false` for certification/capability.
- Source terms, permission status, and retention controls must survive imports and merges.
- Supplier submissions remain pending until reviewed.
- Use soft-delete only where justified; honor valid erasure requests, retention and audit requirements.

## 5. Backend API contract

Prefix `/api/v1`; return consistent error shape `{error:{code,message,request_id,details?}}`; pagination with cursor or page/limit; validation and bounded limits; versioned OpenAPI docs.

Public:
- `GET /healthz` and `GET /readyz` (no secrets).
- `GET /api/v1/categories` and `GET /api/v1/categories/tree`.
- `GET /api/v1/public/companies?category=&state=&city=&role=&q=&page=&limit=` only approved publishable fields.
- `GET /api/v1/public/companies/{slug}` only approved publishable fields.
- `POST /api/v1/supplier-submissions` with validation, rate limit, anti-spam honeypot, optional CAPTCHA after abuse, consent timestamps and privacy notice version.
- `POST /api/v1/privacy-requests` for correction/removal requests.

Admin (authentication + authorization required):
- `POST /api/v1/admin/login`, `POST /logout`, `GET /me`.
- `GET /api/v1/admin/companies`, `POST /companies`, `PATCH /companies/{id}`, `GET /companies/{id}`.
- `GET /api/v1/admin/submissions`, `POST /submissions/{id}/approve`, `/reject`.
- `POST /api/v1/admin/imports` upload CSV, `GET /imports/{id}`, `GET /imports/{id}/rows`, `POST /imports/{id}/commit` with dry-run preview.
- `GET /api/v1/admin/duplicates`, `POST /duplicates/{id}/resolve`.
- `POST /api/v1/admin/companies/{id}/verify`, `POST /companies/{id}/publish`, `POST /companies/{id}/unpublish` with policy checks.
- `GET /api/v1/admin/audit-logs`, `GET /api/v1/admin/stats`.
- `POST /api/v1/admin/exports` permission-filtered export with audit log.

Use role-based authorization (owner/admin/reviewer), transaction boundaries for approvals/merges, idempotency for imports and submissions, rate limits, request size caps, safe URL validation, parameterized queries, CORS allowlist, timeouts, graceful shutdown, and structured logs with redaction. Do not expose admin docs publicly in production.

## 6. Import pipeline for the existing 1,200 records

Accept CSV first; optionally XLSX via a separate well-maintained library. Provide a column-mapping UI. Require declaration of data origin and reuse rights before import. Stages: upload -> parse -> normalize -> validate -> candidate dedupe -> preview -> review -> commit. Unknown source rights must remain private/staged. Normalize state names, phone formatting (without guessing missing country codes), domains, whitespace, categories. Retain raw row and transformation log. Handle partial rows and malformed files. Produce downloadable validation report. Support rollback/undo through transaction or compensating audit actions. Seed 20 synthetic records across rug, namkeen, tile, brick, CNC, and sourcing roles for tests; never imply they are real suppliers.

## 7. Data acquisition connectors and Google Places boundary

Create a `DiscoveryProvider` interface with `SearchBusinesses(ctx, query, location)`, `GetPermittedMetadata(ctx, id)`, and provider-specific retention policy. Implement a **disabled-by-default** Google Places adapter only after reviewing current terms and using an authorized API key. Use proper Places endpoints, billing controls, field masks, attribution, and quota safeguards. Discovery results should appear in a temporary review UI and should NOT be ingested into the permanent supplier record store unless each field's independent permitted source/rights have been established. Avoid writing a Google Maps scraper repository integration.

Other sources: supplier-submitted records; CSV with license metadata; manual first-party research from supplier-authorized materials; optional government open data with verified license and attribution. Provide provider adapter tests with mocks, no live paid API calls in CI. Maintain a `docs/data-source-policy.md` describing permitted collection, prohibited collection, retention, and commercial API eligibility.

## 8. Next.js website information architecture

Public routes:
- `/` homepage: what the directory does, category search, supplier registration CTA, transparent beta status.
- `/industries` and `/industries/[slug]`: useful original descriptions, nested categories, relevant permitted profiles.
- `/locations` and `/locations/[state]`: state-focused pages only where there is genuine useful content.
- `/suppliers/[slug]`: approved profiles only, with factual description, source/verification labels, update date, correction link.
- `/register-supplier`: responsive multi-step form with business identity, roles, categories, products, facilities, business contact, permission checkboxes, review/submit.
- `/about`, `/contact`, `/privacy`, `/terms`, `/data-policy`, `/request-correction`.
- `/login`, `/admin`, `/admin/companies`, `/admin/submissions`, `/admin/imports`, `/admin/duplicates`, `/admin/taxonomy`, `/admin/sources`, `/admin/audit`.

Form UX: mobile-first, keyboard accessible, Zod validation, progress indicator, autosave locally only if privacy-safe, helpful examples, no misleading 'verified' claims, upload documents optional, consent text linked to versioned privacy/data-use policies, email confirmation where available. Supplier must be able to request correction and removal. No public exposure before approval.

Design: trustworthy neutral B2B visual identity, fast pages, clear information hierarchy, accessible contrast, responsive tables, loading/error/empty states, pagination, search and filters, toast feedback, no fake testimonials or fake company counts.

## 9. SEO implementation — without exposing private data

- Next.js server-rendered metadata, canonical URLs, `generateMetadata`, XML sitemap generated only from approved public routes, robots.txt, Open Graph images, semantic HTML, breadcrumbs.
- Schema.org `Organization`, `WebSite`, `BreadcrumbList`, and `LocalBusiness` only when the real entity/fields justify it; no invented ratings, reviews, opening hours or locations.
- Search Console setup guide, analytics with privacy-aware configuration, Core Web Vitals targets, image optimization, structured headings, accessible navigation.
- `noindex` on admin, login, submissions, search result permutations, empty/thin pages, staging, and any private/unapproved profile. Do not create thousands of near-duplicate location-category SEO pages with no original value.
- Unique industry landing pages with educational content and verified examples; avoid keyword stuffing and programmatic doorway pages.
- sitemap and robots tests; 404/410 for removed profiles as appropriate; redirects for slug changes.

## 10. Supplier registration and trust

Draft concise, plain-language supplier-facing copy:
- Why register: free basic profile, discoverability when launched, easy corrections, potential sourcing enquiries; do NOT promise leads or rankings.
- What information is required vs optional.
- How data will be used now (private storage/review), what public listing would mean, and what future commercial/API redistribution could mean.
- Three separate unchecked authorizations: data submission/processing notice acknowledgement (use proper legal basis, not blindly assume consent), optional public listing, optional future commercial/API distribution of specified business fields.
- Explain how to withdraw applicable consent/request corrections and how long data is retained.
- Never bury resale permission in generic terms.

Draft form confirmation email, approval/rejection email, correction request acknowledgement, and a one-to-one invitation template for suppliers. Do not implement unsolicited bulk marketing. A privacy lawyer should review notices and permissions before launch.

## 11. Admin verification workflow

Status machine: `draft -> submitted -> under_review -> approved_private -> published`, plus `rejected`, `needs_changes`, `archived`. Independent verification: `unverified`, `partially_verified`, `verified`, `stale`. Publishing does not imply verification. Verification evidence: direct supplier confirmation, official documents where legally appropriate, website checks, documented manual review. Track reviewer and date; prompt re-verification after configurable intervals. Implement duplicate review and safe merges with provenance preservation.

## 12. Security, privacy and reliability

- Threat model for spam submissions, SQL injection, XSS, SSRF through URLs, malicious file uploads, admin account takeover, data leakage, scraping of your own public pages, API abuse.
- Rate limits on forms/login; safe upload types/size; MIME sniffing; private document storage; malware scan integration optional; backups and restore drills; encryption in transit; secrets outside repo.
- Minimal collection; separation of public company info and personal contact data; field-level access and source rights; retention schedules; correction/deletion workflows; audit logs.
- Evaluate India's applicable DPDP rules, IT requirements, and contractual obligations with qualified counsel. Being a solo founder or not having GST does not remove privacy and licensing responsibilities.
- Monitoring: structured logs, error reporting, basic metrics, health checks, backup alerts. Keep monthly cost modest.

## 13. Testing and acceptance criteria

Backend: migration up/down tests, repository tests, API handler tests, auth/RBAC tests, import validation/dedup tests, permission enforcement tests, publication gating tests, rate-limit tests.
Frontend: component tests for registration form, admin review, search filters; Playwright end-to-end flows: submit -> review -> approve private -> authorize publish -> public profile appears; deny publish -> profile absent; import -> dedup -> commit; correction request.
Security assertions: unpublished records never appear in sitemap, public APIs, page metadata, static builds, or cache; personal contacts never leak; restricted source fields never appear in commercial export; admin endpoints require auth.
Performance: index-backed search and paginated results; test 1,200 records then 100,000 synthetic records; avoid premature distributed infrastructure.

Definition of done for MVP: `docker compose up` starts everything; migrations run; seed loads; supplier submits; admin reviews; CSV import works; public SEO pages render; tests pass; README explains setup; `.env.example` has no secrets; all permission defaults are restrictive.

## 14. Milestones — implement one at a time

**M0 — Discovery and decisions:** Inspect repo, document assumptions, list unresolved provider terms/legal decisions, architecture diagram, detailed task breakdown. Do not code before showing the plan.

**M1 — Database foundation (START HERE):** PostgreSQL Compose, Goose migrations for core tables, constraints/indexes, seed categories and synthetic examples, ERD, data dictionary, test migration. Implement no scraper. Stop and report files changed, commands, schema decisions, and tests.

**M2 — Go API foundation:** configuration, health checks, typed queries, error format, logging, auth/session and RBAC, OpenAPI, tests.

**M3 — Import and private admin:** CSV staging, source rights declaration, normalization, dedup review, admin listing and editing, audit logs.

**M4 — Supplier self-registration:** form, separate permissions, submissions review, confirmation, corrections, anti-spam.

**M5 — Public Next.js pages and SEO:** publish-gated pages, category pages, sitemap, metadata, structured data, accessibility, tests.

**M6 — Discovery adapter:** optional Google Places compliant discovery-only integration with provider terms checklist and cost controls; if permissions are unclear, leave disabled and document why. Licensed source adapters can be added separately.

**M7 — Deployment and operations:** secure VPS/managed DB deployment, backups, monitoring, privacy documentation, launch checklist.

**M8 — Future API product (do not build until demand):** external API keys, rate limiting, usage plans, billing, commercial-permission filtering, data freshness SLAs, developer docs.

At the end of each milestone provide: implementation summary, directory tree changes, migration instructions, example requests, tests run and results, known risks, and the exact next command for the founder. Wait for confirmation before starting the next milestone unless the founder explicitly says to proceed.

## 15. Exact initial instruction to Claude Code

**START NOW WITH M0 AND M1 ONLY.** Inspect the repository; propose the minimal architecture; create PostgreSQL migrations, category seed data, source-rights/provenance model, import staging tables, and synthetic seed records. Add Docker Compose, `.env.example`, Makefile, ERD, and README setup instructions. Include migration and constraint tests. Do not implement Google Maps scraping, API monetization, marketing automation, or the entire UI yet. Show a short explanation of how this schema can support rugs, namkeen, tiles, bricks, CNC component makers, and sourcing companies. Run tests and report the result. If there is no existing repository, initialize a clean monorepo structure.

## 16. Founder checklist (manual work outside code)

- Audit the source and permitted uses of the existing 1,200 records; do not assume resale rights.
- Choose 1–2 geographic/industry clusters for the pilot.
- Register a domain only when ready; use a temporary development hostname meanwhile.
- Create a dedicated business email and privacy contact channel.
- Obtain legal review of supplier registration wording, privacy notice, data licensing, and commercial API resale permissions before collecting sensitive data or launching publicly.
- Interview 10–20 potential procurement/B2B software buyers before adding paid API features.
- Track verification coverage, duplicate rate, supplier permission rate, form conversion, and buyer pilot interest.

## 17. Key architectural decisions to document

Record decisions as ADRs: why PostgreSQL; why Go monolith; why Next.js; why source/field permissions; why no Maps bulk scraper; why publication is opt-in; why external API billing is deferred. Include a realistic small-scale monthly hosting cost estimate only after choosing a provider and checking current pricing. Clearly label all estimates.

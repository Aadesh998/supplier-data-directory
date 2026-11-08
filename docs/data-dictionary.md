# Data dictionary

Schema source of truth: `db/migrations/`. Conventions:
- UUID primary keys (`gen_random_uuid()`).
- `created_at` / `updated_at` maintained by triggers.
- `deleted_at` for soft delete where history matters.
- Enumerations are `text` + `CHECK`, not Postgres enums, so they can change without type migrations.

## Reference data

| Table | Purpose | Key columns |
|---|---|---|
| `indian_states` | 28 states + 8 UTs, ISO 3166-2:IN codes (2023 revision: CG, OD, TS, UK) | `code` PK, `name`, `slug` (URL), `region_type` |
| `business_roles` | 15 roles (manufacturer … industrial_service_provider) | `code` PK |
| `categories` | Arbitrary-depth commercial taxonomy. A trigger prevents cycles. | `parent_id`, `slug` (globally unique; used in URLs; immutable via API), `category_type` (industry, product_category, service, trade), `nic_code`/`nic_version` (only verified official codes; both or neither) |
| `capabilities` | Processes, equipment, quality, services | `slug`, `capability_type` |

## Companies and related records

| Table | Purpose | Notable rules |
|---|---|---|
| `companies` | One business entity | `publication_status` workflow; `verification_status` independent. `published` requires `published_at`, not deleted, not withdrawn, and (trigger) public-display provenance for `display_name`. `slug` stays reserved after soft delete. `normalized_name` (lowercase, legal suffixes stripped) is trigram-indexed for search and dedupe. |
| `slug_redirects` | Old slug → company (after renames and merges) | Public API answers 301 |
| `company_roles` | Many-to-many roles, each with `source_id` | |
| `company_categories` | Many-to-many categories, with `source_id`, `confidence`, `verified_at` | |
| `facilities` | Factories, offices, warehouses… | One primary per company (partial unique index). PIN must be 6 digits for India. Coordinates need their own `geo_source_id`. |
| `products` | What the company makes | `attributes` JSONB for category-specific specs only |
| `company_capabilities` | Claimed capabilities | `verification_status` defaults to `unverified` (claimed, not checked; this is not the same as "doesn't have") |
| `certifications` | ISO, BIS, FSSAI, etc. | Defaults to `unverified`. `verified` requires `verified_at` + `verified_by`. Only `claimed` / `unverified` / `verified` / `expired` are shown publicly. |
| `contacts` | Business and personal contact points | Exactly one of `value` / `value_encrypted`. `individual` must be encrypted and never `is_public`. `is_public` requires `lawful_basis_or_permission_ref`. `value_hash` = HMAC for dedupe. |
| `documents` | Private evidence files (metadata only) | MIME allowlist, ≤10 MB, SHA-256 checksum, `access_level` private/restricted only |
| `verification_events` | Each verification check | `method`, `result`, `checked_by`, `next_review_at` |

## Provenance and permissions

| Object | Purpose |
|---|---|
| `data_sources` | Where data came from. `source_type`: self_submission, authorized_import, official_site, licensed_feed, permitted_api_lookup, manual_verification. **All three permission flags default to false.** Public or commercial rights require internal-use rights. `permitted_api_lookup` sources can never be public or commercial. `retention_expires_at` turns permissions off automatically. `restrictions` JSONB holds free-form terms. |
| `field_provenance` | `(entity_type, entity_id, field_name, source_id)` → `permission_status` (`unknown`, `permitted`, `restricted`, `withdrawn`) |
| `public_field_permissions` (view) | Fields that may be shown publicly |
| `commercial_field_permissions` (view) | Fields that may be redistributed commercially |
| `internal_field_permissions` (view) | Fields usable internally |

## Intake

| Table | Purpose | Notable rules |
|---|---|---|
| `supplier_submissions` | Self-registrations | `payload` excludes the submitter email (encrypted column + HMAC). `storage_consent_at` required. Optional permissions are separate timestamps with separate withdrawal timestamps. `idempotency_key` is unique. Approved submissions must have `company_id`. |
| `import_jobs` | One uploaded file | `declared_source_id` required. Unique active `(file_hash, source)` gives idempotency. Counters for valid, error, duplicate and committed rows. |
| `import_rows` | One CSV row | Keeps `raw_data` untouched plus `normalized_data`, `transformation_log`, `validation_errors`/`warnings`, `duplicate_candidates`. `disposition` is pending, accepted, rejected or merged (merged requires `matched_company_id`). |
| `duplicate_pairs` | Candidate pairs for review | Ordered `(a < b)`, unique. Never auto-merged. Resolution recorded. |
| `deletion_requests` | Privacy requests: correction, removal, withdraw public listing / commercial use, access, other | Requester email encrypted |
| `discovery_leads` | Provider place IDs + our notes only (from the interactive Places lookup) | No provider content stored |
| `discovery_job_batches` | One jobs CSV upload | Counts of inserted / updated / skipped rows, plus warnings |
| `discovery_jobs` | One search: sector keyword × location, with `provider` (`osm_overpass` / `google_places_ids`), coordinates, radius, priority | `job_key` (CSV `job_id`) unique. Status `pending → running (leased) → done → (due again after refresh_interval_days)`, or `failed` after `max_attempts` with exponential backoff, or `paused`. A running job must have a lease. |
| `discovery_results` | Leads found by workers, deduplicated by `(provider, provider_ref)` | **`google_places_ids` rows cannot hold name, coordinates or tags** (CHECK constraint). OSM tags are filtered to a business allowlist. Status `new / contacted / dismissed / promoted`. Promotion creates a private draft company with an internal-only OpenStreetMap source. |

## Platform

| Table | Purpose |
|---|---|
| `admin_users` | Owner / admin / reviewer; Argon2id hash; lockout counters |
| `admin_sessions` | Token SHA-256, CSRF token, expiry, revocation |
| `audit_logs` | Append-only (update/delete raise an error); before/after snapshots with personal and secret keys redacted |
| `api_clients`, `api_keys`, `api_usage_daily` | Future external API. A constraint prevents any client from being `active` in v1. Only key hashes are stored. |

## Retention (enforced by `api maintenance`, daily inside `serve`)

| Data | Rule |
|---|---|
| Spam submissions (never became companies) | Deleted after 30 days |
| Rejected submissions (never became companies) | Deleted after 180 days |
| Expired/revoked sessions | Deleted after 30 days |
| Sources past `retention_expires_at` | Fields stop being permitted. Affected companies are unpublished. |
| Verified companies past `next_review_at` | Marked `stale` |
| Erasure request | `EraseCompany` hard-deletes the company and dependants and anonymises linked submissions. The audit log keeps only ids. |

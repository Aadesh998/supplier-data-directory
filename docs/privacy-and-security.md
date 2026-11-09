# Privacy and security

## Threat model and controls

| Threat | Controls in place | Residual risk / to do |
|---|---|---|
| **Spam submissions** | Honeypot field; minimum 3 s fill time (bots silently dropped); per-IP limits (10/h, burst 5) on submissions and privacy requests; max 3 per email per day; idempotency key; manual review before anything is created | Add a CAPTCHA (e.g. Cloudflare Turnstile) if abuse appears |
| **SQL injection** | All queries parameterised (pgx). Dynamic filters are built from fixed SQL fragments only. | — |
| **XSS** | React escapes output; no `dangerouslySetInnerHTML` except JSON-LD built from `JSON.stringify`; API sends JSON with `nosniff` and a `default-src 'none'` CSP | Add a site-wide CSP in Caddy once inline scripts are audited |
| **SSRF via URLs** | Website URLs normalised and rejected if they are IP literals, localhost, internal TLDs, have credentials or custom ports. The API never fetches stored URLs. | — |
| **Malicious uploads** | CSV only: extension check, content sniffing, 10 MB / 20,000-row caps, UTF-8 validation, formula-injection escaping in generated CSVs. Evidence documents (schema only) have a MIME allowlist and size cap and are always private. | Malware scanning if document upload is added |
| **Admin account takeover** | Argon2id (64 MB, t=3); constant-time compare and dummy hash for unknown emails; lockout after 5 failures for 15 min; per-IP login limit; server-side sessions (token hashed), HttpOnly, SameSite=Lax, Secure in production; logout revokes | Add TOTP 2FA before adding more admins |
| **CSRF** | Per-session token in `X-CSRF-Token` on every admin mutation, plus Origin/Referer allowlist | — |
| **Privilege misuse** | RBAC (reviewer/admin/owner); owner-only for source permission changes, commercial exports, erasure, user management; every mutation and every view of submitter personal data is audit-logged; audit log is append-only at the database level | — |
| **Data leakage** | Publication gate in code and in a DB trigger; field-level permission filtering on every public response and export; personal contacts encrypted (AES-256-GCM) and masked even in admin views; submitter email kept out of JSON payloads; request logs omit query strings and bodies; audit snapshots redact personal keys | — |
| **Scraping of our own pages** | Per-IP limits on public API (120/min); page depth capped at 500; result totals capped at 10,000; public responses cacheable for 60 s only. Public profiles are teasers: contacts, website, legal name, locality, certificate issuers and products beyond 3 are stripped server-side. | Bot management at the CDN if needed |
| **Rate-limit evasion via spoofed IPs** | API trusts `X-Forwarded-For` only when `TRUST_PROXY_HEADERS=true`. Production puts Caddy in front, which discards client-supplied XFF (verified: spoofed headers still hit the limit). | Locally, without Caddy, the limit can be bypassed; account lockout still applies |
| **Worker API misuse** | `/api/v1/worker/*` disabled unless `WORKER_TOKEN` (≥32 chars) is set; constant-time bearer check; jobs are leased to a worker id, so a stale or foreign worker cannot complete them; result payloads capped (2,000 items, 8 MB); OSM tags allowlisted; Google content rejected by the DB | Rotate `WORKER_TOKEN` if it leaks (`wrangler secret put` + redeploy) |
| **Server-side render throttling** | Web server sends `INTERNAL_API_TOKEN` on its own API calls, so page renders don't share one IP bucket; the browser proxy never forwards that header | Keep the token secret; rotate like any secret |
| **API abuse (future)** | `api_clients` cannot be activated in v1; keys stored as hashes only | Build quotas and billing in M8 |
| **Secrets** | Only from environment variables; `.env` ignored by git; production refuses to start without `DATA_ENCRYPTION_KEY` and Secure cookies | Keep the encryption key in a password manager, separately from backups |
| **Data loss** | Nightly `pg_dump` with verification, retention, optional off-site copy; restore drill script | Run the drill monthly and record it |

## Personal data inventory

| Data | Where | Protection | Visible to |
|---|---|---|---|
| Submitter name, designation, phone | `supplier_submissions.payload` | Access audit-logged | Admins |
| Submitter email | `supplier_submissions.submitter_email_encrypted` | AES-256-GCM + HMAC lookup | Admins (decrypted on the detail view, audit-logged) |
| Individual contacts | `contacts` (`individual`) | Encrypted, masked in UI, never public or exported | Masked to admins |
| Privacy requester email | `deletion_requests.requester_email_encrypted` | Encrypted | Admins |
| GSTIN (may embed a proprietor's PAN) | `companies.gstin` | Never in public/commercial responses or exports | Admins |
| IP addresses | `submitter_ip_hash`, `admin_sessions.ip_hash` | HMAC pseudonym only | — |

## Data subject rights workflow

The public form at `/request-correction` creates a request and sends an acknowledgement email. Admin → Privacy requests → resolve with an action:
- `unpublish`
- `withdraw_public`: unpublishes, blocks re-publishing, hides contacts
- `withdraw_commercial`: excluded from commercial exports
- `erase_company` (owner): permanent deletion and anonymised submission

The audit log records the action with ids only.

## Legal (needs qualified review)

- India's Digital Personal Data Protection Act, 2023 and the DPDP Rules 2025 have phased implementation. Confirm your obligations as a Data Fiduciary: notice, consent/legitimate use, purpose limitation, retention, grievance officer, breach notification.
- The supplier form records three separate, unticked authorisations with the privacy-notice version and timestamps. A lawyer should review the wording on `/privacy`, `/terms` and `/data-policy` (marked "Draft — pending legal review").
- Not having a company or GST registration does not remove privacy or licensing responsibilities.

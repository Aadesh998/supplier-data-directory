# ADR 0008: Server-side sessions with CSRF tokens (no JWT)

**Status:** accepted · 2026-11-07

## Context
Only a handful of admins use the console, from a browser on the same origin as the site.

## Decision
- Argon2id password hashes.
- An opaque 256-bit session token in an HttpOnly, SameSite=Lax cookie (Secure in production). Only its SHA-256 is stored in `admin_sessions`.
- A per-session CSRF token sent as `X-CSRF-Token`, plus an Origin allowlist on mutations.
- Lockout after 5 failures, plus per-IP login rate limiting.

## Consequences
- Sessions can be revoked instantly (logout, deactivation) with no token-expiry window.
- Add TOTP 2FA before adding more admins. An external IdP can be added through `admin_users.external_auth_id`.

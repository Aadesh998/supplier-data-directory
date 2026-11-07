# ADR 0006: Publication is opt-in and gated twice

**Status:** accepted · 2026-11-07

## Context
Publishing an unapproved or unpermitted profile is the most damaging mistake this system can make, both legally and for trust.

## Decision
- Every record starts private (`draft`, `submitted` or `approved_private`). Publishing is an explicit admin action.
- `CheckPublishable` requires approval, no withdrawal, and public permission for the display name plus at least one category and one location. A `BEFORE UPDATE` trigger independently refuses `published` without display-name permission.
- Public queries re-check permission on every request rather than trusting the status column.
- Publishing never changes verification status. Pages show verification labels honestly.
- The site is `noindex` and `robots: Disallow /` until `NEXT_PUBLIC_ALLOW_INDEXING=true`.

## Consequences
- More clicks per company. That cost is accepted.
- Defence in depth: a bug in one layer does not leak data.

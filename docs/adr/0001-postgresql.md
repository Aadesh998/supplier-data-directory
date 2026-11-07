# ADR 0001: PostgreSQL as the only datastore

**Status:** accepted · 2026-11-07

## Context
We need relational integrity across companies, facilities, categories and provenance, plus fuzzy search for duplicates and auditability. The team is one person, and hosting must stay cheap.

## Decision
Use PostgreSQL 17 for everything:
- `pg_trgm` handles search and duplicate candidates
- JSONB holds only category-specific product attributes and import staging
- constraints and triggers enforce the policies that must never be broken (publication gate, contact protection, append-only audit, v1 API lockout)

## Consequences
- One system to back up and monitor, and managed offerings exist if needed.
- Search is good to roughly 10⁵–10⁶ rows with the current indexes (100k measured at <0.5 s). A dedicated search engine is deferred until it is measured as necessary.
- Business rules partly live in SQL. They are covered by `internal/database/constraints_test.go`.

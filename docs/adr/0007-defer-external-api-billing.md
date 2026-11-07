# ADR 0007: Defer the external API and billing until buyers validate demand

**Status:** accepted · 2026-11-07

## Context
The blueprint says to validate paying API demand with 10–20 buyer interviews and a paid pilot before heavy engineering. Commercial redistribution also needs legal review of supplier permissions.

## Decision
- Ship the tables (`api_clients`, `api_keys` as hashes, `api_usage_daily`) but forbid activation in v1 with a database constraint (`api_clients_disabled_in_v1`).
- Paid pilots can be served with the owner-only, audit-logged, permission-filtered `commercial` CSV export.
- M8 (keys, quotas, billing, change feed, developer docs) starts only after a buyer commits.

## Consequences
- No billing or quota code to maintain yet.
- Enabling the API requires a migration that drops the constraint, which is a deliberate speed bump.

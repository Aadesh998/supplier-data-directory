# ADR 0002: A single Go API binary (no microservices)

**Status:** accepted · 2026-11-07

## Context
A solo developer needs low operating cost, simple deploys, strong typing and good concurrency for imports.

## Decision
One Go binary (`apps/api`) built on `net/http` + chi, pgx/v5, Goose migrations embedded in the binary, `slog` JSON logs and go-playground/validator. The same binary runs `serve`, `migrate`, `seed`, `create-admin`, `maintenance` and `healthcheck`. Maintenance runs in-process daily for single-instance deployments.

**Deviation from the original spec:** the spec suggested `sqlc`. We use pgx directly with explicit scanning in `internal/store`. Most read queries are dynamic filter combinations (public search, admin lists, permission-filtered composition), which sqlc handles awkwardly. All SQL is still parameterised, and the integration tests exercise every query path. sqlc can be adopted for static queries later without schema changes.

## Consequences
- Deploying means one container. No service discovery and no message queue.
- The in-memory rate limiter assumes a single instance. Swap it for a shared store before scaling horizontally.

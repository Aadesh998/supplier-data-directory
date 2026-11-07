# Bharat Manufacturing Directory — developer commands.
SHELL := /bin/bash
GO_PKGS := ./apps/api/... ./internal/... ./db/... ./packages/...
-include .env
export
# Tests create throwaway databases via the bharatmfg_test maintenance DB (see infra/postgres/init-test-db.sql).
TEST_DATABASE_URL ?= postgres://$(or $(POSTGRES_USER),bharatmfg):$(or $(POSTGRES_PASSWORD),devpassword)@localhost:$(or $(POSTGRES_PORT),5434)/bharatmfg_test?sslmode=disable
export TEST_DATABASE_URL

.PHONY: help up down db-up db-down migrate migrate-down migrate-status reset seed seed-demo admin run web \
        test test-go test-web test-perf e2e lint fmt build docker-build maintenance

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

up: ## Start postgres, mailpit, api and web with Docker Compose
	docker compose up -d --build

down: ## Stop all containers (keeps data)
	docker compose down

db-up: ## Start only PostgreSQL
	docker compose up -d --wait postgres

db-down:
	docker compose stop postgres

migrate: ## Apply migrations (host API binary)
	go run ./apps/api migrate up

migrate-down: ## Roll back the latest migration
	go run ./apps/api migrate down

migrate-status:
	go run ./apps/api migrate status

reset: ## Drop and re-apply all migrations (refused in production)
	go run ./apps/api migrate reset

seed: ## Load 20 synthetic, private records
	go run ./apps/api seed

seed-demo: ## Load synthetic records AND publish them locally (development only)
	go run ./apps/api seed --demo

admin: ## Create an owner account: make admin EMAIL=you@example.com
	go run ./apps/api create-admin --email "$(EMAIL)" --role owner

run: ## Run the API on the host
	go run ./apps/api serve

web: ## Run the Next.js dev server
	cd apps/web && npm run dev

maintenance: ## Run re-verification / retention jobs once
	go run ./apps/api maintenance

test: test-go test-web ## Run all unit and integration tests

test-go: db-up ## Go tests (needs PostgreSQL from `make db-up`)
	REQUIRE_DB=1 go test -race -count=1 $(GO_PKGS)

test-perf: db-up ## 100k-record performance test
	PERF=1 REQUIRE_DB=1 go test -count=1 -run Performance -v ./internal/store/

test-web:
	cd apps/web && npm run typecheck && npm run lint && npm test

e2e: ## Playwright end-to-end tests against the running stack (make up first)
	cd apps/web && npx playwright test

lint: ## go vet + gofmt check + web lint
	go vet $(GO_PKGS)
	@test -z "$$(gofmt -l apps internal db packages)" || (gofmt -l apps internal db packages; exit 1)
	cd apps/web && npm run lint

fmt:
	gofmt -w apps internal db packages

build: ## Build the API binary into bin/
	CGO_ENABLED=0 go build -trimpath -o bin/api ./apps/api

docker-build:
	docker compose build

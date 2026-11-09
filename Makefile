# Peoplvance — developer commands.
SHELL := /bin/bash
GO_PKGS := ./apps/api/... ./internal/... ./db/... ./packages/...
-include .env
export
# Tests create throwaway databases via the bharatmfg_test maintenance DB (see infra/postgres/init-test-db.sql).
# Always the LOCAL container (LOCAL_PG_*), never DATABASE_URL: tests create and drop databases.
TEST_DATABASE_URL ?= postgres://$(or $(LOCAL_PG_USER),bharatmfg):$(or $(LOCAL_PG_PASSWORD),devpassword)@localhost:$(or $(LOCAL_PG_PORT),5434)/bharatmfg_test?sslmode=disable
export TEST_DATABASE_URL

.PHONY: help up down db-up db-down migrate migrate-down migrate-status reset seed seed-demo admin run web \
        test test-go test-web test-perf e2e lint fmt build docker-build maintenance \
        build-linux build-web ec2-setup ec2-env ec2-scp ec2-deploy ec2-web-scp ec2-web-deploy ec2-deploy-all \
        ec2-rollback ec2-web-rollback ec2-restart ec2-status ec2-logs ec2-ssh

help: ## Show targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

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

# ---------------------------------------------------------------------------
# EC2 deploy: everything runs on the box that runs the scraper (santimentos dev server).
#   API + Google Maps worker: one Go binary, two systemd services
#   website: Next.js standalone build under systemd; nginx serves peoplvance.org
# See infra/ec2/ and docs/runbooks/ec2.md.
#   first time:  make ec2-setup && make ec2-env && make ec2-deploy-all
#   every time:  make ec2-deploy (API) / make ec2-web-deploy (website) / make ec2-deploy-all
# ---------------------------------------------------------------------------
EC2_PEM      ?= /home/aadesh-kumar/Documents/santimentos/pem-keys/sentimentos-dev.pem
EC2_HOST     ?= ubuntu@3.225.67.242
EC2_ENV_FILE ?= .env.production
EC2_BIN      := bin/peoplvance-api
WEB_TGZ      := bin/peoplvance-web.tgz
SSH_EC2       = ssh -i "$(EC2_PEM)" "$(EC2_HOST)"
SCP_EC2       = scp -i "$(EC2_PEM)"

build-linux: ## Build the linux/amd64 binary for EC2 into bin/peoplvance-api
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(EC2_BIN) ./apps/api
	@ls -lh $(EC2_BIN)

ec2-setup: ## One-time: install systemd units, nginx site and scripts on EC2 (safe to re-run)
	$(SSH_EC2) 'rm -rf ~/peoplvance-setup'
	$(SCP_EC2) -r infra/ec2 "$(EC2_HOST)":~/peoplvance-setup
	$(SSH_EC2) 'bash ~/peoplvance-setup/setup.sh'

ec2-env: ## Upload $(EC2_ENV_FILE) as the server .env (then: make ec2-restart)
	@test -f "$(EC2_ENV_FILE)" || { echo "$(EC2_ENV_FILE) not found - copy infra/ec2/env.example and fill it in" >&2; exit 1; }
	@grep -qE '^APP_ENV=production' "$(EC2_ENV_FILE)" || { echo "$(EC2_ENV_FILE) must set APP_ENV=production" >&2; exit 1; }
	@grep -qE '^DATABASE_URL=.+' "$(EC2_ENV_FILE)" || { echo "$(EC2_ENV_FILE) has no DATABASE_URL" >&2; exit 1; }
	@grep -qE '^DATA_ENCRYPTION_KEY=.+' "$(EC2_ENV_FILE)" || { echo "$(EC2_ENV_FILE) has no DATA_ENCRYPTION_KEY" >&2; exit 1; }
	$(SSH_EC2) 'mkdir -p ~/peoplvance'
	$(SCP_EC2) "$(EC2_ENV_FILE)" "$(EC2_HOST)":~/peoplvance/.env
	$(SSH_EC2) 'chmod 600 ~/peoplvance/.env'
	@echo "Uploaded. Not live until: make ec2-restart"

ec2-scp: build-linux ## Build and copy the binary to EC2 (does not switch to it)
	$(SCP_EC2) $(EC2_BIN) "$(EC2_HOST)":~/peoplvance-api.new
	@echo "Copied to ~/peoplvance-api.new. Switch to it with: make ec2-deploy (or on the box: ~/peoplvance/deploy.sh)"

ec2-deploy: ec2-scp ## Build, copy, migrate and restart the API + worker on EC2
	$(SSH_EC2) 'bash ~/peoplvance/deploy.sh'

build-web: ## Build the website (Next.js standalone) into bin/peoplvance-web.tgz
	cd apps/web && npm ci --no-audit --no-fund && npm run build
	rm -rf apps/web/.next/standalone/.next/static apps/web/.next/standalone/public
	cp -r apps/web/.next/static apps/web/.next/standalone/.next/static
	cp -r apps/web/public apps/web/.next/standalone/public
	@mkdir -p bin
	tar -czf $(WEB_TGZ) -C apps/web/.next/standalone .
	@ls -lh $(WEB_TGZ)

ec2-web-scp: build-web ## Build and copy the website to EC2 (does not switch to it)
	$(SCP_EC2) $(WEB_TGZ) "$(EC2_HOST)":~/peoplvance-web.tgz
	@echo "Copied to ~/peoplvance-web.tgz. Switch with: make ec2-web-deploy (or on the box: ~/peoplvance/deploy-web.sh)"

ec2-web-deploy: ec2-web-scp ## Build, copy and restart the website on EC2
	$(SSH_EC2) 'bash ~/peoplvance/deploy-web.sh'

ec2-deploy-all: ec2-deploy ec2-web-deploy ## Deploy API + worker, then the website

ec2-rollback: ## Switch the API + worker back to the previous binary on EC2
	$(SSH_EC2) 'bash ~/peoplvance/rollback.sh api'

ec2-web-rollback: ## Switch the website back to the previous build on EC2
	$(SSH_EC2) 'bash ~/peoplvance/rollback.sh web'

ec2-restart: ## Restart API, worker and website on EC2 (e.g. after make ec2-env)
	$(SSH_EC2) 'sudo systemctl restart peoplvance-api peoplvance-gmaps-worker peoplvance-web && systemctl is-active peoplvance-api peoplvance-gmaps-worker peoplvance-web'

ec2-status: ## Service status, API readiness and website check on EC2
	$(SSH_EC2) 'systemctl --no-pager --lines=0 status peoplvance-api peoplvance-gmaps-worker peoplvance-web; curl -fsS -m 5 http://127.0.0.1:8090/readyz && echo; curl -fsS -m 5 -o /dev/null -w "web %{http_code}\n" http://127.0.0.1:3100/'

ec2-logs: ## Follow API, worker and website logs on EC2
	$(SSH_EC2) 'tail -n 50 -F ~/peoplvance/api.log ~/peoplvance/gmaps-worker.log ~/peoplvance/web.log'

ec2-ssh: ## SSH into the EC2 box
	$(SSH_EC2)


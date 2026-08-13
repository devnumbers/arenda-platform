COMPOSE_LOCAL := docker compose -f docker-compose.local.yml
COMPOSE_TEST := docker compose -p arenda-test -f docker-compose.test.yml
BACKEND_DIR := apps/backend
FRONTEND_DIR := apps/frontend
ADMIN_DIR := apps/admin
LANDING_DIR := apps/landing
GOLANGCI_LINT_VERSION := v2.12.2
TEST_DATABASE_URL ?= postgres://arenda:arenda@localhost:5435/arenda?sslmode=disable

.PHONY: local-infra-up local-infra-down local-infra-reset \
        test-infra-up test-infra-down \
        backend-test backend-test-integration frontend-test admin-test test \
        backend-run backend-lint backend-tkassa-spec-check check-bruno-coverage check-backend-env check-migrate-env migrate-up migrate-down \
        perf-db-up perf-db-down perf-db-reset perf-backend-run perf-seed perf-sustainable perf-breakdown \
        admin-install admin-dev admin-build admin-typecheck \
        landing-install landing-dev landing-build \
        attributes-install attributes-gen attributes-check

local-infra-up:
	$(COMPOSE_LOCAL) up -d

local-infra-down:
	$(COMPOSE_LOCAL) down

local-infra-reset:
	$(COMPOSE_LOCAL) down -v --remove-orphans

backend-run:
	@test -f .env || (echo "Missing .env" && exit 1)
	@set -a; . ./.env; set +a; $(MAKE) --no-print-directory check-backend-env
	set -a; . ./.env; set +a; go run ./$(BACKEND_DIR)/cmd/api

check-backend-env:
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required in .env" && exit 1)
	@test -n "$$MIGRATIONS_DIR" || (echo "MIGRATIONS_DIR is required in .env" && exit 1)

check-migrate-env:
	@test -f .env || (echo "Missing .env" && exit 1)
	@set -a; . ./.env; set +a; \
		test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required in .env" && exit 1); \
		test -n "$$MIGRATIONS_DIR" || (echo "MIGRATIONS_DIR is required in .env" && exit 1)

backend-lint:
	cd $(BACKEND_DIR) && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --config ../../.golangci.yml --build-tags=integration ./...

# test-infra-up / test-infra-down manage the isolated test database (port 5435,
# compose project arenda-test) so tests never touch the developer's local DB.
# The database starts empty — runtime-skip integration tests and testcontainers
# both apply migrations themselves via database.MigrateUp; pre-applying here
# would leave schema_migrations in a dirty state.
test-infra-up:
	$(COMPOSE_TEST) up -d --wait

test-infra-down:
	$(COMPOSE_TEST) down

# backend-test runs unit tests with -race (mandatory per docs/testing-strategy.md).
# It does not require a database — unit tests use in-memory fakes.
backend-test:
	cd $(BACKEND_DIR) && go test -race ./...

# Integration tests: by default testcontainers-go starts a dedicated PostgreSQL
# 18 container per test binary and applies migrations internally. To use an
# external database instead, run `make test-infra-up` then
# `TEST_DATABASE_URL=... make backend-test-integration`. -race is mandatory per
# docs/testing-strategy.md.
backend-test-integration:
	cd $(BACKEND_DIR) && go test -tags=integration -race ./...

# frontend-test runs the Vitest suite (pure-logic tests, no DOM).
frontend-test:
	cd $(FRONTEND_DIR) && npm run test

# admin-test runs the Vitest suite.
admin-test:
	cd $(ADMIN_DIR) && npm run test

# test runs the full test suite: backend unit + integration + frontend + admin.
# Unit tests run first for fast fail-fast before the slower testcontainers phase.
# It requires Docker: integration tests use testcontainers-go, which starts a
# dedicated PostgreSQL 18 container per test binary and applies migrations
# internally. The test database (docker-compose.test.yml) is reserved for ad-hoc
# runs via TEST_DATABASE_URL + make backend-test-integration when testcontainers
# is unavailable.
test:
	@set -e; \
	$(MAKE) backend-test; \
	$(MAKE) backend-test-integration; \
	$(MAKE) frontend-test; \
	$(MAKE) admin-test

# Regenerates the T-Kassa spec artifacts and fails if regenerating changed
# them, so CI catches a vendored/patched spec whose generated files were not
# committed. Content hashes (git hash-object) are compared instead of
# git status so the check also works on a dirty tree with in-flight spec work;
# on CI's clean checkout a hash change is exactly a git status change.
TKASSA_SPEC_DIR := $(BACKEND_DIR)/internal/billing/adapters/payment/tkassa/spec
ATTRIBUTES_DIR := tools/property-attributes
ATTRIBUTES_ARTIFACTS := \
	apps/backend/internal/properties/domain/zz_catalog.gen.go \
	apps/frontend/features/property-attributes/lib/generated/attr-keys.ts \
	apps/frontend/features/property-attributes/lib/generated/catalog.ts \
	apps/frontend/features/property-attributes/lib/generated/labels.ts \
	apps/frontend/features/property-attributes/lib/generated/validate.ts \
	apps/admin/src/lib/generated/types.ts \
	apps/admin/src/lib/generated/attr-keys.ts \
	apps/admin/src/lib/generated/catalog.ts \
	apps/admin/src/lib/generated/labels.ts \
	apps/admin/src/lib/generated/format.ts

backend-tkassa-spec-check:
	@cd $(TKASSA_SPEC_DIR) && \
	before=$$(git hash-object openapi.patched.yaml spec.gen.go) && \
	./patch.sh && go generate ./... && \
	after=$$(git hash-object openapi.patched.yaml spec.gen.go) && \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: T-Kassa spec generated files are stale — regenerating changed them:"; \
		git -C $(CURDIR) status --porcelain -- $(TKASSA_SPEC_DIR); \
		echo "Run 'cd $(TKASSA_SPEC_DIR) && ./patch.sh && go generate ./...' and commit the regenerated files (openapi.patched.yaml, spec.gen.go)."; \
		exit 1; \
	fi && \
		echo "backend-tkassa-spec-check: spec generated files are fresh"

attributes-install:
	cd $(ATTRIBUTES_DIR) && npm install

# Regenerates the property-attributes catalog artifacts (validate catalog.json
# then emit Go + frontend TS + admin TS). `npm run generate` validates the
# catalog against the schema before writing anything, so the target is
# self-sufficient: validation + generation in one pass.
attributes-gen:
	cd $(ATTRIBUTES_DIR) && npm run generate

# Regenerates the property-attributes catalog artifacts and fails if
# regenerating changed them, so CI catches a catalog.json change whose
# generated files were not committed. Content hashes (git hash-object) are
# compared instead of git status so the check also works on a dirty tree;
# on CI's clean checkout a hash change is exactly a git status change.
# `npm install` runs only when node_modules is missing so the gate stays fast.
# Runs from the repo root (no `cd`) so $(ATTRIBUTES_ARTIFACTS) paths resolve
# correctly; npm is invoked via `--prefix` to target the generator package.
attributes-check:
	@before=$$(git hash-object $(ATTRIBUTES_ARTIFACTS)) && \
	{ [ -d $(ATTRIBUTES_DIR)/node_modules ] || npm --prefix $(ATTRIBUTES_DIR) install; } && \
	npm --prefix $(ATTRIBUTES_DIR) run generate && \
	after=$$(git hash-object $(ATTRIBUTES_ARTIFACTS)) && \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: property-attributes catalog generated files are stale — regenerating changed them:"; \
		git status --porcelain -- $(ATTRIBUTES_ARTIFACTS); \
		echo "Run 'make attributes-gen' (or 'cd $(ATTRIBUTES_DIR) && npm run generate') and commit the regenerated files."; \
		exit 1; \
	fi && \
	echo "attributes-check: catalog generated files are fresh"

admin-install:
	cd $(ADMIN_DIR) && npm install

admin-dev:
	cd $(ADMIN_DIR) && npm run dev

admin-build:
	cd $(ADMIN_DIR) && npm run build

admin-typecheck:
	cd $(ADMIN_DIR) && npm run typecheck

landing-install:
	cd $(LANDING_DIR) && npm install

landing-dev:
	cd $(LANDING_DIR) && npm run dev

landing-build:
	cd $(LANDING_DIR) && npm run build

check-bruno-coverage:
	./tools/e2e/check-bruno-coverage.sh

migrate-up: check-migrate-env
	set -a; . ./.env; set +a; go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 -database "$$DATABASE_URL" -path "$$MIGRATIONS_DIR" up

migrate-down: check-migrate-env
	set -a; . ./.env; set +a; go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 -database "$$DATABASE_URL" -path "$$MIGRATIONS_DIR" down 1

PERF_HTTP_ADDR ?= :8081
PERF_APP_BASE_URL ?= http://127.0.0.1:8081
PERF_DATABASE_URL ?= postgres://arenda:arenda@localhost:5434/arenda?sslmode=disable
PERF_MIGRATIONS_DIR ?= apps/backend/db/migrations
PERF_POSTGRES_PORT ?= 5434
ENDPOINT ?= get_me
ENDPOINT_ARG := $(if $(strip $(ENDPOINT)),-endpoint $(ENDPOINT),)

PERF_ENV_BASE ?= \
	HTTP_ADDR=$(PERF_HTTP_ADDR) \
	APP_BASE_URL=$(PERF_APP_BASE_URL) \
	DATABASE_URL=$(PERF_DATABASE_URL) \
	MIGRATIONS_DIR=$(PERF_MIGRATIONS_DIR) \
	PAYMENT_PROVIDER=fake \
	DADATA_API_KEY=dummy \
	COOKIE_SECURE=false \
	LOG_SUCCESSFUL_REQUESTS=false \
	RATE_LIMIT_IP_RPS=100000 \
	RATE_LIMIT_IP_BURST=100000 \
	RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR=100000 \
	RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN=100000

PERF_ENV ?= $(PERF_ENV_BASE)

COMPOSE_PERF := docker compose -p arenda-perf -f docker-compose.perf.yml

perf-db-up:
	PERF_POSTGRES_PORT=$(PERF_POSTGRES_PORT) $(COMPOSE_PERF) up -d --wait

perf-db-down:
	$(COMPOSE_PERF) down

perf-db-reset:
	$(COMPOSE_PERF) down -v
	PERF_POSTGRES_PORT=$(PERF_POSTGRES_PORT) $(COMPOSE_PERF) up -d --wait

perf-backend-run:
	@mkdir -p .tmp
	go -C $(BACKEND_DIR) build -o ../../.tmp/perf-api ./cmd/api
	$(PERF_ENV) ./.tmp/perf-api

perf-seed:
	$(PERF_ENV) go -C $(BACKEND_DIR) run ./cmd/perfseed

perf-sustainable:
	$(PERF_ENV) go -C $(BACKEND_DIR) run ./cmd/perfvegeta sustainable $(ENDPOINT_ARG)

perf-breakdown:
	$(PERF_ENV) go -C $(BACKEND_DIR) run ./cmd/perfvegeta breakdown $(ENDPOINT_ARG)

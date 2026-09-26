# Dev compose files live in apps/backend next to the module they serve.
# --env-file: compose resolves .env relative to the directory of the first -f
# file (apps/backend), while POSTGRES_* for local infra live in the root .env
# consumed by backend-run/migrate-up — the flag keeps one env source.
# -p: the compose project name is the only isolation mechanism between
# parallel checkouts (containers, networks, volume prefixes). It comes from
# the checkout's own .env slot (docs/agents/parallel-dev.md): a worktree pins
# AREND_SLOT=N and gets project arenda-wtN with its own postgres volume and
# ports; slot 0 (no AREND_SLOT) keeps arenda-local and today's behavior.
# name: in the compose file stays the fallback for calls without .env and -p.
# A slot lookup that fails (broken AREND_SLOT, node missing) aborts the whole
# make run right here: silently falling back to arenda-local would put this
# checkout's local-infra — including `down -v` — into the main checkout's
# compose project, the exact collision the slots exist to prevent.
COMPOSE_PROJECT := $(shell node tools/dev-env/dev-env.mjs compose-project 2>&1 || echo SLOT_LOOKUP_FAILED)
FRONTEND_DEV_PORT := $(shell node tools/dev-env/dev-env.mjs frontend-port 2>&1 || echo SLOT_LOOKUP_FAILED)
ifneq ($(findstring SLOT_LOOKUP_FAILED,$(COMPOSE_PROJECT))$(findstring SLOT_LOOKUP_FAILED,$(FRONTEND_DEV_PORT)),)
$(error dev-env slot lookup failed — fix the reported AREND_SLOT: $(strip $(COMPOSE_PROJECT)) / $(strip $(FRONTEND_DEV_PORT)))
endif
COMPOSE_LOCAL := docker compose --env-file .env -p $(COMPOSE_PROJECT) -f apps/backend/docker-compose.local.yml
COMPOSE_TEST := docker compose -p arenda-test -f apps/backend/docker-compose.test.yml
BACKEND_DIR := apps/backend
FRONTEND_DIR := apps/frontend
ADMIN_DIR := apps/admin
LANDING_DIR := apps/landing

# Language versions — the single source of truth for Go, Node, and the
# golang-migrate CLI across the repo. `make versions-sync` stamps these into
# every file they own (ci.yml, Dockerfiles, go.work/go.mod, .nvmrc); the CI
# versions-freshness job runs `make versions-check` to fail on drift. Bump a
# version here, never in a stamped file. GO_VERSION is a full patch version
# so CI's setup-go (go-version-file: go.mod) and the prod image build on an
# identical toolchain; NODE_VERSION stays major-only so LTS patches float.
GO_VERSION := 1.26.5
NODE_VERSION := 24
MIGRATE_VERSION := v4.19.1

# Tool pins (invoked via go run / npx / release download, never installed
# globally). Kept alongside the language versions they tool around.
GOLANGCI_LINT_VERSION := v2.12.2
GOVULNCHECK_VERSION := v1.7.0
LEFTHOOK_VERSION := v2.1.10
OAPI_CODEGEN_VERSION := v2.7.1
SQLC_VERSION := v1.31.1
# aquasec/trivy 0.74.0, multi-arch manifest digest
TRIVY_IMAGE := aquasec/trivy@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969
TRIVY_CACHE_VOLUME := arenda-trivy-cache
NPM_AUDIT_DIRS := apps/frontend apps/admin apps/landing tools/property-attributes tools/hooks tools/migration-lint tools/nolint-gate tools/suppression-gate tools/dev-env
TOOLS_TEST_DIRS := tools/hooks tools/migration-lint tools/nolint-gate tools/suppression-gate tools/dev-env
KNIP_VERSION := 6.32.2
KNIP_DIRS := apps/frontend apps/admin tools/property-attributes tools/hooks tools/migration-lint tools/nolint-gate tools/suppression-gate tools/dev-env

# golangci-lint caches per-package lint results in one machine-global directory
# by default (os.UserCacheDir()/golangci-lint), shared by every checkout and
# worktree of this repo. Cache keys are content-based, but cached issues embed
# the absolute file paths of the tree that produced them: identical package
# content in two worktrees collides on one entry, and a lint run replays the
# other tree's paths — "lints the old project" (the standing `cache clean`
# workaround). Pin the cache inside the checkout: every worktree gets its own
# cache by construction, and `git worktree remove` drops it with the tree
# (ignored files do not block removal). The fork trims entries unused for
# 5 days, so per-checkout caches stay small. GOLANGCI_LINT_CACHE must be an
# absolute path — $(CURDIR) is.
export GOLANGCI_LINT_CACHE ?= $(CURDIR)/.golangci-lint-cache

TEST_DATABASE_URL ?= postgres://arenda:arenda@localhost:5435/arenda?sslmode=disable
# Named LINT_MIGRATIONS_DIR to stay distinct from the MIGRATIONS_DIR env
# contract checked in check-env (.env.example).
LINT_MIGRATIONS_DIR := apps/backend/db/migrations

# Squawk (migration linter, wave 3 / #309): pinned binary from GitHub Releases.
# Squawk publishes no checksums file, so the release-asset sha256 of each
# supported platform is pinned by hand. The pin variables are deliberately
# lowercase: squawk-install builds the lookup key from the lowercased
# $(SQUAWK_OS)_$(SQUAWK_ARCH) pair, and make variables are case-sensitive
# (uppercase pins made the lookup silently empty — every cold-cache CI run
# died with "no pinned squawk sha256").
SQUAWK_VERSION := v2.62.0
SQUAWK_SHA256_darwin_arm64 := 699d5a2cc6ed622f1469caf4db2faf047d89049b09d89b91d8307238e002d1ac
SQUAWK_SHA256_darwin_x64 := df7c9dfae0acd65c694ac50754cb356ced172d8b118a46261ce14fb70de5136f
SQUAWK_SHA256_linux_arm64 := 561a1ea458082970f485017561d986a930d3863ef7d348d47af6473a0c82bb9a
SQUAWK_SHA256_linux_x64 := 54bd3e7bf2101502317c3400d1043202c51414a646a10f92f56f7b3032630758
SQUAWK_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
SQUAWK_ARCH := $(subst x86_64,x64,$(subst aarch64,arm64,$(shell uname -m)))
SQUAWK_ASSET := squawk-$(SQUAWK_OS)-$(SQUAWK_ARCH)
SQUAWK_BIN := .tmp/squawk/$(SQUAWK_VERSION)/$(SQUAWK_ASSET)
# Squawk looks for .squawk.toml in the cwd; the config lives next to the
# migrations it lints (apps/backend), so every invocation passes it explicitly.
SQUAWK_CONFIG := apps/backend/.squawk.toml

# Codegen artifact locations (freshness-gate inputs and error messages).
TKASSA_SPEC_DIR := $(BACKEND_DIR)/internal/billing/adapters/payment/tkassa/spec
BACKEND_OPENAPI_OUT := $(BACKEND_DIR)/internal/platform/openapi/generated.gen.go
BACKEND_SQLC_OUT_DIR := $(BACKEND_DIR)/internal/platform/generated/postgres
FRONTEND_API_OUT := $(FRONTEND_DIR)/shared/api/generated.ts
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
CATEGORIES_DIR := tools/payment-categories
CATEGORIES_ARTIFACTS := \
	apps/backend/internal/payments/domain/zz_categories.gen.go \
	apps/frontend/features/payment-categories/lib/generated/categories.ts

# Every file versions-sync owns; versions-check hashes these before/after a
# sync run (git hash-object, the freshness-gate idiom).
VERSIONS_SYNC_FILES := .github/workflows/ci.yml go.work apps/backend/go.mod .nvmrc \
	apps/backend/Dockerfile apps/frontend/Dockerfile apps/admin/Dockerfile apps/landing/Dockerfile

DEFAULT_GOAL := help

.PHONY: help versions-sync versions-check \
	local-infra-up local-infra-down local-infra-reset test-infra-up test-infra-down \
	worktree-new \
	backend-run backend-lint backend-vulncheck backend-nolint \
	backend-tkassa-spec-check backend-openapi-check backend-sqlc-check \
	migrate-up migrate-down check-env \
	billing-time-shift billing-tick \
	frontend-install frontend-dev frontend-build frontend-test frontend-api-check frontend-e2e \
	frontend-e2e-headed frontend-e2e-live-up frontend-e2e-live-down \
	admin-install admin-dev admin-build admin-typecheck admin-test \
	landing-install landing-dev landing-build landing-lint landing-typecheck \
	test backend-test backend-test-integration tools-test \
	attributes-gen attributes-check \
	categories-gen categories-check \
	migrations-lint ts-suppressions squawk-install npm-audit trivy-fs knip hooks-install

##@ Help
help: ## List available targets grouped by section
	@awk 'BEGIN {FS = ":.*## "} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)} \
		/^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ Infrastructure
local-infra-up: ## Start the local dev infrastructure (compose, detached)
	$(COMPOSE_LOCAL) up -d

local-infra-down: ## Stop the local dev infrastructure
	$(COMPOSE_LOCAL) down

local-infra-reset: ## Stop local infra and DELETE its volumes (database data)
	$(COMPOSE_LOCAL) down -v --remove-orphans

# test-infra-up / test-infra-down manage the isolated test database (port 5435,
# compose project arenda-test) so tests never touch the developer's local DB.
# The database starts empty — runtime-skip integration tests and testcontainers
# both apply migrations themselves via database.MigrateUp; pre-applying here
# would leave schema_migrations in a dirty state.
test-infra-up: ## Start the isolated test database (port 5435)
	$(COMPOSE_TEST) up -d --wait

test-infra-down: ## Stop the isolated test database
	$(COMPOSE_TEST) down

##@ Backend
backend-run: check-env ## Run the API server locally (go run; requires .env)
	set -a; . ./.env; set +a; go run ./$(BACKEND_DIR)/cmd/api

# Internal prerequisite of backend-run / migrate-up / migrate-down: verifies
# the root .env exists and defines DATABASE_URL + MIGRATIONS_DIR (the
# .env.example contract) before anything tries to use them.
check-env:
	@test -f .env || { echo "Missing .env (copy .env.example)"; exit 1; }; \
	set -a; . ./.env; set +a; \
	test -n "$$DATABASE_URL" || { echo "DATABASE_URL is required in .env"; exit 1; }; \
	test -n "$$MIGRATIONS_DIR" || { echo "MIGRATIONS_DIR is required in .env"; exit 1; }

# The stand-only time-travel rig of the subscription lifecycle (issue #665;
# recipe with the full acceptance walkthrough: docs/billing-time-travel.md).
# Enabled by BILLING_TIME_TRAVEL=true on local/dev/stage only; the admin
# session cookie comes from the /auth/send + /auth/verify login (see recipe).
#   make billing-time-shift USER_ID=<uuid> PRESET=retry_24h_due
#   make billing-time-shift USER_ID=<uuid> HOURS=-25
#   make billing-tick
BASE_URL ?= http://localhost:8080

billing-time-shift: ## Admin time shift of a subscription (USER_ID=, HOURS= or PRESET=, ADMIN_COOKIE=, BASE_URL=)
	@test -n "$(USER_ID)" || { echo "USER_ID is required"; exit 1; }; \
	test -n "$(ADMIN_COOKIE)" || { echo "ADMIN_COOKIE is required (see docs/billing-time-travel.md)"; exit 1; }; \
	test $$( [ -n "$(HOURS)" ] && echo 1 || echo 0 ) -ne $$( [ -n "$(PRESET)" ] && echo 1 || echo 0 ) || \
		{ echo "Set exactly one of HOURS=<int> or PRESET=<period_expired|retry_24h_due|retry_72h_due|reminder_window>"; exit 1; }; \
	if [ -n "$(HOURS)" ]; then body="$$({ printf '{"shiftHours":'; printf '%s' "$(HOURS)" | tr -d ' '; printf '}'; })"; \
	else body="$$({ printf '{"preset":"'; printf '%s' "$(PRESET)"; printf '"}'; })"; fi; \
	curl -sS -X POST "$(BASE_URL)/admin/users/$(USER_ID)/subscription/time-shift" \
		-H "Cookie: $(ADMIN_COOKIE)" -H 'Content-Type: application/json' -d "$$body" \
		-w '\nHTTP %{http_code}\n'

billing-tick: ## Run one billing worker pass now (ADMIN_COOKIE=, BASE_URL=)
	@test -n "$(ADMIN_COOKIE)" || { echo "ADMIN_COOKIE is required (see docs/billing-time-travel.md)"; exit 1; }; \
	curl -sS -X POST "$(BASE_URL)/admin/billing/tick" -H "Cookie: $(ADMIN_COOKIE)" -w '\nHTTP %{http_code}\n'

backend-lint: ## Run golangci-lint over the backend (integration tags included)
	cd $(BACKEND_DIR) && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --build-tags=integration ./...

# govulncheck over the backend, pinned via go run (same pattern as
# backend-lint). Pre-push security gate; CI runs govulncheck separately (ci.yml).
backend-vulncheck: ## Run govulncheck over the backend
	cd $(BACKEND_DIR) && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# Nolint gate (remediation grid #325, gate #344; target state #323): any
# `nolint` in a Go comment under the backend is a finding — zero suppression
# directives, no whitelist (golangci's nolintlint only validates the form of
# directives that exist). Without FILES checks every *.go under $(BACKEND_DIR)
# (CI, Stop-gate); with FILES checks only the listed files (pre-commit:
# make backend-nolint FILES="{staged_files}"); non-.go entries and staged
# deletions are skipped by the filter/the script itself.
backend-nolint: ## Fail on any nolint directive in backend Go comments (FILES= to scope)
	@set +e; status=0; \
	if [ -n "$(FILES)" ]; then \
		files=`echo "$(FILES)" | tr ' ' '\n' | grep '\.go$$' | tr '\n' ' '`; \
		if [ -n "$${files// /}" ]; then \
			echo "==> nolint gate $$files"; \
			node tools/nolint-gate/nolint-gate.mjs $$files || status=1; \
		else \
			echo "==> nolint gate (skip: no .go files in FILES)"; \
		fi; \
	else \
		echo "==> nolint gate $(BACKEND_DIR)/**/*.go"; \
		files=`find $(BACKEND_DIR) -name '*.go' -type f`; \
		node tools/nolint-gate/nolint-gate.mjs $$files || status=1; \
	fi; \
	if [ $$status -ne 0 ]; then echo "ERROR: nolint gate failed (see above)"; exit 1; fi

# Regenerates the T-Kassa spec artifacts and fails if regenerating changed
# them, so CI catches a vendored/patched spec whose generated files were not
# committed. Content hashes (git hash-object) are compared instead of
# git status so the check also works on a dirty tree with in-flight spec work;
# on CI's clean checkout a hash change is exactly a git status change.
backend-tkassa-spec-check: ## Fail if the T-Kassa spec generated files are stale
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

# Regenerates the platform OpenAPI server types (internal/platform/openapi/
# generated.gen.go from api/openapi/openapi.yaml + oapi-codegen.yaml) and
# fails if regenerating changed them, so CI catches a contract or generator
# config change whose generated file was not committed — the platform-spec
# sibling of backend-tkassa-spec-check. Content hashes (git hash-object) are
# compared instead of git status so the check also works on a dirty tree with
# in-flight spec work; on CI's clean checkout a hash change is exactly a git
# status change.
backend-openapi-check: ## Fail if the OpenAPI generated server types are stale
	@cd $(BACKEND_DIR) && \
	before=$$(git hash-object internal/platform/openapi/generated.gen.go) && \
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) \
		-config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml && \
	after=$$(git hash-object internal/platform/openapi/generated.gen.go) && \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: OpenAPI generated code is stale — regenerating changed it:"; \
		git -C $(CURDIR) status --porcelain -- $(BACKEND_OPENAPI_OUT); \
		echo "Run 'cd $(BACKEND_DIR) && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml' and commit the regenerated file ($(BACKEND_OPENAPI_OUT))."; \
		exit 1; \
	fi && \
		echo "backend-openapi-check: OpenAPI generated code is fresh"

# Regenerates the sqlc output (internal/platform/generated/postgres/*.go from
# db/queries + db/migrations + sqlc.yaml) and fails if regenerating changed
# it, so CI catches a query, migration, or config change whose generated code
# was not committed. The artifact list is a runtime shell glob, not a make
# variable, so a .sql.go file added by generate changes the hash list too.
# Content hashes (git hash-object) are compared instead of git status so the
# check also works on a dirty tree.
backend-sqlc-check: ## Fail if the sqlc generated code is stale
	@cd $(BACKEND_DIR) && \
	before=$$(cd internal/platform/generated/postgres && git hash-object *.go) && \
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate && \
	after=$$(cd internal/platform/generated/postgres && git hash-object *.go) && \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: sqlc generated code is stale — regenerating changed it:"; \
		git -C $(CURDIR) status --porcelain -- $(BACKEND_SQLC_OUT_DIR); \
		echo "Run 'cd $(BACKEND_DIR) && go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate' and commit the regenerated files ($(BACKEND_SQLC_OUT_DIR))."; \
		exit 1; \
	fi && \
		echo "backend-sqlc-check: sqlc generated code is fresh"

migrate-up: check-env ## Apply all pending database migrations
	set -a; . ./.env; set +a; go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) -database "$$DATABASE_URL" -path "$$MIGRATIONS_DIR" up

migrate-down: check-env ## Roll back the last applied migration
	set -a; . ./.env; set +a; go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) -database "$$DATABASE_URL" -path "$$MIGRATIONS_DIR" down 1

##@ Frontend
frontend-install: ## Install frontend dependencies (npm install)
	cd $(FRONTEND_DIR) && npm install

# Worktree slots bind their own dev port (3020+N) so parallel frontends don't
# collide (docs/agents/parallel-dev.md); slot 0 passes no flag — Next's
# default 3000. The port comes from the checkout's own .env, evaluated at
# parse time like COMPOSE_PROJECT above.
FRONTEND_DEV_ARGS := $(if $(filter-out 3000,$(FRONTEND_DEV_PORT)),-- -p $(FRONTEND_DEV_PORT),)

frontend-dev: ## Run the frontend dev server (next dev; a worktree slot binds its own port)
	cd $(FRONTEND_DIR) && npm run dev $(FRONTEND_DEV_ARGS)

frontend-build: ## Build the frontend for production (next build)
	cd $(FRONTEND_DIR) && npm run build

# Regenerates the frontend API client (shared/api/generated.ts from the
# backend OpenAPI contract) and fails if regenerating changed it, so CI
# catches an api/openapi/openapi.yaml change whose frontend client was not
# committed. Content hashes (git hash-object) are compared instead of git
# status so the check also works on a dirty tree. `npm install` runs only
# when node_modules is missing so the gate stays fast (attributes-check
# pattern); npm is invoked via `--prefix` to target apps/frontend, which also
# sets the script cwd so its relative paths resolve.
frontend-api-check: ## Fail if the generated frontend API client is stale
	@before=$$(git hash-object $(FRONTEND_API_OUT)) && \
	{ [ -d $(FRONTEND_DIR)/node_modules ] || npm --prefix $(FRONTEND_DIR) install; } && \
	npm --prefix $(FRONTEND_DIR) run generate:api && \
	after=$$(git hash-object $(FRONTEND_API_OUT)) && \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: frontend API client is stale — regenerating changed it:"; \
		git status --porcelain -- $(FRONTEND_API_OUT); \
		echo "Run 'cd $(FRONTEND_DIR) && npm run generate:api' and commit the regenerated file ($(FRONTEND_API_OUT))."; \
		exit 1; \
	fi && \
		echo "frontend-api-check: frontend API client is fresh"

##@ Admin
admin-install: ## Install admin dependencies (npm install)
	cd $(ADMIN_DIR) && npm install

admin-dev: ## Run the admin dev server (vite)
	cd $(ADMIN_DIR) && npm run dev

admin-build: ## Build the admin for production (vite build)
	cd $(ADMIN_DIR) && npm run build

admin-typecheck: ## Typecheck the admin (tsc)
	cd $(ADMIN_DIR) && npm run typecheck

##@ Landing
landing-install: ## Install landing dependencies (npm install)
	cd $(LANDING_DIR) && npm install

landing-dev: ## Run the landing dev server (next dev)
	cd $(LANDING_DIR) && npm run dev

landing-build: ## Build the landing for production (next build)
	cd $(LANDING_DIR) && npm run build

landing-lint: ## Lint the landing (eslint)
	cd $(LANDING_DIR) && npm run lint

landing-typecheck: ## Typecheck the landing (tsc)
	cd $(LANDING_DIR) && npm run typecheck

##@ Tests
# backend-test runs unit tests with -race (mandatory per docs/testing-strategy.md).
# It does not require a database — unit tests use in-memory fakes.
backend-test: ## Run backend unit tests (go test -race; no database needed)
	cd $(BACKEND_DIR) && go test -race ./...

# Integration tests: by default testcontainers-go starts a dedicated PostgreSQL
# 18 container per test binary and applies migrations internally. To use an
# external database instead, run `make test-infra-up` then
# `TEST_DATABASE_URL=... make backend-test-integration`. -race is mandatory per
# docs/testing-strategy.md.
backend-test-integration: ## Run backend integration tests (testcontainers; Docker required)
	cd $(BACKEND_DIR) && go test -tags=integration -race ./...

# frontend-test runs the Vitest suite (pure-logic tests, no DOM).
frontend-test: ## Run the frontend test suite (vitest)
	cd $(FRONTEND_DIR) && npm run test

# frontend-e2e runs the Playwright screen suite (ticket #456): brings up a
# dedicated disposable stack — postgres (its own compose project, port 5436
# on slot 0), the backend (migrations on boot, fake email/payment providers),
# a production standalone build of the frontend — seeds the owner/session/
# properties fixtures, runs apps/frontend/e2e, and tears everything down.
# The targets source the checkout's root .env so a worktree's E2E_* (slot
# ports + compose project) reach the runner via the process env; a plain
# slot-0 .env carries no E2E_* and the runner defaults apply. Requires
# Docker. Docs: docs/testing-strategy.md "Экранные e2e (Playwright)",
# slots: docs/agents/parallel-dev.md.
SOURCE_ROOT_ENV := set -a; [ -f .env ] && . ./.env; set +a;

frontend-e2e: ## Run frontend Playwright e2e (dedicated stack; Docker required)
	$(SOURCE_ROOT_ENV) ./tools/e2e/frontend/run-frontend-e2e.sh

# frontend-e2e-headed replays the specs in visible Chromium windows: the same
# disposable stack, the same fixtures — the runner just shows its work. Pass a
# Playwright filter through TESTS, e.g. make frontend-e2e-headed TESTS="-g платежи".
frontend-e2e-headed: ## Run frontend Playwright e2e with visible browser windows (Docker required)
	$(SOURCE_ROOT_ENV) ./tools/e2e/frontend/run-frontend-e2e.sh --headed $(TESTS)

# frontend-e2e-live-up / -live-down bracket a live UI walkthrough
# (.agents/skills/ui-walkthrough): raise the seeded stack without running
# Playwright and leave it running (E2E_LIVE prints the connection facts), then
# tear it down when the walkthrough ends.
frontend-e2e-live-up: ## Raise the seeded frontend e2e stack for a live walkthrough (no tests run)
	$(SOURCE_ROOT_ENV) E2E_LIVE=1 ./tools/e2e/frontend/run-frontend-e2e.sh

# Teardown mirrors live-up: project and ports come from this checkout's .env
# (a worktree's slot values) with the slot-0 defaults as fallback — no more
# hardcoded arenda-e2e / 3010 / 8081 / 5436, so a walkthrough in one checkout
# never tears down a parallel session's stack.
frontend-e2e-live-down: ## Tear down the live walkthrough stack (this checkout's e2e slot)
	$(SOURCE_ROOT_ENV) \
	docker compose -p "$${E2E_COMPOSE_PROJECT:-arenda-e2e}" -f apps/backend/docker-compose.e2e.yml down -v --remove-orphans || true; \
	for port in "$${E2E_FRONTEND_PORT:-3010}" "$${E2E_BACKEND_PORT:-8081}" "$${E2E_PG_PORT:-5436}"; do \
	  pids="$$(lsof -ti tcp:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	  [ -z "$$pids" ] || kill $$pids 2>/dev/null || true; \
	done

# admin-test runs the Vitest suite.
admin-test: ## Run the admin test suite (vitest)
	cd $(ADMIN_DIR) && npm run test

# Contract tests of the executable tool scripts under tools/ (today: the
# harness hooks in tools/hooks). Every tools/ package with tests carries its
# own package.json with vitest in devDependencies — the tools/property-
# attributes package pattern. Self-sufficient like attributes-check: installs
# node_modules when missing. Every package runs even after a failure (the
# npm-audit style), so one red report doesn't hide the rest.
tools-test: ## Run contract tests of the tools/ packages (self-installing)
	@set -e; status=0; for dir in $(TOOLS_TEST_DIRS); do \
		echo "==> tools-test $$dir"; \
		{ [ -d $$dir/node_modules ] || npm --prefix $$dir install; } && \
		npm --prefix $$dir run test || status=1; \
	done; \
	if [ $$status -ne 0 ]; then echo "ERROR: tools contract tests failed (see above)"; exit 1; fi

# test runs the full test suite: backend unit + integration + frontend + admin.
# Unit tests run first for fast fail-fast before the slower testcontainers phase.
# It requires Docker: integration tests use testcontainers-go, which starts a
# dedicated PostgreSQL 18 container per test binary and applies migrations
# internally. The test database (apps/backend/docker-compose.test.yml) is
# reserved for ad-hoc runs via TEST_DATABASE_URL + make
# backend-test-integration when testcontainers is unavailable.
test: ## Run the full test suite (backend + frontend + admin + landing + tools; Docker required)
	@docker info >/dev/null 2>&1 || { echo "ERROR: Docker is not available, but make test requires it: integration tests start PostgreSQL via testcontainers. Start Docker and retry; to push past the pre-push hook use: git push --no-verify"; exit 1; }
	@set -e; \
	$(MAKE) backend-test; \
	$(MAKE) backend-test-integration; \
	$(MAKE) frontend-test; \
	$(MAKE) admin-test; \
	$(MAKE) landing-lint; \
	$(MAKE) landing-typecheck; \
	$(MAKE) landing-build; \
	$(MAKE) tools-test

# Full suite with output captured to a file: for agents/workflows that gate on
# `make test` programmatically. Reuses the `test` target as-is (same suites, same
# order, same Docker precheck); the recipe prints only the summary (+ tail on
# failure), the verbose log stays in $(LOG_FILE) inside the checkout (*.log is
# gitignored). LOG_FILE is workspace-relative, so `make -C <worktree> test-log`
# writes it into that worktree.
LOG_FILE ?= .make-test.log

test-log: ## Run the full suite via `test`, capturing output to $(LOG_FILE); prints summary + tail (Docker required)
	@rm -f $(LOG_FILE)
	@$(MAKE) --no-print-directory test >> $(LOG_FILE) 2>&1; \
	status=$$?; \
	if [ $$status -eq 0 ]; then \
		echo "make test-log OK (full suite green), log: $(CURDIR)/$(LOG_FILE)"; \
	else \
		echo "make test-log FAILED (exit $$status), full log: $(CURDIR)/$(LOG_FILE), tail:"; \
		tail -60 $(LOG_FILE); \
	fi; \
	exit $$status

##@ Code generation
# Regenerates the property-attributes catalog artifacts (validate catalog.json
# then emit Go + frontend TS + admin TS). `npm run generate` validates the
# catalog against the schema before writing anything, so the target is
# self-sufficient: validation + generation in one pass; node_modules is
# installed when missing (attributes-check pattern).
attributes-gen: ## Regenerate the property-attributes catalog artifacts
	@{ [ -d $(ATTRIBUTES_DIR)/node_modules ] || npm --prefix $(ATTRIBUTES_DIR) install; } && \
	npm --prefix $(ATTRIBUTES_DIR) run generate

# Regenerates the property-attributes catalog artifacts and fails if
# regenerating changed them, so CI catches a catalog.json change whose
# generated files were not committed. Content hashes (git hash-object) are
# compared instead of git status so the check also works on a dirty tree;
# on CI's clean checkout a hash change is exactly a git status change.
# `npm install` runs only when node_modules is missing so the gate stays fast.
# Runs from the repo root (no `cd`) so $(ATTRIBUTES_ARTIFACTS) paths resolve
# correctly; npm is invoked via `--prefix` to target the generator package.
attributes-check: ## Fail if the property-attributes catalog artifacts are stale
	@before=$$(git hash-object $(ATTRIBUTES_ARTIFACTS)) && \
	{ [ -d $(ATTRIBUTES_DIR)/node_modules ] || npm --prefix $(ATTRIBUTES_DIR) install; } && \
	npm --prefix $(ATTRIBUTES_DIR) run generate && \
	after=$$(git hash-object $(ATTRIBUTES_ARTIFACTS)) && \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: property-attributes catalog generated files are stale — regenerating changed them:"; \
		git status --porcelain -- $(ATTRIBUTES_ARTIFACTS); \
		echo "Run 'make attributes-gen' and commit the regenerated files."; \
		exit 1; \
	fi && \
		echo "attributes-check: catalog generated files are fresh"

# Regenerates the payment-categories catalog artifacts (validate catalog.json
# then emit Go + frontend TS). Same self-sufficient pattern as attributes-gen:
# `npm run generate` validates the catalog (incl. the icon-asset existence
# check) before writing anything; node_modules is installed when missing.
categories-gen: ## Regenerate the payment-categories catalog artifacts
	@{ [ -d $(CATEGORIES_DIR)/node_modules ] || npm --prefix $(CATEGORIES_DIR) install; } && \
	npm --prefix $(CATEGORIES_DIR) run generate

# Payment-categories freshness gate (attributes-check pattern): regenerate and
# compare content hashes (git hash-object) so a catalog.json change whose
# generated files were not committed fails in CI.
categories-check: ## Fail if the payment-categories catalog artifacts are stale
	@before=$$(git hash-object $(CATEGORIES_ARTIFACTS)) && \
	{ [ -d $(CATEGORIES_DIR)/node_modules ] || npm --prefix $(CATEGORIES_DIR) install; } && \
	npm --prefix $(CATEGORIES_DIR) run generate && \
	after=$$(git hash-object $(CATEGORIES_ARTIFACTS)) && \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: payment-categories catalog generated files are stale — regenerating changed them:"; \
		git status --porcelain -- $(CATEGORIES_ARTIFACTS); \
		echo "Run 'make categories-gen' and commit the regenerated files."; \
		exit 1; \
	fi && \
		echo "categories-check: catalog generated files are fresh"

##@ Quality and security
# TS suppression gate (quality mode #378, gate #399): an eslint-disable
# comment, a @ts-ignore/@ts-expect-error/@ts-nocheck, or an explicit `any` in
# the manual TS/JS code of apps/frontend + apps/admin is a finding — zero
# suppressions, no whitelist. Generated code, build artifacts and node_modules
# are excluded by the script itself (the counter looks only at hand-written
# code: .next/types alone carries 20 `any` and 102 `@ts-ignore`). Without
# FILES checks every source file under both apps (CI, Stop-gate); with FILES
# checks only the listed files (pre-commit: make ts-suppressions
# FILES="{staged_files}"); non-source entries and staged deletions are skipped
# by the filter/the script itself.
ts-suppressions: ## Fail on any suppression in manual frontend/admin TS/JS code (FILES= to scope)
	@set +e; status=0; \
	if [ -n "$(FILES)" ]; then \
		files=`echo "$(FILES)" | tr ' ' '\n' | grep -E '\.(ts|tsx|js|jsx|mjs|cjs|mts|cts)$$' | tr '\n' ' '`; \
		if [ -n "$${files// /}" ]; then \
			echo "==> suppression gate $$files"; \
			node tools/suppression-gate/suppression-gate.mjs $$files || status=1; \
		else \
			echo "==> suppression gate (skip: no source files in FILES)"; \
		fi; \
	else \
		echo "==> suppression gate apps/frontend + apps/admin manual sources"; \
		files=`find apps/frontend apps/admin -type f \( -name '*.ts' -o -name '*.tsx' -o -name '*.js' -o -name '*.jsx' -o -name '*.mjs' -o -name '*.cjs' -o -name '*.mts' -o -name '*.cts' \) -not -path '*/node_modules/*' -not -path '*/.next/*' -not -path '*/dist/*'`; \
		node tools/suppression-gate/suppression-gate.mjs $$files || status=1; \
	fi; \
	if [ $$status -ne 0 ]; then echo "ERROR: suppression gate failed (see above)"; exit 1; fi

# Migration lint (wave 3 / #309): squawk (lock-safety, config
# apps/backend/.squawk.toml) + the domain rules in
# tools/migration-lint/domain-rules.mjs
# (money is BIGINT kopecks; no DEFAULT on id). Without FILES lints every
# migration under $(LINT_MIGRATIONS_DIR) (pre-push, CI); with FILES lints only
# the listed files (pre-commit: make migrations-lint FILES="{staged_files}").
#
# FILES-mode squawk quirks handled here (both make squawk exit 1 with
# "Failed to find files", breaking the commit): *.down.sql files are filtered
# out (repo-level exemption, see .squawk.toml excluded_paths), and a list whose
# up files are ALL individually exempt in .squawk.toml is a skip, not a
# failure. A listed path that does not exist is still a hard error. The
# domain-rules pass handles down files itself (skips them with a note).
migrations-lint: squawk-install ## Lint migrations: squawk + domain rules (FILES= to scope)
	@set +e; status=0; \
	if [ -n "$(FILES)" ]; then \
		files="$(FILES)"; \
		up_files=`echo $$files | tr ' ' '\n' | grep -v '\.down\.sql$$' | tr '\n' ' '`; \
		if [ -n "$${up_files// /}" ]; then \
			for f in $$up_files; do \
				[ -f "$$f" ] || { echo "ERROR: migration file not found: $$f"; exit 1; }; \
			done; \
			echo "==> squawk $$up_files"; \
			out=`$(SQUAWK_BIN) -c $(SQUAWK_CONFIG) $$up_files 2>&1`; \
			if [ $$? -eq 0 ]; then \
				echo "$$out"; \
			elif echo "$$out" | grep -q "Failed to find files"; then \
				echo "==> squawk (skip: every listed up migration is exempt in .squawk.toml)"; \
			else \
				echo "$$out"; status=1; \
			fi; \
		else \
			echo "==> squawk (skip: only down migrations, which .squawk.toml exempts)"; \
		fi; \
		echo "==> migration domain rules $$files"; \
		node tools/migration-lint/domain-rules.mjs $$files || status=1; \
	else \
		echo "==> squawk $(LINT_MIGRATIONS_DIR)/*.sql"; \
		$(SQUAWK_BIN) -c $(SQUAWK_CONFIG) $(LINT_MIGRATIONS_DIR)/*.sql || status=1; \
		echo "==> migration domain rules $(LINT_MIGRATIONS_DIR)/*.sql"; \
		node tools/migration-lint/domain-rules.mjs $(LINT_MIGRATIONS_DIR)/*.sql || status=1; \
	fi; \
	if [ $$status -ne 0 ]; then echo "ERROR: migration lint failed (see above)"; exit 1; fi

# Installs the pinned squawk binary (migration linter, wave 3 / #309) from
# GitHub Releases into .tmp (gitignored) when missing, verifying the pinned
# sha256 of the release asset. Idempotent; called automatically by
# migrations-lint.
squawk-install:
	@if [ -x "$(SQUAWK_BIN)" ]; then exit 0; fi; \
	expected="$(SQUAWK_SHA256_$(SQUAWK_OS)_$(SQUAWK_ARCH))"; \
	if [ -z "$$expected" ]; then echo "ERROR: no pinned squawk sha256 for $(SQUAWK_OS)-$(SQUAWK_ARCH)"; exit 1; fi; \
	mkdir -p .tmp/squawk/$(SQUAWK_VERSION); \
	echo "==> downloading squawk $(SQUAWK_VERSION) ($(SQUAWK_OS)-$(SQUAWK_ARCH))"; \
	curl -fsSL -o $(SQUAWK_BIN) https://github.com/sbdchd/squawk/releases/download/$(SQUAWK_VERSION)/$(SQUAWK_ASSET) || exit 1; \
	actual=$$({ sha256sum $(SQUAWK_BIN) 2>/dev/null || shasum -a 256 $(SQUAWK_BIN); } | cut -d' ' -f1); \
	if [ "$$actual" != "$$expected" ]; then \
		echo "ERROR: squawk checksum mismatch (expected $$expected, got $$actual)"; \
		rm -f $(SQUAWK_BIN); \
		exit 1; \
	fi; \
	chmod +x $(SQUAWK_BIN)

# npm audit (--audit-level=high) across the repo's lockfile packages
# (tools/screenshots, a local playwright utility, stays out of the gate).
# Every package runs even after a failure, so one red report doesn't hide the
# rest.
npm-audit: ## Run npm audit (high+) across all lockfile packages; 3 попытки на каталог — bulk-endpoint реестра моргает ETIMEDOUT'ом
	@set -e; status=0; for dir in $(NPM_AUDIT_DIRS); do \
		echo "==> npm audit $$dir"; \
		ok=0; \
		for attempt in 1 2 3; do \
			if (cd $$dir && npm audit --audit-level=high); then ok=1; break; fi; \
			echo "==> npm audit $$dir failed (attempt $$attempt/3)"; \
			sleep 3; \
		done; \
		if [ $$ok -ne 1 ]; then status=1; fi; \
	done; \
	if [ $$status -ne 0 ]; then echo "ERROR: npm audit found advisories (see above)"; exit 1; fi

# Filesystem vuln scan over the repo root via the pinned trivy image — mirrors
# the CI trivy-fs job (scanners: vuln, severity HIGH/CRITICAL, ignore-unfixed,
# exit 1). node_modules, .git, and .tmp are skipped: they are never the shipped
# dependency set and would dominate scan time. The vuln DB is cached in a named
# docker volume so repeat runs don't re-download it.
trivy-fs: ## Scan the repo filesystem with trivy (vuln, HIGH/CRITICAL); .worktrees skipped — локальные чекауты, CI их не видит
	docker run --rm \
		-v "$(CURDIR):/repo" \
		-v $(TRIVY_CACHE_VOLUME):/root/.cache \
		$(TRIVY_IMAGE) fs /repo \
		--scanners vuln \
		--severity HIGH,CRITICAL \
		--ignore-unfixed \
		--exit-code 1 \
		--skip-dirs node_modules \
		--skip-dirs .git \
		--skip-dirs .tmp \
		--skip-dirs .worktrees

# Advisory dead-code/unused-exports/unused-dependencies report (knip, decision
# #295 / issue #305). NOT a gate: --no-exit-code keeps the run green on
# findings — the printed report is the signal; only an infrastructure failure
# (npx download, broken config) fails. Promoting knip to a blocking gate is a
# separate decision once the per-package knip.json configs stabilize
# (docs/agents/tooling.md). Monorepo mode requires a root package.json the
# repo deliberately doesn't have, so each package runs standalone; apps/landing
# is out of scope — маленький маркетинговый одностраничник, где сигнал
# dead-code не окупает настройку (решение карты #888, тикет T1).
# Self-sufficient like tools-test: installs
# node_modules when missing (knip resolves imports through them).
knip: ## Print the advisory dead-code report (knip; not a gate)
	@set -e; status=0; for dir in $(KNIP_DIRS); do \
		echo "==> knip $$dir"; \
		{ [ -d $$dir/node_modules ] || npm --prefix $$dir install; } && \
		npx --yes knip@$(KNIP_VERSION) --directory $$dir --no-exit-code || status=1; \
	done; \
	if [ $$status -ne 0 ]; then echo "ERROR: knip run failed (infrastructure, not findings — see above)"; exit 1; fi

##@ Versions
# Stamps GO_VERSION / NODE_VERSION / MIGRATE_VERSION into every file they own:
# ci.yml (node-version, the golang:…-bookworm test containers, the migrate
# pin), the four Dockerfiles, the go directive of go.work + apps/backend/go.mod,
# and .nvmrc. Idempotent — unchanged variables rewrite identical bytes.
# perl -pi instead of sed -i: BSD and GNU sed take incompatible in-place flags.
versions-sync: ## Stamp GO/NODE/MIGRATE versions into all pinned files
	perl -pi -e 's/^(\s*)node-version: .*/$${1}node-version: $(NODE_VERSION)/' .github/workflows/ci.yml
	perl -pi -e 's|migrate\@v[0-9.]+|migrate\@$(MIGRATE_VERSION)|g' .github/workflows/ci.yml
	perl -pi -e 's|FROM node:[0-9.]+-alpine|FROM node:$(NODE_VERSION)-alpine|' apps/frontend/Dockerfile apps/admin/Dockerfile apps/landing/Dockerfile
	perl -pi -e 's|FROM golang:[0-9.]+-bookworm|FROM golang:$(GO_VERSION)-bookworm|' apps/backend/Dockerfile
	perl -pi -e 's|golang:[0-9.]+-bookworm|golang:$(GO_VERSION)-bookworm|g' .github/workflows/ci.yml
	perl -pi -e 's/^go .*/go $(GO_VERSION)/' go.work apps/backend/go.mod
	printf '%s\n' '$(NODE_VERSION)' > .nvmrc

# Freshness gate for the version pins — the sibling of the codegen freshness
# gates (backend-openapi-check & co): re-runs versions-sync and compares
# content hashes before/after (git hash-object). A file stamped at a different
# version than the Makefile variables fails with the fix command. Runs in CI
# as the versions-freshness job.
versions-check: ## Fail if any file drifted from the Makefile version pins
	@before=$$(git hash-object $(VERSIONS_SYNC_FILES) 2>/dev/null); \
	$(MAKE) --no-print-directory versions-sync; \
	after=$$(git hash-object $(VERSIONS_SYNC_FILES)); \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: version pins are stale — versions-sync changed files:"; \
		git status --porcelain -- $(VERSIONS_SYNC_FILES); \
		echo "Run 'make versions-sync' and commit the result."; \
		exit 1; \
	fi; \
	echo "versions-check: version pins are fresh"

##@ Worktrees
# Creates .worktrees/<WT> on a new branch with a generated slot env (its own
# ports and compose projects — docs/agents/parallel-dev.md). Deliberately
# minimal: dependencies, baseline tests and infra startup are the
# using-git-worktrees skill's side of the recipe. Only on explicit request
# (repo rule: no worktrees by default).
worktree-new: ## Create a worktree with its own slot env (WT=<name>; branch = name)
	node tools/dev-env/worktree-new.mjs "$(WT)"

##@ Setup
# Installs the pinned lefthook binary when missing, then wires the git hooks
# (lefthook install rewrites .git/hooks entries managed by lefthook — idempotent,
# safe to re-run after cloning or when lefthook.yml changes).
hooks-install: ## Install the pinned lefthook binary and wire git hooks
	@gobin=$$(go env GOPATH)/bin; \
	if ! command -v lefthook >/dev/null 2>&1 && [ ! -x "$$gobin/lefthook" ]; then \
		echo "lefthook not found — installing pinned $(LEFTHOOK_VERSION) via go install"; \
		go install github.com/evilmartians/lefthook/v2@$(LEFTHOOK_VERSION) || exit 1; \
	fi; \
	if ! command -v lefthook >/dev/null 2>&1; then \
		case ":$$PATH:" in *":$$gobin:"*) ;; *) echo "NOTE: $$gobin is not on PATH — add it to run lefthook commands directly; the git hooks work regardless (their shim falls back to the absolute binary path)";; esac; \
	fi
	@PATH="$$(go env GOPATH)/bin:$$PATH" lefthook install

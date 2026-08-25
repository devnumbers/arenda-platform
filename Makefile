# Dev compose files live in apps/backend next to the module they serve.
# --env-file: compose resolves .env relative to the directory of the first -f
# file (apps/backend), while POSTGRES_* for local infra live in the root .env
# consumed by backend-run/migrate-up — the flag keeps one env source.
COMPOSE_LOCAL := docker compose --env-file .env -f apps/backend/docker-compose.local.yml
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
NPM_AUDIT_DIRS := apps/frontend apps/admin apps/landing tools/property-attributes tools/hooks tools/migration-lint tools/nolint-gate tools/suppression-gate
TOOLS_TEST_DIRS := tools/hooks tools/migration-lint tools/nolint-gate tools/suppression-gate
KNIP_VERSION := 6.32.2
KNIP_DIRS := apps/frontend apps/admin tools/property-attributes tools/hooks tools/migration-lint tools/nolint-gate tools/suppression-gate
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
	backend-run backend-lint backend-vulncheck backend-nolint \
	backend-tkassa-spec-check backend-openapi-check backend-sqlc-check \
	migrate-up migrate-down check-env \
	frontend-install frontend-dev frontend-build frontend-test frontend-api-check \
	admin-install admin-dev admin-build admin-typecheck admin-test \
	landing-install landing-dev landing-build \
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

frontend-dev: ## Run the frontend dev server (next dev)
	cd $(FRONTEND_DIR) && npm run dev

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

landing-dev: ## Run the landing dev server (vite)
	cd $(LANDING_DIR) && npm run dev

landing-build: ## Build the landing for production (vite build)
	cd $(LANDING_DIR) && npm run build

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
test: ## Run the full test suite (backend + frontend + admin + tools; Docker required)
	@docker info >/dev/null 2>&1 || { echo "ERROR: Docker is not available, but make test requires it: integration tests start PostgreSQL via testcontainers. Start Docker and retry; to push past the pre-push hook use: git push --no-verify"; exit 1; }
	@set -e; \
	$(MAKE) backend-test; \
	$(MAKE) backend-test-integration; \
	$(MAKE) frontend-test; \
	$(MAKE) admin-test; \
	$(MAKE) tools-test

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
npm-audit: ## Run npm audit (high+) across all lockfile packages
	@set -e; status=0; for dir in $(NPM_AUDIT_DIRS); do \
		echo "==> npm audit $$dir"; \
		(cd $$dir && npm audit --audit-level=high) || status=1; \
	done; \
	if [ $$status -ne 0 ]; then echo "ERROR: npm audit found advisories (see above)"; exit 1; fi

# Filesystem vuln scan over the repo root via the pinned trivy image — mirrors
# the CI trivy-fs job (scanners: vuln, severity HIGH/CRITICAL, ignore-unfixed,
# exit 1). node_modules, .git, and .tmp are skipped: they are never the shipped
# dependency set and would dominate scan time. The vuln DB is cached in a named
# docker volume so repeat runs don't re-download it.
trivy-fs: ## Scan the repo filesystem with trivy (vuln, HIGH/CRITICAL)
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
		--skip-dirs .tmp

# Advisory dead-code/unused-exports/unused-dependencies report (knip, decision
# #295 / issue #305). NOT a gate: --no-exit-code keeps the run green on
# findings — the printed report is the signal; only an infrastructure failure
# (npx download, broken config) fails. Promoting knip to a blocking gate is a
# separate decision once the per-package knip.json configs stabilize
# (docs/agents/tooling.md). Monorepo mode requires a root package.json the
# repo deliberately doesn't have, so each package runs standalone; apps/landing
# is out of scope — a Figma Make export whose template ui-library makes the
# dead-code signal non-actionable. Self-sufficient like tools-test: installs
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

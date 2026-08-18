COMPOSE_LOCAL := docker compose -f docker-compose.local.yml
COMPOSE_TEST := docker compose -p arenda-test -f docker-compose.test.yml
BACKEND_DIR := apps/backend
FRONTEND_DIR := apps/frontend
ADMIN_DIR := apps/admin
LANDING_DIR := apps/landing
GOLANGCI_LINT_VERSION := v2.12.2
GOVULNCHECK_VERSION := v1.7.0
LEFTHOOK_VERSION := v2.1.10
OAPI_CODEGEN_VERSION := v2.7.1
SQLC_VERSION := v1.31.1
# aquasec/trivy 0.74.0, multi-arch manifest digest
TRIVY_IMAGE := aquasec/trivy@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969
TRIVY_CACHE_VOLUME := arenda-trivy-cache
NPM_AUDIT_DIRS := apps/frontend apps/admin apps/landing tools/property-attributes tools/hooks tools/migration-lint tools/nolint-gate
TOOLS_TEST_DIRS := tools/hooks tools/migration-lint tools/nolint-gate
KNIP_VERSION := 6.32.2
KNIP_DIRS := apps/frontend apps/admin tools/property-attributes tools/hooks tools/migration-lint tools/nolint-gate
TEST_DATABASE_URL ?= postgres://arenda:arenda@localhost:5435/arenda?sslmode=disable
# Named LINT_MIGRATIONS_DIR to stay distinct from the MIGRATIONS_DIR env
# contract checked in check-backend-env / check-migrate-env (.env.example).
LINT_MIGRATIONS_DIR := apps/backend/db/migrations
# Squawk (migration linter, wave 3 / #309): pinned binary from GitHub Releases.
# Squawk publishes no checksums file, so the release-asset sha256 of each
# supported platform is pinned by hand.
SQUAWK_VERSION := v2.62.0
SQUAWK_SHA256_DARWIN_ARM64 := 699d5a2cc6ed622f1469caf4db2faf047d89049b09d89b91d8307238e002d1ac
SQUAWK_SHA256_DARWIN_X64 := df7c9dfae0acd65c694ac50754cb356ced172d8b118a46261ce14fb70de5136f
SQUAWK_SHA256_LINUX_ARM64 := 561a1ea458082970f485017561d986a930d3863ef7d348d47af6473a0c82bb9a
SQUAWK_SHA256_LINUX_X64 := 54bd3e7bf2101502317c3400d1043202c51414a646a10f92f56f7b3032630758
SQUAWK_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
SQUAWK_ARCH := $(subst x86_64,x64,$(subst aarch64,arm64,$(shell uname -m)))
SQUAWK_ASSET := squawk-$(SQUAWK_OS)-$(SQUAWK_ARCH)
SQUAWK_BIN := .tmp/squawk/$(SQUAWK_VERSION)/$(SQUAWK_ASSET)

.PHONY: local-infra-up local-infra-down local-infra-reset \
        test-infra-up test-infra-down \
        backend-test backend-test-integration frontend-test admin-test test \
        backend-run backend-lint backend-tkassa-spec-check backend-openapi-check backend-sqlc-check frontend-api-check \
        check-bruno-coverage check-backend-env check-migrate-env migrate-up migrate-down \
        perf-db-up perf-db-down perf-db-reset perf-backend-run perf-seed perf-sustainable perf-breakdown \
        admin-install admin-dev admin-build admin-typecheck \
        landing-install landing-dev landing-build \
        attributes-install attributes-gen attributes-check \
        hooks-install backend-vulncheck npm-audit trivy-fs tools-test knip \
        squawk-install migrations-lint backend-nolint

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
	@docker info >/dev/null 2>&1 || { echo "ERROR: Docker is not available, but make test requires it: integration tests start PostgreSQL via testcontainers. Start Docker and retry; to push past the pre-push hook use: git push --no-verify"; exit 1; }
	@set -e; \
	$(MAKE) backend-test; \
	$(MAKE) backend-test-integration; \
	$(MAKE) frontend-test; \
	$(MAKE) admin-test; \
	$(MAKE) tools-test

# Contract tests of the executable tool scripts under tools/ (today: the
# harness hooks in tools/hooks). Every tools/ package with tests carries its
# own package.json with vitest in devDependencies — the tools/property-
# attributes package pattern. Self-sufficient like attributes-check: installs
# node_modules when missing. Every package runs even after a failure (the
# npm-audit style), so one red report doesn't hide the rest.
tools-test:
	@set -e; status=0; for dir in $(TOOLS_TEST_DIRS); do \
		echo "==> tools-test $$dir"; \
		{ [ -d $$dir/node_modules ] || npm --prefix $$dir install; } && \
		npm --prefix $$dir run test || status=1; \
	done; \
	if [ $$status -ne 0 ]; then echo "ERROR: tools contract tests failed (see above)"; exit 1; fi

# Installs the pinned lefthook binary when missing, then wires the git hooks
# (lefthook install rewrites .git/hooks entries managed by lefthook — idempotent,
# safe to re-run after cloning or when lefthook.yml changes).
hooks-install:
	@gobin=$$(go env GOPATH)/bin; \
	if ! command -v lefthook >/dev/null 2>&1 && [ ! -x "$$gobin/lefthook" ]; then \
		echo "lefthook not found — installing pinned $(LEFTHOOK_VERSION) via go install"; \
		go install github.com/evilmartians/lefthook/v2@$(LEFTHOOK_VERSION) || exit 1; \
	fi; \
	if ! command -v lefthook >/dev/null 2>&1; then \
		case ":$$PATH:" in *":$$gobin:"*) ;; *) echo "NOTE: $$gobin is not on PATH — add it to run lefthook commands directly; the git hooks work regardless (their shim falls back to the absolute binary path)";; esac; \
	fi
	@PATH="$$(go env GOPATH)/bin:$$PATH" lefthook install

# govulncheck over the backend, pinned via go run (same pattern as
# backend-lint). Pre-push security gate; CI runs govulncheck separately (ci.yml).
backend-vulncheck:
	cd $(BACKEND_DIR) && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# npm audit (--audit-level=high) across the repo's lockfile packages
# (tools/screenshots, a local playwright utility, stays out of the gate).
# Every package runs even after a failure, so one red report doesn't hide the
# rest.
npm-audit:
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
trivy-fs:
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

# Migration lint (wave 3 / #309): squawk (lock-safety, config .squawk.toml at
# the repo root) + the domain rules in tools/migration-lint/domain-rules.mjs
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
migrations-lint: squawk-install
	@set +e; status=0; \
	if [ -n "$(FILES)" ]; then \
		files="$(FILES)"; \
		up_files=`echo $$files | tr ' ' '\n' | grep -v '\.down\.sql$$' | tr '\n' ' '`; \
		if [ -n "$${up_files// /}" ]; then \
			for f in $$up_files; do \
				[ -f "$$f" ] || { echo "ERROR: migration file not found: $$f"; exit 1; }; \
			done; \
			echo "==> squawk $$up_files"; \
			out=`$(SQUAWK_BIN) $$up_files 2>&1`; \
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
		$(SQUAWK_BIN) $(LINT_MIGRATIONS_DIR)/*.sql || status=1; \
		echo "==> migration domain rules $(LINT_MIGRATIONS_DIR)/*.sql"; \
		node tools/migration-lint/domain-rules.mjs $(LINT_MIGRATIONS_DIR)/*.sql || status=1; \
	fi; \
	if [ $$status -ne 0 ]; then echo "ERROR: migration lint failed (see above)"; exit 1; fi

# Nolint gate (remediation grid #325, gate #344; target state #323): any
# `nolint` in a Go comment under the backend is a finding — zero suppression
# directives, no whitelist (golangci's nolintlint only validates the form of
# directives that exist). Without FILES checks every *.go under $(BACKEND_DIR)
# (CI, Stop-gate); with FILES checks only the listed files (pre-commit:
# make backend-nolint FILES="{staged_files}"); non-.go entries and staged
# deletions are skipped by the filter/the script itself.
backend-nolint:
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
knip:
	@set -e; status=0; for dir in $(KNIP_DIRS); do \
		echo "==> knip $$dir"; \
		{ [ -d $$dir/node_modules ] || npm --prefix $$dir install; } && \
		npx --yes knip@$(KNIP_VERSION) --directory $$dir --no-exit-code || status=1; \
	done; \
	if [ $$status -ne 0 ]; then echo "ERROR: knip run failed (infrastructure, not findings — see above)"; exit 1; fi

# Regenerates the T-Kassa spec artifacts and fails if regenerating changed
# them, so CI catches a vendored/patched spec whose generated files were not
# committed. Content hashes (git hash-object) are compared instead of
# git status so the check also works on a dirty tree with in-flight spec work;
# on CI's clean checkout a hash change is exactly a git status change.
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

# Regenerates the platform OpenAPI server types (internal/platform/openapi/
# generated.gen.go from api/openapi/openapi.yaml + oapi-codegen.yaml) and
# fails if regenerating changed them, so CI catches a contract or generator
# config change whose generated file was not committed — the platform-spec
# sibling of backend-tkassa-spec-check. Content hashes (git hash-object) are
# compared instead of git status so the check also works on a dirty tree with
# in-flight spec work; on CI's clean checkout a hash change is exactly a git
# status change.
backend-openapi-check:
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
backend-sqlc-check:
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

# Regenerates the frontend API client (shared/api/generated.ts from the
# backend OpenAPI contract) and fails if regenerating changed it, so CI
# catches an api/openapi/openapi.yaml change whose frontend client was not
# committed. Content hashes (git hash-object) are compared instead of git
# status so the check also works on a dirty tree. `npm install` runs only
# when node_modules is missing so the gate stays fast (attributes-check
# pattern); npm is invoked via `--prefix` to target apps/frontend, which also
# sets the script cwd so its relative paths resolve.
frontend-api-check:
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

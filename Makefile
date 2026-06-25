COMPOSE_LOCAL := docker compose -f docker-compose.local.yml
BACKEND_DIR := apps/backend
GOLANGCI_LINT_VERSION := v2.12.2

.PHONY: local-infra-up local-infra-down local-infra-reset backend-run backend-lint check-bruno-coverage check-backend-env check-migrate-env migrate-up migrate-down \
        perf-db-up perf-db-down perf-db-reset perf-backend-run perf-seed perf-sustainable perf-breakdown

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
	cd $(BACKEND_DIR) && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --config ../../.golangci.yml ./...

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
	SMS_SENDER=fake \
	PAYMENT_PROVIDER=fake \
	DADATA_API_KEY=dummy \
	COOKIE_SECURE=false \
	LOG_SUCCESSFUL_REQUESTS=false \
	RATE_LIMIT_IP_RPS=100000 \
	RATE_LIMIT_IP_BURST=100000 \
	RATE_LIMIT_PHONE_SEND_PER_HOUR=100000 \
	RATE_LIMIT_PHONE_VERIFY_PER_15MIN=100000

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

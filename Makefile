COMPOSE_LOCAL := docker compose -f docker-compose.local.yml
BACKEND_DIR := apps/backend
GOLANGCI_LINT_VERSION := v2.12.2

.PHONY: local-infra-up local-infra-down local-infra-reset backend-run backend-lint check-backend-env check-migrate-env migrate-up migrate-down bruno-run

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

migrate-up: check-migrate-env
	set -a; . ./.env; set +a; go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 -database "$$DATABASE_URL" -path "$$MIGRATIONS_DIR" up

migrate-down: check-migrate-env
	set -a; . ./.env; set +a; go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 -database "$$DATABASE_URL" -path "$$MIGRATIONS_DIR" down 1
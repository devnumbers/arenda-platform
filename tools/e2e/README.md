# E2E tests

This folder contains the unified end-to-end test suite for the Arenda backend.

## What's inside

```
tools/e2e/
├── run-e2e-with-db-checks.sh       # main orchestrator
├── README.md                        # this file
├── sql/                             # SQL verification checks
└── bruno/
    └── arenda-api-e2e/              # Bruno E2E collections
        ├── environments/            # Local environment (copy)
        ├── system-e2e/              # happy-path system scenario
        └── system-e2e-edge/         # negative and boundary tests
```

Manual API request collections live separately in `tools/bruno/arenda-api/`.

## Prerequisites

- Docker and local infrastructure running:
  ```bash
  make local-infra-up
  ```
- Backend running on `localhost:8080`:
  ```bash
  make backend-run
  ```
- `.env` filled from `.env.example` with `PAYMENT_PROVIDER=fake` and `APP_ENV=local`.
- Tools installed:
  - [Bruno CLI](https://docs.usebruno.com/bru-cli/overview) (`npm install -g @usebruno/cli`)
  - `jq`
  - `curl`
  - `docker`

## Run the full suite

From the repository root:

```bash
./tools/e2e/run-e2e-with-db-checks.sh
```

The runner will:
1. Check backend and PostgreSQL health.
2. Send a fake SMS code and extract it from the backend log.
3. Run `system-e2e` sequentially with SQL checks after each folder.
4. Run `system-e2e-edge` (expected non-2xx responses are counted separately).
5. Run random-user lifecycles and concurrency/race scenarios.
6. Write reports to:
   - `.tmp/e2e-report-*.md`
   - `.tmp/e2e-bugs-*.md`

## Run a single Bruno folder manually

```bash
cd tools/e2e/bruno/arenda-api-e2e
bru run system-e2e/70-reminders -r \
  --env Local \
  --env-var skipSend=true \
  --env-var code="123456"
```

## Structure of `system-e2e`

Folders are numbered and run in order:

1. `00-auth-setup`
2. `10-subscription`
3. `20-property-lifecycle`
4. `30-tenant-contacts`
5. `40-leases`
6. `50-operations`
7. `60-recurring-operations`
8. `70-reminders`
9. `80-readonly-recovery`
10. `85-webhooks`
11. `99-final-cleanup`

See `bruno/arenda-api-e2e/system-e2e/README.md` for details.

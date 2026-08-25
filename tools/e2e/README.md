# E2E tests

This folder contains the unified end-to-end test suite for the Arenda backend.

## What's inside

```
tools/e2e/
├── run-e2e-with-db-checks.sh       # main orchestrator
├── README.md                        # this file
├── sql/                             # SQL verification checks
├── frontend/                        # Playwright e2e of the frontend screens
│   ├── run-frontend-e2e.sh          # `make frontend-e2e` orchestrator
│   ├── seed.sql                     # deterministic seed (owner, session, properties)
│   └── e2e-crypto.mjs               # seed crypto: token HMAC, phone ciphertext
└── bruno/
    └── arenda-api-e2e/              # Bruno E2E collections
        ├── environments/            # Local environment (copy)
        ├── system-e2e/              # happy-path system scenario
        └── system-e2e-edge/         # negative and boundary tests
```

The frontend screen suite itself (specs and fixtures) lives with the app in
`apps/frontend/e2e`; see `frontend/README.md` here and the «Экранные e2e
(Playwright)» section of `docs/testing-strategy.md`.

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
1. Run the coverage gate to ensure every public backend endpoint is covered by
   at least one Bruno request.
2. Check backend and PostgreSQL health.
3. Send a login code and extract it from the backend log (the dev fake email
   sender logs the message body).
4. Run `system-e2e` sequentially with SQL checks after each folder.
5. Run `system-e2e-edge` (expected non-2xx responses are counted separately).
6. Run random-user lifecycles and concurrency/race scenarios.
7. Write reports to:
   - `.tmp/e2e-report-*.md`
   - `.tmp/e2e-bugs-*.md`

Test phone numbers are randomized per run so repeated local executions don't
reuse leftover users/subscriptions from earlier runs.

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
9. `75-webhooks` — fake provider payment webhook (succeeded + invalid body). A pending payment
   for the positive webhook is created by the runner, which queries Postgres for `provider_payment_id`
   and passes both ids into the folder as environment variables.
10. `80-readonly-recovery`
11. `99-final-cleanup`

See `bruno/arenda-api-e2e/system-e2e/README.md` for details.

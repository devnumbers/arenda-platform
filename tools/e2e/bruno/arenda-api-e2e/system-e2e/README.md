# System E2E collection

This Bruno collection exercises the entire Arenda backend API from registration to
subscription management, property lifecycle, leases, tenant contacts, operations,
recurring operations, reminders, webhooks, and readonly recovery.

It now also includes the deep reminder and subscription coverage that previously
lived in separate `reminders-e2e` and `subscriptions-e2e` collections.

## Run order

The folders are numbered so they run sequentially when invoked recursively:

1. `00-auth-setup` — send/verify phone code, fetch current user.
2. `10-subscription` — upgrade to Pro, add/activate payment methods, confirm fake payment.
3. `20-property-lifecycle` — property CRUD, archive/unarchive, tariff-limit check.
4. `30-tenant-contacts` — tenant contact CRUD and duplicate-phone guard.
5. `40-leases` — future/active leases, updates, completion, duplicate/archived/dates negatives.
6. `50-operations` — income/expense operations, update, delete, validation negatives.
7. `60-recurring-operations` — recurring operation lifecycle and generated concrete operations.
8. `70-reminders` — operation/lease/recurring reminders, pagination, lifecycle rescheduling/cancellation.
9. `75-webhooks` — fake provider payment webhook (succeeded + invalid body). A pending payment
   for the positive webhook is created by the runner, which queries Postgres for `provider_payment_id`
   and passes both ids into the folder as environment variables.
10. `80-readonly-recovery` — cancel subscription, verify mutation block, recover to Business.
11. `99-final-cleanup` — downgrade, remove payment methods, logout.

## Requirements

- `PAYMENT_PROVIDER=fake` and `APP_ENV=local` in `.env`.
- Local infrastructure is running (`make local-infra-up`).
- Backend is running on `localhost:8080` (`make backend-run`).
- `.env` is filled in (see `.env.example`).
- Bruno CLI (`bru`) is installed: `npm install -g @usebruno/cli`.

## Automated run

From the repository root run the unified E2E orchestrator:

```bash
./tools/e2e/run-e2e-with-db-checks.sh
```

The script:
- sends a fake SMS code and extracts it from the backend logs,
- runs this `system-e2e` collection folder by folder,
- runs the `system-e2e-edge` negative/boundary collection,
- executes SQL verification checks,
- runs random-user and concurrency tests,
- writes reports to `.tmp/e2e-report-*.md` and `.tmp/e2e-bugs-*.md`.

## Manual run of a single folder

1. Start the backend: `make backend-run`.
2. Send a code to the test phone and read the code from the backend logs.
3. Run the folder, telling Bruno the code has already been sent:
   ```bash
   cd tools/e2e/bruno/arenda-api-e2e
   bru run system-e2e/70-reminders -r \
     --env Local \
     --env-var skipSend=true \
     --env-var code="$code"
   ```

   Without `skipSend=true`, the first request sends a fresh code. Run it within
   one minute to avoid the 1-minute rate limit.

## Notes

- All resource names use `Date.now()` to avoid collisions between runs.
- IDs captured in `script:post-response` are stored as Bruno runtime vars; some are
  also stored as env vars when they cross module boundaries.
- The auth setup extracts `session_id` (and `cookieName`) from the `Set-Cookie`
  header so the rest of the collection is authenticated.

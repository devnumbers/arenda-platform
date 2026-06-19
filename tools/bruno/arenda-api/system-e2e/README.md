# System E2E collection

This Bruno collection exercises the entire Arenda backend API from registration to
subscription management, property lifecycle, leases, tenant contacts, operations,
recurring operations, reminders, and readonly recovery.

## Run order

The folders are numbered so they run sequentially when invoked recursively:

1. `00-auth-setup` — send/verify phone code, fetch current user.
2. `10-subscription` — upgrade to Pro, add/activate payment method, confirm fake payment.
3. `20-property-lifecycle` — property CRUD, archive/unarchive, tariff-limit check.
4. `30-tenant-contacts` — tenant contact CRUD and duplicate-phone guard.
5. `40-leases` — future/active leases, updates, completion, duplicate/archived/dates negatives.
6. `50-operations` — income/expense operations, update, delete, validation negatives.
7. `60-recurring-operations` — recurring operation lifecycle and generated concrete operations.
8. `70-reminders` — operation reminders CRUD and past-date validation.
9. `80-readonly-recovery` — cancel subscription, verify mutation block, recover to Business.
10. `99-final-cleanup` — downgrade, remove payment methods, logout.

## Requirements

- `PAYMENT_PROVIDER=fake` and `APP_ENV=local` in `.env`.
- Local infrastructure is running (`make local-infra-up`).
- `.env` is filled in (see `.env.example`).
- Bruno CLI (`bru`) is installed: `npm install -g @usebruno/cli`.

## Automated run

From the repository root:

```bash
./tools/bruno/run-system-e2e.sh
```

The script starts the backend, extracts the fake SMS code from logs, and runs the
collection with a unique phone number.

## Manual run

1. Start the backend: `make backend-run`.
2. Send a code to the test phone and read the code from the backend logs (or use
   the runner from the previous section, which handles this step).
3. Set the code in your environment:
   ```bash
   export code=123456
   ```
4. Run the collection, telling it the code has already been sent:
   ```bash
   bru run tools/bruno/arenda-api/system-e2e -r \
     --env Local \
     --env-var skipSend=true \
     --env-var code="$code"
   ```

   Without `skipSend=true`, the collection sends a fresh code itself. In that
   case run it within one minute of sending to avoid the 1-minute rate limit.

## Notes

- All resource names use `Date.now()` to avoid collisions between runs.
- IDs captured in `script:post-response` are stored as Bruno runtime vars; some are
  also stored as env vars when they cross module boundaries.
- The auth setup extracts `session_id` (and `cookieName`) from the `Set-Cookie`
  header so the rest of the collection is authenticated.

# Subscription E2E Collection

This Bruno collection exercises the subscription/tariff/payment flow using the fake payment provider.

## Requirements

- Backend running locally with:
  - `PAYMENT_PROVIDER=fake`
  - `APP_ENV=local` (so `/internal/*` endpoints are available)
- A valid `session_id` cookie in the active Bruno environment.

## Flow

1. **Setup** — upgrade to Pro, which creates a pending fake payment.
2. **Positive** — confirm the fake payment, verify the subscription is Pro, list tariffs/payments/methods, schedule a downgrade to Basic, and switch the active payment method.
3. **Negative** — attempt to delete the active method, submit an invalid period, and confirm a non-existent payment.
4. **Cleanup** — disable auto-renew and remove the now-inactive first payment method.

## Readonly recovery path

The recovery endpoints (`/tariffs`, `/subscription`, `/subscription/change`, `/subscription/auto-renew`, `/subscription/payment-methods`, `/internal/fake-subscription-payment/*`) are intentionally exempt from readonly middleware. To fully verify that a blocked/cancelled subscription can still call these endpoints, manually set the user's subscription status to `blocked` or `cancelled` in the database, then re-run the recovery requests.

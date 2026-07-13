# ADR 0015: Login for Registered Users Without the Email Step

## Status

Accepted

## Context

The login/registration UX used to be a single flow: phone → email → code sent
to the email address. The email step was shown to everyone, including already
registered users whose email is stored in their profile. The goal of this
change is to remove the redundant step for registered users: after entering
the phone number, the code must be sent to the stored email immediately,
without re-entering the address.

## Decision

`POST /auth/send` and `POST /auth/verify` accept an optional email.

`POST /auth/send` without email:

- the user is found by phone and has an email → the code is sent to the stored
  email, response `200 {sent: true, retryAfter: 60}`;
- the user is not found or has no email → response `200 {sent: false}`, and the
  client shows the email input step (the classic registration path).

`POST /auth/verify` without email: the email is resolved from `users` by phone
number, `hashCode` stays consistent — a code sent to the stored email passes
verification without explicitly passing the address.

The code entry step shows neutral text without disclosing the email.

### User enumeration

The `/auth/send` response inherently reveals whether a phone number is
registered (`sent: true/false`). This is a deliberate tradeoff, inseparable
from the target UX: to skip the email step, the client must know whether the
number is registered. Mitigations:

- IP rate limit (20 rps);
- phone-keyed bucket on code sending;
- service-level 60-second cooldown (`minSendInterval`) per phone+email pair;
- verify attempt lockout (15 failures / 30 minutes).

## Alternatives

- **(a) A separate check-phone endpoint.** Rejected: two round-trips instead
  of one, which hurts UX and doubles the load on auth.
- **(b) Uniform response without the `sent` flag.** Rejected: incompatible
  with the target UX — the client cannot tell whether to show the email input
  step.
- **(c) Masked email hint on the code step** (e.g. `a***@d***.ru`). Rejected
  for privacy reasons: the email is not disclosed to the client at all.

## Consequences

- The OpenAPI contract was updated: email became optional in `SendCodeRequest`
  and `VerifyCodeRequest`, the `sent` field was added to `SendCodeResponse`,
  and a `409` response was added to `/auth/send`.
- The classic path with an explicit email is unchanged; the
  `409 ErrEmailDoesNotMatch` error is preserved — an explicitly passed email
  must still match the stored one.
- (+) Registered users log in one step faster.
- (~) The fact that a number is registered is revealed via `sent`; the risk is
  compensated by rate limiting, the send cooldown, and the verify attempt
  lockout.

## See also

- ADR 0004: cookie sessions (`0004-cookie-sessions.md`).
- ADR 0013: in-memory rate limiters (`0013-in-memory-rate-limiters.md`).
- Entity description: `docs/entities/polzovatel.md`.

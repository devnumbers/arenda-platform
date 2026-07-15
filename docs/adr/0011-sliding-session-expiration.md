# ADR 0011: Sliding Session Expiration

## Status

Accepted

## Context

Sessions are opaque server-side tokens stored in PostgreSQL and delivered through `HttpOnly` cookies (ADR 0004). The original implementation used a fixed 30-day lifetime. A stolen cookie would therefore remain valid for a full month.

We need a balance between convenience (users should stay logged in while actively using the app) and risk reduction (limiting the window of opportunity if a cookie is compromised).

## Decision

Use a sliding session expiration:

- A session starts with a base lifetime of 7 days.
- Every authenticated request refreshes the session expiration to `now + base lifetime`, up to a hard maximum of 30 days since the session was created.
- A dedicated endpoint allows the user to revoke all of their sessions at once.

The session token itself, the hashing scheme, and the cookie attributes (`HttpOnly`, `Secure`, `__Host-` prefix in production) remain unchanged. The cookie was originally kept at `SameSite=Strict`; this was later changed to `SameSite=Lax` — see ADR 0018.

## Consequences

- Active users stay signed in without re-entering the SMS code.
- A stolen cookie is usable only until the session expires; because expiration is refreshed on activity, the effective compromise window is bounded by the base lifetime unless the attacker can keep the session active.
- The maximum lifetime guarantees that even an attacker who can continuously refresh the session cannot extend it beyond 30 days from the original login.
- Each authenticated request may update the session row in PostgreSQL; this is acceptable for the expected MVP load.

# ADR 0056: Pure Sliding Sessions with Compensating Hardening

## Status

Accepted (amends ADR 0011 and ADR 0018; both remain authoritative for everything not amended here)

## Context

ADR 0011 introduced sliding session expiration: a 7-day base TTL refreshed on every authenticated request, capped at a 30-day absolute maximum since creation. The cap bounded the usable lifetime of a stolen cookie even when an attacker kept the session active.

The product decision for the devices-and-sessions effort (map #724, grilling #727) is **pure sliding without an absolute cap**: an actively using owner must never be logged out. A record-keeping platform without money movement (ADR 0036) accepts a lower risk profile than banking, and OWASP itself allows long-lived sessions «when it is an application requirement» provided compensating controls. The same grilling rejected access+refresh tokens (both would live in the same HttpOnly cookie — no gain; revisit only for mobile apps or a public API) and rejected double-submit CSRF tokens.

Removing the cap removes the only control that limits a hijacked, actively-refreshed cookie. That control must be replaced, not just dropped.

## Decision

**1. The cap is removed.** `Session.Refresh` extends the expiration to `now + 7 days` whenever that moves it forward; there is no `SessionMaxTTL`. An active session never expires; an idle one dies within 7 days.

**2. Token rotation every 14 days (renewal timeout).** The OWASP-recommended compensation for long-lived sessions: when `now - rotated_at ≥ 14 days`, the session token is replaced — a fresh 256-bit token, a new HMAC hash, the old hash moved to `previous_token_hash`, `rotated_at` restarted. The swap is a single guarded `UPDATE` run through the Unit-of-Work (ADR 0033): a concurrent rotation of the same session makes the loser a no-op, and exactly one swap wins per window.

**Grace window.** For 2 minutes after a rotation the previous token hash is still accepted on lookup (`GetSessionByTokenHash` matches `previous_token_hash` while `rotated_at` is fresh). In-flight requests carrying the old cookie complete instead of dying mid-rotation; logout within the window kills the row by either hash. There is no state-changing GET endpoint (ADR 0018), so a stale GET cannot mutate anything.

**3. Devices list as the user-facing detection loop.** Every session carries the client description captured once at creation (raw User-Agent, device type, browser + major, OS), the last client IP and its GeoIP city (ADR 0057), and the last-activity stamp (throttled: at most one write per minute per session). The API exposes `GET /me/sessions` (with the current session flagged), `DELETE /me/sessions/{sessionId}` (a foreign session; the current one is refused — logout ends it) and `POST /me/sessions/logout-others`. This is OWASP's «visibility and remote termination of active sessions» — the primary user-level detector of a hijack that the cap no longer bounds.

**4. Second CSRF layer beside SameSite=Lax.** `net/http.CrossOriginProtection` (stdlib, Go 1.25+) rejects non-safe cross-origin browser requests via Fetch Metadata / Origin, with insecure bypasses for the two non-browser paths: the T-Kassa payment webhooks (`POST /webhooks/`) and the internal performance diagnostics (`/internal/perf/`). Additionally `DecodeJSONBody` requires `Content-Type: application/json` (an empty body without a declared type stays tolerated for the optional-body endpoints) — a cross-site form physically cannot send JSON. Double-submit tokens stay rejected: origin verification is the required OWASP mitigation, tokens would add state without adding protection for a same-origin SPA.

**5. Audit of revocations.** `auth.session_revoked` and `auth.other_sessions_revoked` entries record who, when, and from which IP (requestctx via the audit recorder). Unlike logout (the documented fail-open exception), revocations keep the ADR 0020 default: the entry and the delete share one transaction, and an audit failure rolls the revocation back.

## Consequences

- (+) An active owner is never logged out; a stolen cookie can no longer be kept alive indefinitely *by the user-visible detection loop* — every rotation invalidates it within 14 days even under constant attacker activity, because a rotated token cannot be re-derived by the thief.
- (+) The rotation and its grace window are atomic and race-safe by construction (one guarded UPDATE).
- (+) CSRF now has defense in depth (Lax + origin verification + JSON content type) instead of a single layer.
- (-) A hijacker keeping the session active lives up to 14 days per stolen token instead of 30; the residual window is compensated by the devices list and audit, not by time.
- (-) One extra table read per lookup miss path (the grace branch of `GetSessionByTokenHash`) and one `UPDATE` per rotation per 14 days — negligible.
- (-) The migration (000131) deletes every existing session once; all users re-login (map #724 decision «б» — accepted).
- (-) The frontend must follow the new cookie on a rotation response; the cookie is re-issued server-side, so a plain fetch-based SPA needs no extra handling.
- The `SameSite=Lax` decision of ADR 0018 stands unchanged; the «no state-changing GETs» rule remains mandatory.

## See also

- [`0011-sliding-session-expiration.md`](./0011-sliding-session-expiration.md) — the base sliding mechanism; its absolute cap is removed here.
- [`0018-samesite-lax.md`](./0018-samesite-lax.md) — SameSite=Lax stands; this ADR adds the second layer.
- [`0033-unit-of-work-transactional-seam.md`](./0033-unit-of-work-transactional-seam.md) — the rotation runs through the UoW.
- [`0020-audit-log.md`](./0020-audit-log.md) — revocation audit follows the fail-safe default.
- [`0057-city-geolocation-in-app.md`](./0057-city-geolocation-in-app.md) — where the session city is resolved.

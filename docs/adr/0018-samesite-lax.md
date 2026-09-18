# ADR 0018: SameSite=Lax for the Session Cookie

## Status

Accepted (CSRF hardening extended by ADR 0056: CrossOriginProtection + Content-Type check alongside SameSite=Lax)

## Context

In production the session cookie was set with `SameSite=Strict` (ADR 0011). With `Strict`, the browser withholds the cookie on any cross-site navigation, including the very first top-level navigation that brings a user to the site: opening a link from a messenger, restoring a browser tab, or following a bookmark from an external context. An authenticated user arriving this way is seen by the backend as anonymous and is redirected to `/login`, even though their session is still valid. The effect is especially visible on mobile devices, where most entries come from messengers and restored tabs.

A refresh of the page immediately after landing would send the cookie (the navigation becomes same-site), but by then the user has already landed on the login form, which reads as a lost session.

## Decision

Set the session cookie to `SameSite=Lax` in all environments, including production. The conditional "Lax locally, Strict when `Secure`" logic in `setSessionCookie`/`clearSessionCookie` is removed; the cookie is always written with `http.SameSiteLaxMode`.

All other cookie attributes remain unchanged: `HttpOnly`, `Secure` in production, `Path=/`, expiration/`MaxAge`, and the `__Host-` prefix in production (which keeps the cookie locked to the exact host and requires `Secure`).

This ADR amends the `SameSite=Strict` statement in ADR 0011; the rest of ADR 0011 (sliding expiration, token hashing) still stands.

## Consequences

- Authenticated users arriving via cross-site top-level navigations (messengers, restored tabs, external links) keep their session and no longer hit a spurious `/login` redirect.
- CSRF analysis: `Lax` causes the browser to send the cookie only on top-level navigations with safe (GET) methods. Cross-site `POST`/`fetch`/XHR requests, form submissions, and subresource or iframe loads still do not carry the cookie, so the classic CSRF vectors remain blocked.
- The API is consumed same-origin by the frontend, so it does not rely on cross-site credentialed requests; `Lax` does not weaken any cross-origin API scenario.
- There are no state-changing `GET` endpoints in the API; `/auth/logout` is a `POST`. Therefore the only requests that newly receive the cookie cross-site (top-level GET navigations) cannot mutate state.
- Residual risk compared to `Strict`: a cross-site top-level GET navigation now carries the session cookie, so any future state-changing GET endpoint would become CSRF-exploitable. The "no state-changing GETs" rule must be preserved in API design.

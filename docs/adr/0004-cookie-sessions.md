# ADR 0004: Use Server-Side Cookie Sessions

## Status

Accepted

## Context

The MVP is a browser web application. Sessions must be revocable on logout and admin subscription block.

## Decision

Use opaque server-side sessions stored in PostgreSQL and sent to the browser through an `HttpOnly` cookie.

## Consequences

The browser stores only a random session token. Session state and revocation remain server-controlled.

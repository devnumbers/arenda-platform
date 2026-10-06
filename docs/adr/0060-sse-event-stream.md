# ADR 0060: Shared User Event Stream — SSE

The platform has one long-lived, per-user event stream — `GET
/notifications/stream` (Server-Sent Events) — instead of per-feature
polling (карта #734, ticket #742). The notifications context is its first
publisher (`notification.created`, `notification.unread_count`, pushed by
the delivery pipeline post-commit); the shared-access map #714 rewires as a
consumer of the same transport.

## Status

Accepted (amended 2026-09-25: the §5 event-name sketch for map #714 —
`access.updated` and friends — is superseded by
[ADR 0062](./0062-realtime-entity-events.md): those events ride the
dedicated `GET /realtime/stream` as the single `entity.changed` name, and
the notifications stream contract stays frozen; everything else in this
ADR stands). Implements the realtime axis of the map #734 charter decision
(«SSE — общий пользовательский стрим»), based on research #736
(`docs/research/2026-09-17-notifications-sse.md`). Complements
[ADR 0059](./0059-notification-delivery-queue-river.md) (the delivery
pipeline whose commit the stream follows) and
[ADR 0014](./0014-in-memory-event-dispatcher.md) (the in-process domain
dispatcher, which stays as-is — the stream hub is a different primitive).

## Context

The frontend learns about new notifications today only by polling react-query
keys. Live toasts, unread badges and the future realtime access/history of
map #714 all need a push channel. The facts that shaped the decision:

- one backend instance per environment (owner-accepted, `docs/deployment.md`);
  the in-process limitation is documented in ADR 0014 with horizontal
  scaling as the trigger to revisit;
- the API runs behind Caddy (`deploy/caddy/rentlee.caddy`), which flushes
  `text/event-stream` responses immediately and buffers them only through the
  site-level `encode`;
- `http.Server.WriteTimeout` was 30s — a hard write deadline that would kill
  every stream in every environment;
- auth is the cookie session via `SessionMiddleware`; there is no CORS
  (same-origin only).

## Decision

1. **SSE, not WebSockets.** The transport is one-directional (server →
   client); every mutation goes through the regular API. `EventSource`
   gives reconnection, the `retry` hint and the `Last-Event-ID` cursor out
   of the box, over plain HTTP that Caddy already proxies. No polyfill: the
   target browsers (PWA on iOS 16.4+, Android Chrome) support SSE fully.

2. **In-memory hub, one instance.** `internal/platform/sse.Hub` keeps the
   registry of open connections (max 8 per user — tabs/devices; the oldest
   is displaced) and fans frames out non-blocking: a connection whose buffer
   (32 frames) overflows is a slow consumer — dropped and closed, it
   reconnects and re-reads through the API. Memory per connection is tens of
   KB; the load picture is observable as `sse.connections` and
   `sse.frames.dropped` in Uptrace.

3. **WriteTimeout: 0, honestly.** The 30s write deadline could not be lifted
   per-connection: `http.ResponseController.SetWriteDeadline` needs
   `Unwrap`/`SetWriteDeadline` forwarding through the middleware chain, and
   the otelhttp wrapper (httpsnoop) hides the server writer. Lifting the
   global deadline moves the protection to where it still holds:
   `ReadHeaderTimeout` 5s and `ReadTimeout` 10s (the slow-request vector),
   `IdleTimeout` 120s, bounded API payloads behind a buffering Caddy, and
   the stream's own lifecycle — a 25s heartbeat whose write error surfaces
   dead connections and a 1h connection TTL that forces re-authentication.

4. **Graceful shutdown closes the hub first.** On the lifecycle
   cancellation the hub closes every connection's channel, so the stream
   handlers return immediately and the 5s `server.Shutdown` window is not
   burned by long-lived handlers. Clients reconnect with the `retry: 3000`
   hint and re-read state on open — the 5–15s deploy gap is invisible.

5. **Envelope v1, coarse event names.** Every frame: `id` (the hub's
   monotonic sequence — a replay cursor for a future v2 buffer), `event`
   (stable name: `connected`, `notification.created`,
   `notification.unread_count`; the map #714 sketch — `access.updated` and
   friends — is superseded by
   [ADR 0062](./0062-realtime-entity-events.md): those events ride
   `GET /realtime/stream` as the single `entity.changed` name; unknown
   names are ignored by old clients, so adding is backward
   compatible), `data` — one line of JSON `{v, occurredAt, payload}`.
   Payloads carry ids and display fields only; clients re-read state through
   the API — the stream never becomes a second system of record.

6. **Publication hook in the pipeline, post-commit.** The `Publisher` (#740)
   pushes `notification.created` + `notification.unread_count` strictly
   after its transaction commits, through the `application.StreamPublisher`
   port implemented by the `adapters/stream` formatter over the hub. The
   transport is best-effort: a failed unread count drops only the badge
   frame, and nothing stream-related can fail a publication.

7. **Auth is the existing session middleware.** The route is an ordinary
   authenticated path (contract-first in `openapi.yaml`): no actor → 401
   problem+json before the stream starts. A session checked at connect is
   not re-checked for the connection's lifetime — accepted because the
   stream is read-only triggers; the 1h TTL bounds the window.

8. **Replay is not implemented (v1).** `Last-Event-ID` is logged, not
   served. The consumer contract compensates: on open (including every
   reconnect) the frontend invalidates its react-query keys. A ring-buffer
   replay per user is the pre-planned v2 if map #714 needs it.

9. **The N-replica upgrade path is LISTEN/NOTIFY, not sticky sessions.**
   When a second API instance appears, `Hub.Publish` is replaced by a
   Postgres `NOTIFY` bridge (dedicated pgx connection, payload < 8000
   bytes) with the same local fan-out; the `Subscribe` surface does not
   change. Sticky balancing is not needed for a stateless API.

10. **Caddy: the stream is excluded from compression.** The site-level
    `encode zstd gzip` matches `text/*` — including `text/event-stream` —
    and would start buffering frames after 512 bytes; the fragment matches
    `not header Content-Type text/event-stream` instead. Nothing else in the
    proxy needs a change.

## Consequences

- (+) Live toasts, badges and future map #714 features ride one transport
  with one auth story; adding an event is a name and a payload, no new
  infrastructure.
- (+) Delivery stays exactly as ADR 0059 defined it — the stream adds a
  post-commit push, not a second delivery path; the feed row remains the
  system of record.
- (−) `WriteTimeout: 0` applies to every response, not only the stream; the
  mitigations above are recorded here so the risk is a known decision, not
  an accident.
- (−) Streams are in-memory per instance: they do not survive deploys and
  do not work across replicas — the reconnect/re-read contract and the
  LISTEN/NOTIFY bridge are the documented answers.
- (=) The hub closes on shutdown — the deploy gap stays inside the
  documented 5–15s.

## See also

- `docs/research/2026-09-17-notifications-sse.md` — full research #736
  (spec links, Caddy/Next specifics, timeout analysis, front-end provider).
- `apps/backend/internal/notifications/GLOSSARY.md` — the stream vocabulary
  (Хаб, Envelope, event names, limits).

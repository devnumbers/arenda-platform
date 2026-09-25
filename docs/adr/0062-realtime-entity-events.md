# ADR 0062: Realtime Entity Events — the Invalidation Stream

Map #714: members of shared properties see each other's changes without a
page reload. Grilling ticket #715 (owner decisions, 2026-09-25) fixes the
contract: a second per-user SSE endpoint `GET /realtime/stream` over the
existing transport of [ADR 0060](./0060-sse-event-stream.md), coarse
invalidation frames (`entity.changed`, no entity data), and carrier-style
post-commit publication from the mutation seams of every entity-owning
context.

## Status

Accepted (grilling #715 of map #714). Complements
[ADR 0060](./0060-sse-event-stream.md) — the shared transport (hub, frame
writer, heartbeat, TTL, envelope, Caddy exclusion) is reused as built;
nothing in ADR 0060 changes and the notifications stream keeps its endpoint
and contract untouched. ADR 0060 §5 sketched map #714's events as new event
names on the notifications stream; this ADR supersedes that sketch — the
events ride a dedicated endpoint instead, so the notifications stream
contract stays frozen. The §2 precise-consumer extension (ticket #718) is
amended in place with the code that introduced it. The §2 owner-book
null-propertyId frames (`InOwnerBook`) are amended in place likewise.

## Context

The frontend learns about other participants' changes only by refetching on
navigation; shared-property members see stale screens until they reload.
The transport for live pushes exists since map #734 (ADR 0060):
`internal/platform/sse` is a per-user hub (max 8 connections per user, 25s
heartbeat, 1h TTL, slow-consumer eviction), its docblock names this map as
the next consumer, and the frontend has the full client precedent
(streaming Next route handler with `Last-Event-ID` passthrough, EventSource
provider with manual backoff). The map charter (2026-09-15) sketched a
"stream per object" before the hub was built. Three post-commit publication
precedents exist in mutation pipelines: grace_events (#284), tariff_events
(#752) and the notifications `StreamPublisher` port (#742); the history
journal (#707, ADR 0061) already records manual actions in-transaction
across the same pipelines.

## Decision

1. **Per-user second stream, owned by a new small context.**
   `internal/realtime` owns `GET /realtime/stream` (chi,
   `SessionMiddleware`) and its Next route handler
   `/api/realtime/stream` — the catch-all proxy's 30s timeout kills
   streams, so the dedicated-handler shape of the notifications route
   repeats (same headers, `Last-Event-ID` passthrough). The client opens
   one extra EventSource per tab; the hub's 8-connections-per-user budget
   covers both streams. Rejected: per-property endpoints
   (`GET /properties/{id}/events`) — the per-user hub is built, and
   per-property means one connection per object per screen plus a parallel
   subscription topology; merging into the notifications stream — endpoint
   ownership stays with the notifications context, and realtime
   invalidation must not follow notification delivery's lifecycle.

2. **Coarse frames, one event name, dictionary of eight entities.** The
   stream speaks one event name, `entity.changed`, in the envelope v1
   (`{v, occurredAt, payload}`, ADR 0060 §5); the payload is
   `{propertyId, entity}`. `propertyId` is null for the owner-book
   mutations outside any property (contacts and tasks created without one,
   the `InOwnerBook` pairs), and such a frame is delivered to the author
   alone. Frames carry no entity data — clients re-read
   through the API; the stream never becomes a second system of record.
   The entity dictionary is the invalidation contract between backend and
   frontend:

   | entity | frontend query-key families invalidated |
   |---|---|
   | `payments` | paymentKeys, globalPaymentKeys |
   | `operations` | paymentOperationKeys, globalOperationKeys |
   | `tasks` | taskKeys |
   | `contacts` | contactKeys |
   | `rentals` | rentalKeys |
   | `property` | propertyKeys |
   | `access` | accessKeys, participantsKeys, propertyKeys |
   | `history` | historyKeys |

   The `access` row covers the property families as well (consumer ticket
   #719): a grant, a resume or a role change is published as an `access`
   frame, and the recipient — an active member at publication, hence in the
   audience — reads the change on surfaces living in `propertyKeys` (their
   properties hub list, the role pill on the property detail). Without the
   coverage the new object would appear in their book only after a reload.

   The provider may scope invalidation by the frame's `propertyId` or
   invalidate whole families; over-invalidation is the accepted cost of
   coarse frames. Adding an entity is one dictionary entry plus its capture
   points — backward compatible by construction. A screen may also become a
   precise consumer of its entity's frames (ticket #718, the live history
   feed): while such a subscriber is mounted, the provider delivers the
   frame to it and suppresses the blanket invalidation of that entity's
   families — the feed merges fresh rows into the cached first page (a
   page-prepend breaks refetch, #718) instead of refetching the loaded
   window (a window refetch slides the keyset and yanks a reader of old
   rows); with no subscriber the blanket
   path applies unchanged, and the on-open re-read still re-anchors the
   mounted feed's window.

3. **Carrier publication at the mutation seams, strictly post-commit.**
   The same discipline as grace_events, tariff_events and the history
   journal: each context's transaction captures the distinct
   (entity, property) pairs it changed; publication dispatches strictly
   after the commit and is best-effort — a failed frame is logged and never
   fails or rolls back the mutation. Per-transaction dedup: one frame per
   distinct pair, so a bulk operation (e.g. «Удалить все выполненные»)
   emits one frame, not N. The in-process domain dispatcher
   ([ADR 0014](./0014-in-memory-event-dispatcher.md)) stays out of the
   path: there is one consumer, and the indirection would hide the
   publication from the pipelines that own the facts. The publisher port
   lives in the realtime context's application layer; publisher contexts
   depend on the port, the hub-facing adapter computes delivery.

   Scheduled system materialization (the hourly zone sweep and the tick
   materialization) publishes no frames — recorded out-of-v1 (#716): those
   changes are not other people's edits, the hourly cadence and the on-open
   re-read cover them. The billing worker's tariff phases
   (Enforce/Recover/ArchiveExcess/RestoreGrace) stay silent in v1 the same
   way — the recipient's screens catch up on the next stream open; full
   publication is a v2 candidate (owner decision, R3 gate).

4. **Audience is resolved at publication time, the actor included.** The
   adapter resolves each frame's recipients through derived property access
   (ADR 0028) at the moment of publication: a member whose access is
   revoked or suspended simply stops receiving frames — no connection
   management on access changes (the earlier «revocation closes the
   stream» reading of ticket #716 is dropped with the per-user topology).
   The audience includes the actor: their other tabs and devices need the
   invalidation, and the origin tab re-reads idempotently. For the
   null-propertyId owner-book frames the audience degenerates to the actor
   alone: the book has no members to resolve, so the adapter short-circuits
   to `[actor]` without touching derived access — the open API's
   `/realtime/stream` description states the same null case.

5. **No replay in v1.** Same contract as ADR 0060 §8: on every open
   (including every reconnect) the provider re-reads live state through
   react-query invalidation; `Last-Event-ID` is logged, not served. A
   replay buffer stays the pre-planned v2 of ADR 0060.

## Out of scope

- **Concurrent-edit conflicts** (409 «правило изменил другой участник»,
  `updatedAt` preconditions) — a separate future effort: it touches every
  mutation contract and UI flow and has no dependency on this stream.
- **Web Push offline delivery** — a future product effort: an offline
  client has no cache to invalidate and re-reads on its next open; a
  «someone changed something» push needs its own product decisions (which
  events qualify, texts, dedup) beside the notification channels of the
  notifications context.

## Consequences

- (+) Live shared-property screens ride the built transport with one extra
  connection per tab; adding an entity to the stream is a dictionary entry
  and capture points, no new infrastructure.
- (+) Access changes need no stream bookkeeping: the audience is computed
  per frame, revoked members silently stop receiving.
- (−) Coarse frames over-invalidate: a change to one object may refetch
  mounted queries of other objects in the same family; acceptable for the
  product's scale (few properties per owner).
- (−) Frames are in-memory per instance (ADR 0060 §9): they do not survive
  deploys and do not cross replicas — the reconnect-and-reread contract
  absorbs both.

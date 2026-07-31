# ADR 0025: Property Deletion (Cascade and Detach Modes)

## Status

Accepted

## Context

Owners need to delete a property. Deletion is only allowed while the property
has no open lease. Two outcomes are required, chosen by the user at deletion
time:

- delete the property **together with all its data** (leases, operations,
  recurring operations, reminders) — for owners who no longer need the
  history;
- delete **only the property**, keeping the financial history as records no
  longer attached to any property — for owners who want the numbers to
  survive.

Key questions:

- Hard-delete or soft-delete (`properties.deleted_at`)?
- What do foreign keys from `leases`, `operations`, `recurring_operations`,
  `reminders` and `property_photos` do when a property row is deleted?
- Is "no property" (`property_id = NULL`) a legal state for leases,
  operations and recurring operations, and how far does it propagate
  (DB → domain → API → admin → frontend)?
- What happens to recurring operations and scheduled reminders when the
  property is detached?

## Decision

### 1. Hard-delete with a `mode` query parameter

`DELETE /properties/{id}?mode=cascade|detach` performs a hard delete.
Soft-delete was rejected: it would require reworking every property query
and a separate answer for the visibility of detached leases (see the design
document). The deletion is irreversible in both modes.

API semantics:

- missing or invalid `mode` → `400 Bad Request`;
- property not found or owned by another user → `404 Not Found`;
- the property has an open lease → `409 Conflict` — **both modes** are
  blocked, the owner must finish the lease first;
- success → `204 No Content`.

### 2. FK rules: `ON DELETE SET NULL` for business tables

Migration `000086_property_deletion_detach` changes the FK rule to
`ON DELETE SET NULL` and makes `property_id` nullable for:

- `leases.property_id`
- `operations.property_id`
- `recurring_operations.property_id`
- `reminders.property_id` (already nullable)

`property_photos.property_id` stays `ON DELETE CASCADE`: photos are
meaningless without the property, so they are removed in both modes (S3
cleanup is best-effort after commit, as in `DeletePropertyPhoto`).

This is a deliberate deviation from the `ON DELETE CASCADE` convention used
for user-owned rows in ADR 0022/0023: leases and operations are financial
history that must be preservable, not child rows that only exist inside
their parent. `SET NULL` matches the audit-log precedent (`actor_id` in ADR
0020), where the record outlives the entity it referenced.

In `cascade` mode the service explicitly deletes operations, recurring
operations and leases (in that order, because operations reference the other
two) before deleting the property row; reminders referencing them are
removed by the existing `ON DELETE CASCADE` on their target columns, so no
orphaned reminders remain.

### 3. Detach mode pauses recurring operations like archiving does

Before deleting the property row in `detach` mode, the service runs the
existing `billingLifecycle.Suspend` inside the same transaction: recurring
operations of the property are paused, future unedited operations are
hard-deleted, and pending reminders are cancelled — the same semantics as
manual archiving. Reusing `Suspend` keeps "stop the billing of a property
that is gone" in exactly one place.

### 4. "No property" is a legal state, end-to-end

Detached leases, operations and recurring operations keep existing with
`property_id = NULL`. This propagates through all layers:

- **Go domain**: the Nil-convention — `uuid.Nil` means "not attached"
  (helper `PropertyIDPtr`, same pattern as the neighbouring
  `LeaseIDPtr`/`RecurringOperationIDPtr`); the API boundary maps it to a
  nil pointer, never to the zero UUID string.
- **OpenAPI**: `property_id` is `nullable: true` and out of `required` in
  `LeaseResponse`, `OperationResponse`, `RecurringOperationResponse`,
  `FinanceReportPropertyRow`, `AdminLease` and `AdminOperation`. Create
  requests still require a property.
- **Finance report**: the NULL group is shown as a separate row
  «Без объекта» (`property_id: null`) instead of being filtered out, so
  `ByProperty` stays consistent with `Totals`.

### 5. CreateLease takes a row lock on the property

`CreateLease` locks the property row (`GetByIDAndOwnerForUpdate`) inside
its transaction and re-checks that the property exists and is not archived
before creating the lease. This closes the race with `DeleteProperty`:
a lease can no longer be created against a property that is being deleted
concurrently. The earlier pre-check remains as a fast path.

## Consequences

- (+) Both user stories are covered with one endpoint and explicit,
  transactional semantics per mode.
- (+) `detach` keeps financial history intact with minimal code: the FK
  does the unbinding, `Suspend` reuses the archiving lifecycle.
- (+) The nullable `property_id` contract is uniform: one convention
  (`uuid.Nil` in the domain, `null` in the API) across user and admin
  surfaces, verified by the type system after regeneration.
- (+) The finance report remains internally consistent
  (`ByProperty` vs `Totals`) because the detached group is reported, not
  dropped.
- (−) Hard delete is irreversible; if restore is ever needed, soft-delete
  must be reconsidered as a separate change.
- (−) The down-migration of `000086` is **not runnable after the first real
  detach**: its `SET NOT NULL` fails while any row has
  `property_id IS NULL`. Rolling back past `000086` requires manual cleanup
  (re-attach or delete detached rows). This does not affect the deploy
  pipeline, which never runs `migrate down` automatically (ADR 0024); the
  caveat is documented in `docs/deployment.md`.
- (~) Detached leases no longer participate in the occupancy index and are
  invisible in any property context — accepted, expected behaviour.

## See also

- [`docs/plans/2026-07-29-property-deletion-design.md`](../plans/2026-07-29-property-deletion-design.md)
  — the approved feature design (including the soft-delete rejection) and
  the post-review amendments.
- [`docs/adr/0020-audit-log.md`](./0020-audit-log.md) — the audit entries
  written for `property.deleted`; the `SET NULL` precedent for references
  that outlive their target.
- [`docs/adr/0022-notification-preferences.md`](./0022-notification-preferences.md),
  [`docs/adr/0023-popup-views.md`](./0023-popup-views.md) — the
  `ON DELETE CASCADE` convention for user-owned rows that this ADR
  deliberately does not follow for property references.
- [`docs/adr/0024-deploy-pipeline-ghcr-runner.md`](./0024-deploy-pipeline-ghcr-runner.md)
  — expand-contract migration policy; why the irreversible down-migration
  is acceptable in the deploy flow.

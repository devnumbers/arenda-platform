# ADR 0055: Grace v2 — One Active Property in Grace, Billing Archive and Restoration

The grace period after a failed renewal charge used to be a pure notification
window: the owner kept every property active on the failed tariff while the
dunning retries ran. The owner decided (map #611, ticket #615) that grace must
already reflect the unpaid state: the subscription keeps exactly one active
property, the excess is archived by billing, and a later successful payment
restores precisely what billing archived. This ADR amends the grace semantics
of ADR 0008 §4; the rest of the lifecycle there is unchanged.

## Status

Accepted (amends ADR 0008)

## Decision

**Entry into grace** (every path — the renewal worker's failed charge, the
asynchronously failed webhook charge, and the no-chargeable-method planning
exit, all through the single `enterGrace` seam): in the same transaction as
the grace transition, billing archives the owner's active properties beyond
one — the survivor is the most recently updated (`updated_at DESC`, the
existing excess-archive policy) — and suspends the affected recipients' shared
memberships through the access bridge (`grace_entry` trigger). The archived
ids are snapshotted on the subscription row
(`user_subscriptions.grace_archived_property_ids`, restoration-priority order,
newest first). This snapshot is the restoration debt billing owes the owner.

**During grace**: data mutations stay open (`CanMutateData` unchanged), but
the effective active-property limit is 1 regardless of the tariff — the
property limiter returns the grace limit, so create and restore count against
the survivor. Archived data stays readable.

**Restoration on success**: every applied payment lands in the single
success-application seam (`applySucceededPayment`), so all recovery paths —
the dunning retry, the manual «Оплатить тариф» renewal, an upgrade — restore
in the payment's transaction exactly the snapshotted ids, respecting the
(possibly changed) tariff limit, newest-first, skipping ids gone or already
active since the snapshot; the access bridge re-enforces recipient slots per
restored property (same as a manual unarchive). What fits clears from the
snapshot with the same commit; a limit-constrained remainder stays as the
debt for the next applied payment. A bridge failure fails the whole
application, the provider redelivers, and the seam retries — the debt is
never silently dropped. Manual archiving the user did themselves is never
touched.

**Debt dropped**: the fall to basic (grace expiry, refund of the current
period, cancellation expiry) clears the snapshot — the grace archive stays in
the archive, nothing is restored. A service-subscription assignment clears it
with the rest of the overwritten state. Cancel inside the grace window keeps
the snapshot: only a payment restores, so a later reactivation payment
(#429) still settles the debt, and the expiry path drops it if the user never
pays. Repeated grace entry overwrites the snapshot — a fresh window owes a
fresh restoration; an open window is never re-entered, so a failed dunning
retry re-archives nothing.

**Service subscriptions** never enter grace: they carry no auto-renew and no
charges, so the renewal selection never reaches them (structural, pinned by
tests).

## Considered Options

- **Recover the archived set from the audit `trigger: "billing_limit"`
  marker** — rejected: audit is an incident tool, not operational state;
  reconstructing a restoration set from log entries is fragile and cannot
  carry the restoration order.
- **Restore everything on payment regardless of ids** — rejected: would also
  resurrect properties the user archived deliberately during grace.
- **Keep the grace limit at the tariff limit (status quo)** — rejected by the
  owner: grace after a failed charge must not offer the full paid experience.

## Consequences

- (+) The unpaid state is visible in the product: one property survives, the
  archive explains the rest, and payment restores exactly what billing took.
- (+) The restoration is transactional with the payment and idempotent under
  the existing no-op guards of the success seam.
- (~) A cancelled subscription inside its grace window keeps the cancelled
  rule of reading the tariff limit from the remaining validity — the grace
  limit of 1 no longer applies once the status leaves grace; the pending
  snapshot stays until a reactivation payment or the expiry downgrade.
  Ticket #617 owns the cancel-side semantics.
- (~) The restoration order is the snapshot order (updated_at DESC at entry);
  if the tariff limit does not fit all ids, the newest restore first and the
  debt remainder stays until the next applied payment or the fall to basic.
  Ids gone or already active since the snapshot are settled by the restore
  itself and leave the snapshot.

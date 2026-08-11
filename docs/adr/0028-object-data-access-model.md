# ADR 0028: Object Data Access Model (Policy Port and Actor/Scope)

## Status

Accepted

## Context

The platform today authorizes implicitly: SQL queries filter
`WHERE owner_id = $authenticatedUser`, and the application layer threads a
single `ownerID` through every service and repository. This works because each
user only ever touches their own data. But PRD #153 (Property Sharing) will let
an owner grant another user access to a property (roles: viewer, full_access).
When that happens, an actor must be able to read and sometimes write another
user's (the data owner's) data — the single-owner assumption breaks. The
question is where authorization lives: scattered in SQL, in the transport, or
in a single application-layer port.

Key questions:

- Where is the single point of authorization?
- How to separate "who acts" from "whose data" without changing current
  behavior?
- What happens to SQL owner-scoped queries?
- How are capabilities (view/edit/manage-members/lifecycle) expressed?

## Decision

### 1. Policy port as the single point of authorization

A cross-cutting `Policy` port (`internal/shared/policy`) is the only place
where an actor's role relative to a data owner (scope) is decided. Signature:
`Role(ctx, actor, scope) (Role, error)`. Capabilities are derived from the role
via pure functions `CanView`, `CanEdit`, `CanManageMembers`, `CanLifecycle`.
No service or repository re-implements authorization; it delegates to the
policy port. The port mirrors the existing `OwnerTimezoneResolver`
cross-cutting pattern (`internal/shared/tzresolver`).

### 2. Actor and scope: splitting the owner concept

The former single `ownerID` is split into two concepts threaded through the
application layer:

- **actor** — the operation initiator (the authenticated user, or system for
  background jobs). Public service methods receive `actor`.
- **scope** — the data owner whose data is read or written. Repository methods
  receive `scope`; SQL filters `WHERE owner_id = $scope`.

For the owner's own data, `actor == scope`. In T3 (property membership), an
actor who is a member of another owner's property will operate with `scope` set
to that owner; the policy port resolves the role and the capabilities gate the
operation.

### 3. SQL stays scoped by data owner, not by actor

Owner-scoped SQL queries are unchanged: they filter
`WHERE owner_id = $scope` where `scope` is the data owner. The sqlc-generated
`Params.OwnerID` field name stays (it maps to the `owner_id` column).
Membership is NOT a SQL concern in T2; it will be resolved by the policy port.
This avoids premature SQL joins against a membership table that does not yet
exist and keeps the behavior identical to the pre-T2 single-owner world.

### 4. Capabilities: single branch today, ready for T3 roles

The four capability functions already account for all planned roles:
`CanView` returns true for owner, full_access, viewer; `CanEdit` and
`CanManageMembers` for owner and full_access; `CanLifecycle`
(archive/unarchive/delete) only for owner. The T2 policy (`OwnerOnlyPolicy`)
returns only `RoleOwner` or `RoleNone` — the member roles (`RoleFullAccess`,
`RoleViewer`) are defined but not yet returned. T3 will replace the policy
implementation with one that consults property membership; the capabilities and
the actor/scope seam need no further changes.

### 5. Scope of the split

The actor/scope split applies to property-scoped data contexts: properties,
leases, notifications (reminders, free reminders). Billing (subscriptions,
payments, payment methods) and popups remain keyed by `userID` — they are
personal account data, never delegated via property membership (per
CONTEXT.md: a shared object consumes the recipient's own tariff slot;
subscription/payment data is never shared).

## Consequences

- (+) Single point of authorization: the policy port is the only seam T3 needs
  to change to enable property sharing.
- (+) Actor/scope separation is in place everywhere property data flows, so T3
  adds membership logic without touching repository or SQL signatures.
- (+) Capabilities are pure functions of role — easy to test, easy to extend,
  no hidden authorization in SQL or transport.
- (+) Billing and popups correctly excluded: their data model is personal, and
  forcing actor/scope onto them would be ceremony with no behavioral path.
- (−) Mechanical churn: ~60 repository signatures and ~40 handler call sites
  were renamed; the compiler and existing tests catch any miss, but the diff is
  large for a behavior-preserving change.
- (~) The policy field on services is unused in T2 (the port returns
  `RoleOwner` for `actor == scope`, which always passes capability gates); it
  becomes load-bearing in T3. This is the intended "expand" — the
  infrastructure is built before the feature.

## See also

- [`docs/adr/0025-property-deletion-modes.md`](./0025-property-deletion-modes.md)
  — the format reference and a neighbour lifecycle decision (object deletion is
  a `CanLifecycle`-gated operation).
- Issue [`#155`](https://github.com/devnumbers/arenda-platform/issues/155) —
  the T2 ticket that introduced the policy port and the actor/scope split.
- Issue [`#153`](https://github.com/devnumbers/arenda-platform/issues/153) —
  PRD: Property Sharing (the feature this seam prepares for).
- `CONTEXT.md` entries: "Совместный доступ к объекту", "Участник объекта",
  "Полный доступ", "Просмотр" — the sharing roles the policy port will resolve
  in T3.

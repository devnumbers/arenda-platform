# ADR 0026: Property Contacts as a Separate Entity

## Status

Accepted

## Context

Owners need to attach arbitrary contacts to a property (plumber, management
company, concierge, dispatcher) — people to call about the property who are
not tenants. The system already has `tenant_contacts`: an owner-level contact
book living in the `leases` bounded context, linked to a property only
through `leases.tenant_contact_id`.

Key questions:

- Extend `tenant_contacts` into a unified contact book (one table, contacts
  reusable across properties and linkable to leases) or model property
  contacts as a separate entity?
- Should contacts be deduplicated or shared between properties?
- What happens to contacts when the property is deleted (ADR 0025)?

The decision was made in the wayfinder map #85 (ticket #86) and is recorded
here because it fixes the data model in a way that is hard to reverse and
rejects the seemingly natural alternative.

## Decision

### 1. Separate entity `property_contacts` in the `properties` context

Property contacts are a new table `property_contacts`
(`property_id` FK NOT NULL + `owner_id`) owned by the `properties` bounded
context. `tenant_contacts` and the lease→tenant link are unchanged.

The unified contact book was rejected: a property contact and a tenant
contact have different lifecycles, fields (a property contact needs only
name + phone; a tenant contact carries email, comment, lease links) and
ownership semantics (per-property vs per-owner book). Merging them would
couple the `properties` and `leases` contexts for no current user value.

### 2. A contact belongs to exactly one property; no deduplication

The same person being a contact of several properties yields independent
rows — phone and role typically differ per property, and independent editing
is a feature, not a bug. There is no uniqueness constraint and no
cross-property reuse.

### 3. Contacts are cascade-deleted with the property in both deletion modes

`property_contacts.property_id → properties(id) ON DELETE CASCADE` applies
in both `cascade` and `detach` modes (ADR 0025), mirroring `property_photos`:
a contact without its property is meaningless and unreachable, because the
property page is the only surface where contacts live. There is no link
between property contacts and tenants at the data level; one person may
silently be both.

## Rejected alternatives

- **Unified contact book** (generalize `tenant_contacts`): rejected — see
  section 1. It would also require dedup/merge semantics the product does
  not want.
- **Dedup by `(owner_id, phone)`** like tenant contacts: rejected — that
  rule deduplicated the owner's contact *book*; here duplicates are
  legitimate (one city number of the management company for "dispatcher"
  and "accounting").
- **Keeping contacts on detach**: rejected — unlike leases and operations,
  contacts carry no standalone history value.

## Consequences

- The fog item about generalizing `tenant_contacts` dissolves; the fog item
  about deduplication is resolved negatively.
- If a cross-property contact directory is ever wanted, it must be built as
  a new read model or via data migration — the separate-entity model makes
  "merge later" non-trivial.
- Canonical domain term: «Контакт объекта» / Property Contact; the glossary
  collision with «контакт» (= tenant contact) is resolved in `CONTEXT.md`.

## See also

- ADR 0025 (property deletion modes), ADR 0020 (audit log), ADR 0019 (uuid v7).

# ADR 0019: Application-Generated UUIDv7 Identifiers

## Status

Accepted

## Context

Entity ids were UUID v4, and their generation was split between the database and the application. Every `id` column carried `DEFAULT gen_random_uuid()`, so an `INSERT` without an explicit `id` silently received a database-generated value. At the same time, domain constructors and application code assigned their own v4 ids via `uuid.New()`, `uuid.NewString()`, or `uuid.NewRandom()`. Because the sqlc INSERT statements did not include the `id` column, repositories ignored the domain-assigned id and persisted a different, database-generated one: the entity the application held in memory (and referenced in domain events) did not share identity with the stored row, and id generation was invisible to application code.

Random v4 ids also land at random positions in B-tree indexes, which worsens insert locality as tables grow.

## Decision

Entity identifiers are UUIDv7, generated exclusively by the application.

- All ids are created with `uuid.NewV7()` from `github.com/google/uuid`; the id assigned by the domain or application layer is the id that gets persisted. Call sites whose signature returns an error propagate the `NewV7` error; call sites that previously used the panicking `uuid.New()`/`uuid.NewString()` use `uuid.Must(uuid.NewV7())`. Public signatures of domain constructors are unchanged.
- Migration `000079_uuid_v7_app_side` drops `DEFAULT gen_random_uuid()` from the `id` columns of all 18 tables (see `db/migrations/000079_uuid_v7_app_side.up.sql`). Existing data is untouched; the down migration restores the defaults.
- Every sqlc INSERT takes an explicit `id` as its first parameter, and repositories map the domain `uuid.UUID` to `pgtype.UUID` at the persistence boundary, as the identity repositories already did.
- Existing v4 rows stay in place. Mixing v4 and v7 values in the same column is safe: both are valid 128-bit UUIDs; only the internal bit layout, and therefore id ordering, differs.
- The native `uuidv7()` function of PostgreSQL 18 is reserved for seeds and manual SQL inserts. Application-written rows never rely on database-side id generation.

UUIDv7 was chosen over keeping v4 because its leading bits carry a Unix-millisecond timestamp: new ids are roughly time-ordered, which keeps `ORDER BY id` meaningful and improves B-tree insert locality compared to random v4.

## Consequences

- An INSERT without an explicit `id` now fails with a `NOT NULL` violation. This is intentional: a missing id surfaces immediately at the persistence boundary instead of silently diverging from the domain entity.
- `ORDER BY id` remains a valid tie-breaker (typically after `created_at`): rows written after this change have ids ordered by creation time. Pre-migration v4 rows order randomly by id, which only affects result sets that order by `id` alone across old and new rows.
- New tables are created without a `DEFAULT` on `id`; new migrations and seeds must set ids explicitly (in SQL, `uuidv7()`).
- Domain-assigned ids are authoritative: repositories persist the id the domain generated, so entities returned before and after persistence, and any domain events, reference the same id.

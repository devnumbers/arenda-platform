---
name: postgresql-best-practices
description: PostgreSQL rules of this repository — schema, migrations, sqlc, indexes, and transactions under repo domain invariants (BIGINT money, app-side UUIDv7, no ORM). Use for database schema, migrations, SQL, sqlc queries, indexes, transactions, locks, or financial invariants.
---

# PostgreSQL (this repository)

A thin orientation skill: the database contract lives in repo documents and lint gates; this skill points at them, inverts the generic advice that conflicts with them, and lists primary sources. It replaces a generic community PostgreSQL skill whose defaults (NUMERIC money, SERIAL ids, RLS tenancy) are wrong for this codebase.

## Read first (repo sources of truth)

1. `apps/backend/AGENTS.md`, section "API And Persistence" — the migrations + queries workflow, sqlc, and schema invariants.
2. `apps/backend/db/migrations` and `apps/backend/db/queries` — the actual conventions; `make migrations-lint` (squawk + `tools/migration-lint/domain-rules.mjs`, config `.squawk.toml`) is the gate.
3. ADRs: `docs/adr/0008` (money kopecks), `0019` (UUIDv7 app-generated), `0028` (object data access model), `0033` (transaction/UoW), `0020` (audit records in the business transaction).

## Generic advice inverted here

| Generic PostgreSQL advice | This repository |
|---|---|
| `NUMERIC`/`DECIMAL` for financial data | Banned outright in migrations — money is `BIGINT` kopecks; `numeric`/`decimal`/`real`/`double`/`float` columns fail `make migrations-lint` |
| `SERIAL`/`BIGSERIAL` ids; `DEFAULT gen_random_uuid()` | UUIDv7 generated in the application (`uuid.NewV7()`); `id` columns carry no DB `DEFAULT` |
| Row-level security for multi-tenancy | No RLS — authorization goes through the policy port (`internal/shared/policy`); owner-scoped queries filter by `scope` (ADR 0028) |
| ORM or query builders | sqlc only (depguard `no-orm`); explicit SQL in `db/queries`, regenerated and freshness-checked in CI |
| PgBouncer / pgpool-II | pgx v5 built-in pool; no external pooler in the stack |
| Free-form migration DDL | Versioned golang-migrate files; lock-safety linted by squawk; where SQL must generate ids, use PostgreSQL 18 `uuidv7()` |

Also: `date` for domain dates, `timestamptz` for system timestamps; durable rules live in the database (`NOT NULL`, foreign keys, `CHECK` constraints, indexes, triggers where they protect invariants); integration tests run against a real PostgreSQL 18 via testcontainers.

## Primary sources (verify here, in this order)

1. Official docs: `https://www.postgresql.org/docs/` — currently version 18 (see `apps/backend/AGENTS.md`, integration tests).
2. PostgreSQL Internals (Egor Rogov): `https://postgrespro.com/community/books/internals` — planner, indexes, MVCC, vacuum.
3. Use The Index, Luke: `https://use-the-index-luke.com/` — index design against real queries.
4. pganalyze blog (`https://pganalyze.com/blog`) and brandur.org — operational and performance practice.
5. The Art of PostgreSQL (Dimitri Fontaine) — SQL-first query patterns.

Repo ADRs and `make migrations-lint` outrank this skill and any external source on conflict.

# ADR 0003: Use pgx and sqlc for PostgreSQL

## Status

Accepted

## Context

The backend needs explicit PostgreSQL modeling for financial and lifecycle rules. ORMs can hide query behavior and blur persistence models with domain models.

## Decision

Use explicit SQL, `sqlc`-generated query code, and `pgx` for PostgreSQL access.

## Consequences

SQL is visible and reviewable. Repositories map generated persistence types into domain models.

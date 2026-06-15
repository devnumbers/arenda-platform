# ADR 0001: Start Backend as a DDD Modular Monolith

## Status

Accepted

## Context

The MVP has several domain areas: identity, properties, billing, leases, payments, reminders, and analytics. Splitting them into separate services would add deployment, networking, consistency, and observability overhead before the product has validated service boundaries.

## Decision

Start with one Go backend process organized as DDD bounded contexts under `apps/backend/internal`.

## Consequences

Domain boundaries are enforced through package dependencies, application ports, tests, and documentation. Future service extraction remains possible after real operational pressure appears.

# ADR 0002: Use Contract-First OpenAPI

## Status

Accepted

## Context

The frontend needs a stable API contract. Backend domain code should not leak generated transport DTOs into business logic.

## Decision

Maintain `apps/backend/api/openapi/openapi.yaml` as the public API contract and generate transport edge code from it. Map generated DTOs to application commands and queries at the HTTP boundary.

## Consequences

OpenAPI is reviewed as a product contract. Generated code must stay out of domain and application packages.

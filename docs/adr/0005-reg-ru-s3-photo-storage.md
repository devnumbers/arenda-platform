# ADR 0005: Store Property Photos in REG.RU S3

## Status

Accepted

## Context

Object photos are part of the MVP. The project will use REG.RU S3-compatible object storage instead of running a local MinIO service.

## Decision

Use a storage port in the backend and implement it with REG.RU S3-compatible credentials and endpoint configuration. Tests use fake storage unless explicitly run with real S3 credentials.

## Consequences

Docker Compose remains smaller. Developers need S3 environment variables for real photo upload testing.

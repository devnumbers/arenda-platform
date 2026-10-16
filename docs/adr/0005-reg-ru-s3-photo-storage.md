# ADR 0005: Store Property Photos in REG.RU S3

## Status

Accepted; частично заменён [ADR 0065](./0065-private-photos-one-image-per-entity.md) (карта #1217, решение #1220): провайдер (Рег.ру S3) и порт хранилища остаются, публичная часть — перманентные публичные URL и canned-ACL — уходит вместе со старым контрактом до-10-фото.

## Context

Object photos are part of the MVP. The project will use REG.RU S3-compatible object storage instead of running a local MinIO service.

## Decision

Use a storage port in the backend and implement it with REG.RU S3-compatible credentials and endpoint configuration. Tests use fake storage unless explicitly run with real S3 credentials.

## Consequences

Docker Compose remains smaller. Developers need S3 environment variables for real photo upload testing.

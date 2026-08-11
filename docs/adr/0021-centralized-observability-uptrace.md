# ADR 0021: Centralized Observability with Uptrace

## Status

Superseded — transferred to the devnumbers/observability repository
(ADR 0001). The observability stack, its compose project, configuration,
and operational procedures no longer live in this repository.

## Context

This ADR recorded the decision to run a self-hosted Uptrace stack for
stage/prod logs, traces, and metrics. The stack was originally maintained
in this repository as compose project `arenda-obs`
(`docker-compose.obs.yml` + the `observability/` directory).

## Decision

The observability stack was extracted into a dedicated repository —
[devnumbers/observability](https://github.com/devnumbers/observability)
(ADR 0001). On the server it runs from `/opt/observability` as compose
project and Docker network `observability`. This repository no longer
carries the stack files, the `obs-*` make targets, or the `.env.obs`
configuration.

The consumer side — how stage/prod backends connect to the stack — is
documented in [`docs/deployment.md`](../deployment.md) (section
"Observability"): the external `observability` network, the OTLP endpoint
`http://uptrace:14317`, the `OTEL_*` environment block, and the deployment
marker in `OTEL_RESOURCE_ATTRIBUTES`.

## See also

- [devnumbers/observability](https://github.com/devnumbers/observability) —
  ADR 0001, stack configuration, and operational procedures.
- [`docs/deployment.md`](../deployment.md) — consumer-side observability
  connection for stage/prod.
- [`docs/adr/0020-audit-log.md`](./0020-audit-log.md) — business audit in
  the database; the boundary between the business journal and system
  observability.

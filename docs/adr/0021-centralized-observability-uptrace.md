# ADR 0021: Centralized Observability with Uptrace

## Status

Accepted (amended 2026-07-24: Telegram replaced with email alerts; resource
limits raised after the host upgrade: clickhouse 3 GiB / 2 CPU, uptrace
512 MiB)

## Context

Technical (system) logs of all services lived only in the Docker json-file:
rotation `max-size=10m`, `max-file=5` — roughly 50 MB of history per
container. The position "system logs stay as slog → stdout → docker logs"
was recorded in the audit log design (see ADR 0020), but that was the scope
of that design — business audit is not system logging — not a long-term
observability decision; this ADR revises it.

In that shape the platform has no log search, no history beyond rotation,
and no alerting at all: we learn about outages after the fact. The
requirements confirmed by the owner are broader than logs:

- one observability service for stage and prod (both environments live on
  the same VPS anyway);
- no Grafana/ELK stacks;
- full coverage: logs of all containers, Caddy access logs, browser JS
  errors, server and container metrics, application traces and metrics;
- alerts to Telegram;
- remote UI access via `https://logs.rentlee.ru`.

Constraints: logs and traces may contain 152-FZ-sensitive data (PII), so
they must not leave our own infrastructure — self-hosted only; the VPS is
modest (5.8 GB RAM, an upgrade to ~8-10 GB approved by the owner).

## Decision

We run **Uptrace self-hosted** (AGPLv3) on the same VPS: logs, traces, and
metrics in a single UI with email alerts. The stack is a separate
compose project `arenda-obs` (`docker-compose.obs.yml` + the
`observability/` directory in the repository); on the server it is deployed
from `/opt/arenda/obs` — outside the stage/prod checkouts, which are cleaned
by `git clean -ffdx`.

Components and data flows:

- **Uptrace**: `clickhouse` (storage), `uptrace-pg` (metadata), `uptrace`
  (OTLP intake + UI), `redis` (cache, required by uptrace 2.0.3). Data
  retention ~30 days (TTL in `uptrace.yml`).
- **Vector** collects logs: sources `docker_logs` (stdout of all containers,
  read-only docker socket) and the files `/var/log/caddy/access.log`,
  `/var/log/arenda/healthcheck.log`; ships to Uptrace over the official
  Vector HTTP intake (`http://uptrace:14318/api/v1/vector/logs`,
  `uptrace-dsn` header). The local json-file driver is kept — `docker logs`
  works as before.
- **OTel Collector** (contrib) collects infrastructure metrics: receivers
  `hostmetrics` (host CPU/RAM/disk/net) and `docker_stats` (per-container),
  exported to Uptrace over OTLP.
- **Backend** activates the already built-in OpenTelemetry SDK
  (`internal/platform/observability`): traces and metrics over OTLP to
  `http://uptrace:14317` through the external docker network `arenda-obs`,
  which both environments' backends join. Configuration is the `OTEL_*`
  block in `.env.stage`/`.env.prod`.
- **slog stdout remains the logging source of truth**: logs reach Uptrace
  through Vector from container stdout; OTel Logs are not added to the
  backend. With tracing enabled, slog records carry the real OTel
  `trace_id`/`span_id`, which correlates logs with traces in the UI.
- **Browser JS errors**: public `POST /client-errors` (validation, field
  truncation, sanitization, dedicated rate limit, no DB writes) — the
  backend logs them to stdout, from where they take the usual path through
  Vector.
- **Alerts**: email alerts via SMTP (`mailer.smtp` in `uptrace.yml`); the
  channel is configured in the UI, and the SMTP credentials are not stored
  in the repository.
- **UI access**: `https://logs.rentlee.ru` — Caddy `basic_auth` +
  `reverse_proxy 127.0.0.1:14318` (credentials live only in the server
  Caddyfile); fallback is an SSH tunnel to `127.0.0.1:14318`. The OTLP port
  `14317` is not published externally and is reachable only from the
  `arenda-obs` network.

Resource budget: six containers (`clickhouse`, `uptrace-pg`, `uptrace`,
`redis`, `vector`, `otelcol`) with RAM/CPU limits following the stage/prod
compose conventions, ~2.1 GB RAM total — acceptable after the approved host
upgrade.

## Rejected alternatives

- **OpenObserve.** Lighter on resources and also all-in-one, but noticeably
  less polished (UI, alerts, ecosystem).
- **VictoriaLogs.** Logs only; server and container metrics would require a
  second stack (VictoriaMetrics + vmalert) with a spartan UI, and there
  would be no traces at all.
- **SaaS (Axiom, Better Stack, and the like).** Logs and traces with
  potential PII would leave our own infrastructure — a conflict with our
  152-FZ policy.
- **ELK / Grafana stacks (Loki + Prometheus + Grafana, Tempo).** Do not fit
  into the RAM of a modest VPS, and the owner does not want to maintain
  such a zoo.

## Consequences

- (+) Search and history across all logs, traces, and metrics in one UI;
  email alerts; log ↔ trace correlation via real OTel `trace_id`/`span_id`.
- AGPLv3 accepted: internal self-hosted use, no distribution, no service
  offered to third parties.
- The shared host gains +6 containers with ~2.1 GB RAM limits.
- `.env.stage`/`.env.prod` gain the `OTEL_*` block (`OTEL_SERVICE_NAME`,
  `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`,
  `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`,
  `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER_ARG`).
- New public endpoint `POST /client-errors` (rate-limited, sanitized, no
  DB) — the only new public surface of the backend.
- The PII policy for logs is unchanged: no bodies/cookies/secrets/PII in
  logs, low-cardinality metrics and span names (route pattern),
  sanitization of client errors.
- UI access is protected by Caddy `basic_auth`; all stack ports are bound
  to `127.0.0.1`, and OTLP is reachable only from the `arenda-obs` docker
  network.
- (-) Accepted risk: `vector` and `otelcol` mount `/var/run/docker.sock`
  read-only, but `ro` on a unix socket does not restrict Docker API calls —
  compromising either container is equivalent to controlling dockerd, i.e.
  root on the host. This is the commonly accepted trade-off of the
  `docker_logs`/`docker_stats` receivers; both images are first-party and
  run only in the isolated `arenda-obs` network. Possible future hardening:
  a docker-socket-proxy with an ACL in front of the socket.
- `audit_log` in PostgreSQL (ADR 0020) remains the business audit and is
  unrelated to this decision: the business journal is viewed in the admin
  panel, system observability — in Uptrace.

## See also

- [`docs/adr/0020-audit-log.md`](./0020-audit-log.md) — business audit in
  the database; the boundary between the business journal and system
  observability.

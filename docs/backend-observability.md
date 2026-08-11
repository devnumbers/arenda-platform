# Backend Observability

Код-conventions для observability в backend. Инфраструктура observability (Uptrace, Vector, OTLP endpoint) — в `docs/deployment.md`, раздел "Observability".

- Use standard-library `log/slog` with JSON output for backend logs. Prefer `InfoContext`, `WarnContext`, and `ErrorContext` so request-scoped context can carry correlation fields.
- Keep logging middleware, request IDs, OpenTelemetry setup, exporters, and metrics in `cmd/api` or `internal/platform`. Domain packages must not import loggers, OpenTelemetry, HTTP middleware, or platform observability helpers.
- Every HTTP request must have an `X-Request-ID`. Preserve an incoming request ID when present; otherwise generate one. Include the same value in `Problem.requestId`, the response `X-Request-ID` header, and request outcome logs.
- Request outcome logs must include low-cardinality operational fields: `request_id`, method, route or operation, status, duration, and problem/error code when available.
- Do not log session tokens, cookies, authorization headers, S3 secrets, raw DaData payloads, raw request/response bodies, or other PII/secrets. Do not log SMS codes or full phone numbers in production integrations. The local/dev fake SMS sender is the only exception: it may log the SMS code and phone number so developers can complete manual login without a real SMS provider.
- OpenTelemetry is the tracing/metrics foundation. The telemetry backend is Uptrace (self-hosted) per `docs/adr/0021-centralized-observability-uptrace.md` (superseded — the stack now lives in the devnumbers/observability repo): the OTel SDK in `internal/platform/observability` exports traces and metrics over OTLP to `http://uptrace:14317`, enabled by the `OTEL_*` environment block in stage/prod. Keep exporters vendor-neutral through OTLP-compatible configuration; changes to the telemetry backend, Collector topology, production alerts, or SLO policy are tracked in the devnumbers/observability repo.
- Name HTTP server spans by the chi route pattern (set after routing), never by raw `r.URL.Path`: entity IDs in span names create high cardinality.
- With a valid OTel span context, slog records carry the real OpenTelemetry `trace_id`/`span_id`, so Uptrace correlates logs with traces.
- Keep metric labels low-cardinality. Do not use user IDs, property IDs, phone numbers, addresses, object storage keys, or raw paths as labels.
- OpenTelemetry Logs are intentionally not a backend signal: structured `slog` stdout remains the logging source of truth and reaches Uptrace through Vector (see docs/deployment.md, section "Observability").
- `POST /client-errors` is a public, rate-limited endpoint for browser JS errors: sanitized and truncated payload, no PII, nothing written to the DB; errors are logged to stdout and flow into the observability pipeline.
- Panics are logged structurally by the platform recovery middleware (`level=error` with stack trace and `request_id`, sanitized) and answered with an RFC 7807 problem via `writeProblem`; do not reintroduce the plain chi `Recoverer`.
- The audit log is not an observability log: business-audit records (who did what) persist to the `audit_log` table via `internal/audit` and are viewed in the admin panel, while structured `slog` stdout logs remain the source of truth for technical/operational logging. See `docs/adr/0020-audit-log.md`.

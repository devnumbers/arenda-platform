# ADR 0057: Session City Geolocation Resolves In the Application, Not at the Edge

## Status

Accepted

## Context

The devices list (map #724, ticket #728) shows a city for every session, resolved from the client IP. The platform's reverse proxy is Caddy (ADR 0024, ADR 0045, proxy inversion map #649), but it is a *shared host-level* server: this product deploys only a config fragment into `/etc/caddy/conf.d`, next to other projects' sites. Custom Caddy builds with GeoIP plugins, or edge-side header injection, would be an operation on shared infrastructure owned by nobody here.

The database choice was made in grilling #726/#728: **DB-IP City Lite (CC BY 4.0)** — free, no account or secret, direct download in CI, redistribution inside the image allowed with UI attribution, mmdb-compatible with `oschwald/geoip2-golang`, Russian city names built in. MaxMind GeoLite2 was rejected (EULA sanction clauses, unreliable access from RF); Sypex Geo stays plan B.

## Decision

The city is resolved **inside the backend application**, at session bookkeeping time — once at session creation, then on client-IP change:

- The DB-IP City Lite mmdb file is baked into the Docker image at build time (download in the build stage, `DBIP_MONTH` build-arg busts the layer monthly); the monthly image rebuild refreshes the data.
- The resolver adapter (`identity/adapters/geoip`) prefers the `ru` name and falls back to `en`; it answers «not found» for private, reserved, and unknown IPs — an unresolved lookup never erases a stored city.
- The frontend is served the ready city string; it never talks to a geolocation service.

## Consequences

- (+) No runtime dependency on external geolocation APIs; lookups are in-process and free.
- (+) No custom Caddy build: the shared proxy stays stock (ADR 0024/0045), the product deploys only its config fragment.
- (+) Attribution duty (CC BY 4.0) is satisfied by a credit line on the frontend «Информация» page.
- (-) The image grows by ~120 MB; data freshness is bounded by the image rebuild cadence — acceptable for city-level IP data.
- (-) A missing database (local development) degrades to «no city»; sessions keep working.

## See also

- [`0056-pure-sliding-sessions-hardening.md`](./0056-pure-sliding-sessions-hardening.md) — the session lifecycle this feeds.
- [`docs/research/2026-09-16-ua-geoip-go.md`](../research/2026-09-16-ua-geoip-go.md) — the database comparison behind DB-IP City Lite.

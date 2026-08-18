# ADR 0041: gosec G124 (cookie SameSite) is excluded module-wide

gosec's G124 flags any cookie whose `SameSite` is not `Strict`. The session cookie is deliberately `SameSite=Lax` in every environment (ADR 0018 — a product decision), with `HttpOnly` always set and `Secure` driven by `APP_ENV` (plus the `__Host-` prefix in production), so in this codebase the rule can only fire on a policy we have already decided; its signal is zero. The suppression lives in the golangci-lint config (`gosec.excludes: [G124]`, quality bar #323, ticket #343) instead of per-line `nolint` directives in `platform/httpsupport/session.go`.

## Consequences

- G124 is off for the whole module: a genuinely weak `SameSite` on some new cookie will not be flagged automatically. Cookie attributes are owned by `platform/httpsupport` and reviewed against ADR 0018, which keeps this acceptable.
- If ADR 0018 is ever revised back to `Strict`, remove the exclude from `.golangci.yml` — the rule becomes meaningful again.

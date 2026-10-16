# ADR 0042: webpush VAPID keeps crypto/elliptic ScalarBaseMult until the ecdh migration

## Status

Superseded — the ecdh migration landed (remediation ticket #345, grid #325;
the decision is now ADR 0043). The staticcheck exclusion this ADR sanctioned
was deleted from `.golangci.yml` together with the deprecated call itself.

The Web Push VAPID signer (`notifications/adapters/webpush/vapid.go`) signs ES256 JWTs (RFC 8292), which requires an `ecdsa.PrivateKey`. The key arrives as a raw 32-byte P-256 scalar, and the only stdlib route from that scalar to the public point — and thus to `ecdsa.PrivateKey` — is the deprecated `elliptic.Curve.ScalarBaseMult`: `crypto/ecdh` validates the scalar and exposes the public key bytes but has no conversion back to `ecdsa.PrivateKey`, and stdlib offers no signing API over `ecdh` keys. We accept the deprecated call and suppress staticcheck SA1019 for exactly this call site via a scoped golangci exclusion (path + text `ScalarBaseMult`, quality bar #323, ticket #343) instead of a `nolint` directive.

## Considered Options

- **Migrate the VAPID sender key to `crypto/ecdh` end-to-end and drop `crypto/elliptic`** — removes the deprecated API at the cost of hand-rolling the ES256 signature path; planned as remediation ticket #345, which also deletes the exclusion.
- **Third-party JOSE library** — rejected: a new dependency for a single signature widens the dependency surface for no other gain.

## Consequences

- The exclusion cannot mask other SA1019 findings: it matches one file and one message text.
- When #345 lands, the rule must be deleted; `warn-unused` in the golangci config flags a stale rule automatically.

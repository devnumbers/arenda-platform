# ADR 0043: webpush VAPID signs ES256 locally over a crypto/ecdh key

The Web Push VAPID signer (`notifications/adapters/webpush/vapid.go`) signs ES256 JWTs (RFC 8292), but stdlib offers no ES256 signing over `crypto/ecdh` keys, and the only stdlib route to `ecdsa.PrivateKey` ran through the deprecated `crypto/elliptic` `ScalarBaseMult` (the interim state of ADR 0042, remediation ticket #345, grid #325). The sender key now stays a `crypto/ecdh` key end-to-end and the ECDSA signature equation is implemented locally: the nonce k is rejection-sampled from `crypto/rand` into [1, n−1], the nonce point k·G is derived through an ephemeral `ecdh` public key, and r, s follow the modular equation s = k⁻¹(e + r·d) mod n over `math/big` with the P-256 order pinned as a constant. This drops the deprecated API, the last `crypto/elliptic` import, and the SA1019 exclusion — without a new dependency.

## Considered Options

- **Keep `ScalarBaseMult` behind a scoped exclusion** — the ADR 0042 interim state; rejected as a permanent fix: the deprecated API can be removed from stdlib, and the exclusion is one more rule to keep honest.
- **Reconstruct `ecdsa.PrivateKey` from the ecdh-derived public point** — parsing X, Y out of `ecdh` public key bytes and calling `ecdsa.Sign` avoids the deprecated call while keeping stdlib signing, but it keeps the struct-reconstruction workaround the remediation set out to remove; the module leaves the ecdsa/elliptic pairing entirely only if signing moves onto the ecdh key.
- **Third-party JOSE library** — rejected (as in ADR 0042): a new dependency for a single signature widens the dependency surface for no other gain.

## Consequences

- Hand-rolled crypto is a liability, so it is deliberately tiny: one modular equation, one curve constant, zero point arithmetic — the only curve operation (k·G) is delegated to `crypto/ecdh`.
- Correctness is pinned by three independent checks in `vapid_test.go`: a known-answer vector computed by an external implementation for a fixed (key, nonce, digest) triple, cross-verification of 25 random signatures per run with stdlib `ecdsa.Verify`, and the pinned curve order compared against `elliptic.P256().Params().N` (test-only import; the production code has no `crypto/elliptic`).
- The nonce is uniform and rejection-sampled — no modulo bias; a zero r or s triggers a redraw (FIPS 186 §6.4.1, chance ~2⁻²⁵⁶).
- The secret-dependent scalar arithmetic (`r·d`, `k⁻¹` modulo n) runs on `math/big`, which is not constant-time — a paper regression from stdlib `ecdsa.Sign`'s constant-time P-256 path. Exposure is limited: signing is server-side, rare (one token per origin per 12 h), and the nonce is fresh per signature, so a practical timing oracle would need repeated same-shape observations; a constant-time port remains the escape hatch if that assessment changes.
- RFC 8291 payload encryption (`crypto.go`) is unaffected — it never left `crypto/ecdh`.

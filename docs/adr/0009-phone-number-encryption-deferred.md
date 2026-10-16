# ADR 0009: Phone Number Encryption Deferred

## Status

Accepted — encryption implemented (migration 000048); the original "deferred"
decision is superseded by the write-time encryption described below. A residual
risk is accepted: legacy plaintext rows created before encryption are never
re-encrypted (no startup backfill).

## Context

User phone numbers are used as the primary identifier for authentication. They are stored in PostgreSQL in the `users` and `login_attempts` tables (the `sms_codes` table was dropped earlier; the SMS channel itself is removed by ADR 0044).

Encrypting phone numbers at rest would reduce the impact of a database compromise, but it is a breaking data change:

- Existing rows would need to be migrated to ciphertext.
- All reads by phone number (login, reminders fan-out) would require decryption.
- Indexes and lookups by phone number would need to be redesigned (e.g., deterministic encryption or a separate search index).

## Decision

Defer phone-number encryption to a dedicated follow-up task. Document it as an accepted risk for the current phase.

Rationale:

- The MVP threat model treats the application database as a trusted boundary.
- Encryption significantly changes authentication lookup semantics and requires a migration path for existing users.
- Other controls (TLS in transit, limited database access, hashed session tokens) mitigate the most likely exposure paths in the short term.

## Consequences

- A database breach could expose user phone numbers.
- When encryption is implemented, it must cover `users.phone` and `login_attempts.phone`, and must include a migration that encrypts existing rows without downtime.
- The repository layer should use the existing `encryption.Encryptor` abstraction so the change is transparent to application services.

### Encryption implemented (migration 000048, phone_encrypted flag)

Phone-number encryption was later implemented. Each phone column gained a
`phone_encrypted BOOLEAN NOT NULL DEFAULT FALSE` flag (migration 000048). New
rows are deterministically encrypted at write time by the repository layer
(`PhoneEncrypted: !enc.IsNoop()`) and marked `phone_encrypted = true`. Reads
transparently decrypt when the flag is set and return plaintext otherwise, so
the column coexists with historical plaintext rows.

### Startup backfill removed (issue #216)

A startup-time `BackfillPhoneEncryption` that re-encrypted legacy plaintext rows
was added in `cmd/api/wire` and later **removed** (issue #216): it created
operational risks — a concurrent double-encrypt under multi-instance deploys, a
full table scan without batching on every boot, and a startup block when the
backfill errored mid-way. Phone encryption is now a write-time-only concern:
every newly created row is encrypted at insert/update.

**Accepted residual risk:** any rows created before encryption was enabled stay
in plaintext (`phone_encrypted = false`) forever — they are read transparently
(`decryptPhone` returns the value as-is when the flag is false) but are never
re-encrypted, since no backfill runs. If encrypting those rows later becomes a
requirement, it must be done as a controlled, one-shot operation (advisory lock
+ batching), not as a startup hook.

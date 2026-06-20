# ADR 0009: Phone Number Encryption Deferred

## Status

Accepted risk (deferred)

## Context

User phone numbers are used as the primary identifier for authentication and as a contact channel for reminders. They are currently stored in PostgreSQL as plaintext in the `users`, `sms_codes`, and `login_attempts` tables.

Encrypting phone numbers at rest would reduce the impact of a database compromise, but it is a breaking data change:

- Existing rows would need to be migrated to ciphertext.
- All reads by phone number (login, SMS sending, reminders) would require decryption.
- Indexes and lookups by phone number would need to be redesigned (e.g., deterministic encryption or a separate search index).

## Decision

Defer phone-number encryption to a dedicated follow-up task. Document it as an accepted risk for the current phase.

Rationale:

- The MVP threat model treats the application database as a trusted boundary.
- Encryption significantly changes authentication lookup semantics and requires a migration path for existing users.
- Other controls (TLS in transit, limited database access, hashed session tokens) mitigate the most likely exposure paths in the short term.

## Consequences

- A database breach could expose user phone numbers.
- When encryption is implemented, it must cover `users.phone`, `sms_codes.phone`, and `login_attempts.phone`, and must include a migration that encrypts existing rows without downtime.
- The repository layer should use the existing `encryption.Encryptor` abstraction so the change is transparent to application services.

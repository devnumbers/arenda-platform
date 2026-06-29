# Design: Enforce unique email across users

## Problem

The profile form allows any user to set an email that is already used by another user. The `users` table has no unique constraint on `email`, and `PATCH /me` does not check for duplicates.

## Decision

Enforce uniqueness at two levels:

1. **Application layer** — check for an existing user with the same email before saving, and return a clear error if found.
2. **Database layer** — add a `UNIQUE` constraint on `users.email` to guarantee integrity even if the application check is bypassed.

Also normalize emails to lowercase before storing, so `Ivan@Example.com` and `ivan@example.com` are treated as the same address.

## Backend design

### Database migration

Create migration `000049_user_email_unique`:

- **Up:** `ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);`
- **Down:** `ALTER TABLE users DROP CONSTRAINT users_email_unique;`

PostgreSQL considers multiple `NULL` values distinct, so users without an email are unaffected.

### Email normalization

In `domain.User.UpdatePersonalData`, trim and lower-case a non-empty email before validating and storing:

```go
v := strings.ToLower(strings.TrimSpace(email.Value))
```

### Lookup by email

Add `GetUserByEmail(ctx context.Context, email string) (domain.User, error)` to the `UserRepository` port and implement it via a new sqlc query:

```sql
-- name: GetUserByEmail :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted
FROM users WHERE email = $1;
```

### Duplicate check in `UpdateUser`

In `AuthService.UpdateUser`:

1. Apply personal data with `user.UpdatePersonalData(...)`.
2. If `cmd.Email.Set` and the resulting `user.Email` is non-nil:
   - Call `users.GetByEmail(ctx, *user.Email)`.
   - If a user is found and `user.ID != currentUserID`, return `ErrEmailAlreadyTaken`.
3. Save the user.

Add the error:

```go
var ErrEmailAlreadyTaken = errors.New("email already taken")
```

### HTTP error mapping

In `AuthHandlers.UpdateMe`, map `application.ErrEmailAlreadyTaken` to HTTP `409 Conflict` with the detail "Email is already in use".

### OpenAPI contract

Update `api/openapi/openapi.yaml`:

- Add `description` to `UserUpdateRequest.email` noting that emails are stored in lowercase.
- Add a `409` response to `PATCH /me` for the duplicate-email case.
- Regenerate generated code.

## Frontend design

In `PersonalDataForm.tsx`, handle the `409` response from `useUpdateMe`. If the mutation error has status `409`, show the message "Этот email уже используется" instead of the generic error text.

## Tests

- **Domain:** email is trimmed and lowercased.
- **Service:** updating to another user's email returns `ErrEmailAlreadyTaken`; updating to the user's own email succeeds; updating to a new unique email succeeds.
- **HTTP handler:** `PATCH /me` returns `409 Conflict` for a duplicate email.
- **Frontend manual check:** saving a duplicate email shows the Russian error message.

## Out of scope

- Deduplicating existing duplicate emails in production — must be done before the migration is applied.
- Case-insensitive unique index in PostgreSQL — application-level lowercasing is sufficient for now.

## Affected files

- `apps/backend/db/migrations/000049_user_email_unique.up.sql`
- `apps/backend/db/migrations/000049_user_email_unique.down.sql`
- `apps/backend/db/queries/identity.sql`
- `apps/backend/internal/platform/generated/postgres/identity.sql.go`
- `apps/backend/internal/identity/application/ports.go`
- `apps/backend/internal/identity/adapters/postgres/repository.go`
- `apps/backend/internal/identity/domain/user.go`
- `apps/backend/internal/identity/application/service.go`
- `apps/backend/internal/platform/httpapi/auth_handlers.go`
- `apps/backend/api/openapi/openapi.yaml`
- `apps/backend/internal/platform/openapi/generated.gen.go`
- `apps/backend/internal/identity/domain/user_test.go`
- `apps/backend/internal/identity/application/service_test.go`
- `apps/frontend/widgets/profile/ui/PersonalDataForm.tsx`
- `CHANGELOG.md`

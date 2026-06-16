# Phase 2: Clean Architecture & Clean Code — Implementation Plan

> **Goal:** Eliminate the clean-code debt found during code review (clock abstraction, duplicated helpers, raw-string enums, unused code) while keeping the public API and database schema unchanged.
>
> **Architecture:** Introduce small, platform-level helper packages (`timeutil`, `pgconv`, `phonenumber`) and a shared HTTP helper; inject `Clock` into all application services; tighten type safety in the `leases` domain by using typed enums; remove dead/duplicate code.
>
> **Tech Stack:** Go 1.26, pgx/v5, chi, sqlc, oapi-codegen.
>
> **Scope:** `apps/backend`. No tests, no OpenAPI spec changes, no database migrations.
>
> **Verification:** `make backend-lint`, `cd apps/backend && go build ./...`.

---

## Task 1: Inject `Clock` into all application services

**Files:**
- `apps/backend/cmd/api/main.go` — wire `realClock{}` into `httpapi.Deps` and service constructors.
- `apps/backend/internal/platform/httpapi/server.go` — pass `deps.Clock` into `SessionMiddleware`.
- `apps/backend/internal/platform/httpapi/session.go` — remove local `defaultClock`; keep fallback only when `clock == nil`.
- `apps/backend/internal/properties/application/service.go` — add `clock Clock`, replace `time.Now()` with `s.clock.Now()`.
- `apps/backend/internal/leases/application/service.go` — add `clock Clock`, replace `time.Now()`.
- `apps/backend/internal/leases/application/operation_service.go` — add `clock Clock`, replace `time.Now()`.
- `apps/backend/internal/leases/application/recurring_operation_service.go` — add `clock Clock`, replace `time.Now()`.
- `apps/backend/internal/leases/application/rent_service.go` — add `clock Clock`, replace `time.Now()`.
- `apps/backend/internal/leases/application/tenant_contact_service.go` — add `clock Clock` if needed.
- `apps/backend/internal/properties/application/ports.go` — no changes unless `Clock` import needed.

**Steps:**
1. Define (or reuse) `Clock` interface in `identity/application` and accept it in constructors.
2. Replace every `time.Now()` in application logic with `s.clock.Now()`.
3. Update `main.go` to pass `realClock{}` to all affected services and to `httpapi.Deps`.
4. Verify lint/build.

---

## Task 2: Extract shared UTC-date helper (`timeutil`)

**Files:**
- Create `apps/backend/internal/platform/timeutil/timeutil.go` with `func Date(t time.Time) time.Time`.
- `apps/backend/internal/properties/application/service.go` — remove local `date()`, use `timeutil.Date`.
- `apps/backend/internal/leases/domain/rent_schedule.go` — remove local `date()`, use `timeutil.Date`.
- `apps/backend/internal/leases/application/rent_service.go` — remove local `date()`, use `timeutil.Date`.
- `apps/backend/internal/leases/application/recurring_operation_service.go` — replace inline `date(...)` calls.

**Steps:**
1. Create package returning UTC midnight truncation.
2. Replace all three duplicate implementations and inline usages.
3. Verify lint/build.

---

## Task 3: Extract shared pgx conversion helpers (`pgconv`)

**Files:**
- Create `apps/backend/internal/platform/database/pgconv/pgconv.go` with:
  - `UUIDToPgtype`, `UUIDFromPgtype`
  - `TextToString`, `StringPtrToPgtype`
  - `DateToPgtype`, `DatePtrToPgtype`
  - `TimestamptzToTime`, `TimestamptzToPtrTime`
- `apps/backend/internal/identity/adapters/postgres/repository.go` — replace local helpers with `pgconv`.
- `apps/backend/internal/properties/adapters/postgres/repository.go` — replace local helpers.
- `apps/backend/internal/leases/adapters/postgres/repository.go` — replace local helpers.

**Steps:**
1. Audit each adapter for duplicated conversion functions.
2. Move them to `pgconv` with clear names.
3. Replace local copies; keep behavior identical (including current empty-string handling unless changed by Task 7).
4. Verify lint/build.

---

## Task 4: Consolidate phone normalization

**Files:**
- Create `apps/backend/internal/shared/phone/phone.go` with `Normalize`, `Validate`, `NewPhone` (shared value object, not platform).
- `apps/backend/internal/identity/domain/phone.go` — delegate to `shared/phone` or replace usages.
- `apps/backend/internal/leases/domain/tenant_contact.go` — replace `NormalizePhone`/`ValidatePhone` with shared package.
- Update callers and constructors accordingly.

**Steps:**
1. Extract regex/normalization logic into `shared/phone`.
2. Update identity and leases domains to use the shared value object.
3. Verify lint/build.

---

## Task 5: Use typed enums in `leases` domain structs

**Files:**
- `apps/backend/internal/leases/domain/operation.go` — change `Operation.Type` to `OperationType`, `Operation.Category` to `OperationCategory`.
- `apps/backend/internal/leases/domain/recurring_operation.go` — change `Type`, `Category`, `Periodicity`, `Status` to typed enums.
- `apps/backend/internal/leases/adapters/postgres/repository.go` — convert to/from string at persistence edge.
- `apps/backend/internal/leases/application/*.go` — update construction and comparisons.
- `apps/backend/internal/platform/httpapi/*.go` — update mapping from OpenAPI strings to typed enums.

**Steps:**
1. Change struct field types to existing typed enums.
2. Update all assignments and comparisons.
3. Keep repository SQL using strings (sqlc generates string columns).
4. Verify lint/build.

---

## Task 6: Deduplicate HTTP and application helpers

**Files:**
- `apps/backend/internal/platform/httpapi/context.go` — create package-level `ownerIDFromContext(w, r)` helper.
- `apps/backend/internal/platform/httpapi/property_handlers.go` — remove duplicate method.
- `apps/backend/internal/platform/httpapi/lease_handlers.go` — remove duplicate method.
- `apps/backend/internal/platform/httpapi/operation_handlers.go` — remove duplicate method.
- `apps/backend/internal/platform/httpapi/recurring_operation_handlers.go` — remove duplicate method.
- `apps/backend/internal/leases/application/operation_service.go` and `recurring_operation_service.go` — extract shared `parseTypeAndCategory` and `validateProperty` helpers into a small internal package or unexported funcs in `leases/application/shared.go`.
- `apps/backend/internal/billing/application/service.go` — delete unused application service (logic duplicated by `billing/adapters/postgres/onboarding.go`).

**Steps:**
1. Extract `ownerIDFromContext` to package level.
2. Extract `leases/application` shared helpers.
3. Remove dead billing application service or wire it; prefer deletion to avoid duplication.
4. Verify lint/build.

---

## Task 7: Minor cleanups

**Files:**
- `apps/backend/internal/identity/adapters/postgres/repository.go` — fix `pgtypeTextPtr` to return `&t.String` when `t.Valid` (preserve empty strings).
- `apps/backend/internal/leases/adapters/postgres/repository.go` — handle `ParseLeaseStatus` error instead of discarding it.
- `apps/backend/internal/properties/adapters/postgres/repository.go` — replace `"active"` literal with `domain.PropertyStatusActive`.
- `apps/backend/internal/platform/httpapi/server.go` — wire `deps.Clock` (Task 1) and remove unused `defaultClock`.

**Steps:**
1. Apply minor fixes.
2. Verify lint/build.

---

## Task 8: Final verification

**Commands:**
```bash
cd apps/backend && go build ./...
cd /Users/smirnowwwivan/Nambers/arenda-planform && make backend-lint
```

Expected: build succeeds, lint reports 0 issues.

---

## Out of Scope (Phase 3)

- `PropertyBillingLifecycle` port collapse.
- `ScheduleMaterializer` extraction.
- `RentService` transaction-bound refactor.
- `OccupancyProvider` split.
- Read-side mutation removal (GET writing DB).
- Transaction-port leakage (`Tx` in cross-context ports).

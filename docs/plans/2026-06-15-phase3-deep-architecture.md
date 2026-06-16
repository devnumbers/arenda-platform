# Phase 3: Deep Architecture — Implementation Plan

> **Goal:** Fix the deepest architectural seams identified in the code review: split the transaction package into a driver-free port and a pgx adapter, collapse the three shallow property-side billing adapters into one semantic `PropertyBillingLifecycle` port, and narrow the `OccupancyProvider` seam.
>
> **Architecture:** Keep the modular-monolith boundaries but replace shallow, repository-shaped ports with deep, semantic ports. Move driver-specific transaction creation to the platform/postgres adapter. Move leases-side archive/unarchive side effects behind a single lifecycle port owned by the properties context but implemented in the leases context.
>
> **Tech Stack:** Go 1.26, pgx/v5, sqlc.
>
> **Scope:** `apps/backend`. No tests, no OpenAPI spec changes, no database migrations.
>
> **Verification:** `make backend-lint`, `cd apps/backend && go build ./...`.

---

## Task 1: Split `internal/transaction` into driver-free port and pgx adapter

**Files:**
- Modify `apps/backend/internal/transaction/transaction.go` — keep `Tx` and add `Beginner` interface; remove `pgxpool` import and `NewBeginner`/`Begin` struct.
- Create `apps/backend/internal/platform/database/postgres/transaction.go` — implement `transaction.Beginner` with `NewBeginner(pool *pgxpool.Pool) transaction.Beginner`.
- Modify `apps/backend/cmd/api/main.go` — import `platform/database/postgres` and use `postgres.NewBeginner(pool)` instead of `transaction.NewBeginner(pool)`.

**Steps:**
1. In `transaction.go`, define:
   ```go
   type Tx interface { Commit(ctx context.Context) error; Rollback(ctx context.Context) error }
   type Beginner interface { Begin(ctx context.Context) (Tx, error) }
   ```
2. Move concrete implementation to `platform/database/postgres/transaction.go`.
3. Update all `transaction.NewBeginner(pool)` calls in `main.go`.
4. Verify lint/build.

---

## Task 2: Introduce `PropertyBillingLifecycle` port

**Files:**
- Modify `apps/backend/internal/properties/application/ports.go`:
  - Remove `OperationArchiver`, `RecurringOperationStatusUpdater`, `RecurringOperationScheduler`, and the `RecurringOperation` DTO.
  - Add:
    ```go
    type PropertyBillingLifecycle interface {
        Suspend(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID, asOf time.Time) error
        Resume(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID, ownerID uuid.UUID, asOf time.Time) error
        WithTx(tx transaction.Tx) PropertyBillingLifecycle
    }
    ```
- Modify `apps/backend/internal/properties/application/service.go`:
  - Replace the three adapter fields with one `billingLifecycle PropertyBillingLifecycle`.
  - Update constructor signature.
  - In `ArchiveProperty`, after occupancy check call `s.billingLifecycle.Suspend(ctx, tx, id, timeutil.Date(s.clock.Now()))` instead of delete/pause loop.
  - In `UnarchiveProperty`, after unarchive call `s.billingLifecycle.Resume(ctx, tx, id, ownerID, timeutil.Date(s.clock.Now()))` instead of resume/generate loop.
- Create `apps/backend/internal/leases/adapters/postgres/property_billing_lifecycle.go`:
  - Implement `PropertyBillingLifecycle` using leases `OperationRepository` and `RecurringOperationRepository`.
  - `Suspend` deletes future unedited operations and pauses recurring operations.
  - `Resume` activates recurring operations and generates missing operations up to 12 months.
- Modify `apps/backend/cmd/api/main.go`:
  - Remove construction of `propertyOperationArchiver`, `propertyRecurringOpUpdater`, `propertyRecurringOpScheduler`.
  - Construct `propertyBillingLifecycle := leasespg.NewPropertyBillingLifecycle(operationRepo, recurringOpRepo, realClock{})`.
  - Pass it to `propertiesapp.NewPropertyService`.
- Delete `apps/backend/internal/leases/adapters/postgres/property_adapter.go`.

**Steps:**
1. Define the new port.
2. Implement it in leases adapters.
3. Refactor `PropertyService` to use the port.
4. Update wiring in `main.go`.
5. Delete old shallow adapters.
6. Verify lint/build.

---

## Task 3: Split `OccupancyProvider` seam

**Files:**
- Modify `apps/backend/internal/properties/application/ports.go`:
  - Replace `OccupancyProvider` with:
    ```go
    type OccupancyProvider interface {
        IsOccupied(ctx context.Context, ownerID, propertyID uuid.UUID) (bool, error)
        OccupiedPropertyIDs(ctx context.Context, ownerID uuid.UUID) (map[uuid.UUID]bool, error)
    }
    ```
- Modify `apps/backend/internal/properties/adapters/postgres/occupancy.go`:
  - Implement both methods. `IsOccupied` checks a single property; `OccupiedPropertyIDs` returns the map for lists.
- Modify `apps/backend/internal/properties/application/service.go`:
  - In single-property callers (`GetProperty`, `UpdateProperty` status check, `ArchiveProperty`), use `s.occupancyProvider.IsOccupied(ctx, ownerID, property.ID)`.
  - In `ListProperties`, keep using `OccupiedPropertyIDs`.

**Steps:**
1. Update interface.
2. Update adapter implementation.
3. Update callers.
4. Verify lint/build.

---

## Task 4: Final verification

**Commands:**
```bash
cd apps/backend && go build ./...
cd /Users/smirnowwwivan/Nambers/arenda-planform && make backend-lint
```

Expected: build succeeds, lint reports 0 issues.

---

## Out of Scope (future)

- Extracting a shared `ScheduleMaterializer` for operation generation across `RentService`, `RecurringOperationService`, and `PropertyBillingLifecycle`.
- Removing read-side mutations (`GetLease`/`ListLeases` status recalculation, `ListRecurringOperationsByProperty` horizon extension).
- Removing `transaction.Tx` from cross-context ports such as `OnboardingService` and `RecurringOperationScheduler` (handled in this phase for `PropertyBillingLifecycle` only).

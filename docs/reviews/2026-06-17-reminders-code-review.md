# Code Review: Reminders / Notifications Feature

**Date:** 2026-06-17  
**Scope:** All changes in `024107fb..a6165dfb` (reminders/notifications subsystem on `master`).  
**Reviewers:** Multi-agent review (Go architecture, leases integration, PostgreSQL, HTTP/OpenAPI).  
**Status:** ✅ All Critical and Important issues resolved in follow-up commits up to `a7bfcc3`.

---

## Executive Summary

The implementation follows Clean Architecture and is mostly consistent with the approved design doc. The domain/application/adapters layering is clean, transactional lifecycle coupling is in place, and the worker uses the correct `FOR UPDATE SKIP LOCKED` claim pattern. However, several correctness, concurrency, and schema issues remain that can cause duplicate sends, failed lease updates, or inconsistent data.

**Recommendation:** Fix the Critical issues, then the Important issues, before declaring the feature complete.

---

## Strengths

1. **Clean Architecture.** `internal/notifications/domain` is pure; ports live in application; adapters depend inward.
2. **Transactional coupling.** Other bounded contexts call `ReminderScheduler.WithTx(tx)` so reminders are created/cancelled atomically with operations/leases.
3. **Worker claim pattern.** `ListDueReminders` uses `FOR UPDATE SKIP LOCKED`; reminders move to `sending`; stale `sending` recovery exists.
4. **No PII in logs.** Worker and notifier log only IDs/event types; phone is stored only in the audit table.
5. **OpenAPI-first contract.** Handlers reuse generated DTOs and RFC 7807 problem responses.
6. **Partial indexes.** `idx_reminders_due`, `idx_reminders_active_unique`, and `sent_sms_reminders` unique index are appropriate for the hot paths.

---

## Critical Issues

### C1. Worker is not idempotent — duplicate SMS sends under concurrency
- **Files:** `internal/platform/scheduler/reminder_worker.go` (`finalizeSuccess`, `recoverFinalizeFailure`), `db/queries/notifications.sql` (`CreateSentSMSReminder`), `db/migrations/000012_sent_sms_reminder_unique.up.sql`
- **Problem:** A worker can reset a `sending` reminder to `pending` while another worker has already sent it and inserted the audit row. The next attempt sends again; the audit insert conflicts; finalize fails; the reminder is reset to `pending` again. This creates a retry loop and duplicate sends.
- **Fix:** Before dispatching, check `ExistsSentSMSReminder` and idempotently mark the reminder `sent` instead of notifying. Add `ON CONFLICT (reminder_id) DO NOTHING` to `CreateSentSMSReminder` and make `MarkSent` succeed when the reminder is already `sent`.

### C2. `ReminderService` batch methods start independent transactions
- **File:** `internal/notifications/application/service.go` (`CreateForRecurringOperation`, `CreateForLease`, `Cancel`)
- **Problem:** These methods call `s.beginner.Begin(ctx)` even when invoked on a `WithTx(tx)` service, so they commit independently of the caller’s transaction. This breaks the design promise that reminder mutations happen inside the same transaction as the owning entity, and risks deadlocks.
- **Fix:** Remove transaction management from these methods; let callers provide a transaction (as the `scheduler` already does), or make them transaction-aware by reusing a stored `tx`.

### C3. `POST /operations/{id}/reminders` does not replace existing reminders and returns 500 on conflict
- **Files:** `internal/platform/httpapi/reminder_handlers.go` (`CreateOperationReminder`), `internal/notifications/application/service.go` (`CreateForOperation`)
- **Problem:** `CreateForOperation` simply inserts. If a pending reminder already exists, the active unique index raises a duplicate-key error returned as 500. The endpoint is not idempotent.
- **Fix:** Use `ScheduleForOperation` (cancel-then-save) or implement an upsert so the endpoint replaces the existing pending reminder.

### C4. Incomplete `exactly_one_target` CHECK constraint
- **File:** `db/migrations/000008_notifications.up.sql`
- **Problem:** The `operation` branch does not require `recurring_operation_id IS NULL`, allowing an invalid row with both `operation_id` and `recurring_operation_id` set.
- **Fix:** Add `AND recurring_operation_id IS NULL` to the operation branch.

### C5. Overly permissive `UpdateReminder` query
- **File:** `db/queries/notifications.sql` (`UpdateReminder`)
- **Problem:** Updates immutable fields (`owner_id`, target FKs, `event_type`, `created_at`) and lacks an owner/status guard, allowing a sent/cancelled reminder to be overwritten or moved between owners.
- **Fix:** Restrict to mutable fields only (`scheduled_at`, `message_title`, `message_body`) and add `WHERE id = $1 AND owner_id = $2`. If unused, delete it.

### C6. Unique index on recurring-operation operations ignores soft deletes
- **File:** `db/migrations/000014_recurring_operation_date_unique.up.sql`
- **Problem:** `idx_operations_recurring_date_unique` does not exclude `deleted_at IS NOT NULL`, so a soft-deleted operation can block regeneration of the same date and behaviour is inconsistent with queries that filter `deleted_at IS NULL`.
- **Fix:** Make the partial condition `WHERE recurring_operation_id IS NOT NULL AND deleted_at IS NULL`.

### C7. Stale lease reminders when `EndDate` is cleared
- **File:** `internal/leases/application/service.go` (`UpdateLease`)
- **Problem:** Lease reminders are only rescheduled when `cmd.EndDate != nil`. Changing `EndDate` from a date to `nil` leaves existing `lease_expiring` / `lease_requires_action` reminders pending.
- **Fix:** Cancel lease reminders whenever `cmd.EndDate` differs from the original; reschedule only when the new value is non-nil.

### C8. Recurring-operation reminders are not cancelled on lease completion
- **File:** `internal/leases/application/service.go` (`CompleteLease`)
- **Problem:** `CompleteLease` pauses the recurring operation but only calls `CancelByLease`. Future `operation_due` reminders for the recurring operation stay pending.
- **Fix:** Fetch the lease’s recurring operation and call `CancelByRecurringOperation` before committing.

---

## Important Issues

### I1. `ScheduleForLease` fails for leases with a past end date
- **File:** `internal/notifications/application/service.go` (`buildLeaseReminders`, `ScheduleForLease`)
- **Problem:** The `requires_action` reminder is always created for `end_date + 1 day`. If that is in the past, validation rejects it and the lease create/update fails.
- **Fix:** Clamp the `requires_action` date to `max(end_date + 1 day, now)`, consistent with `EnsureRequiresActionReminder`.

### I2. Updating a manual operation’s date leaves its user-created reminder stale
- **File:** `internal/leases/application/operation_service.go` (`UpdateOperation`)
- **Problem:** `UpdateOperation` only reschedules reminders for recurring-operation-generated operations. A reminder created via `POST /operations/{id}/reminders` for a manual operation is cancelled but never recreated.
- **Fix:** When the operation date changes and a pending reminder exists, cancel and call `ScheduleForOperation` with the new date regardless of `RecurringOperationID`.

### I3. Updating a recurring-op-generated operation can fail inside the reminder offset window
- **File:** `internal/leases/application/operation_service.go` (`UpdateOperation`)
- **Problem:** The new reminder date is computed as `operationDate - offsetDays`. If the operation is moved within `offsetDays` of today, the reminder date is in the past and `ScheduleForOperation` returns `ErrInvalidReminderDate`, rolling back the update.
- **Fix:** Skip scheduling when the computed reminder date is before today, matching the filtering in `scheduleRemindersForOperations`.

### I4. Cancellation SQL only targets `pending` reminders
- **Files:** `db/queries/notifications.sql` (`CancelReminderByTarget`, `CancelRemindersByRecurringOperationID`)
- **Problem:** A reminder in `sending` state is not cancelled, so it can still be dispatched after the underlying entity has changed.
- **Fix:** Include `sending` in the cancellation predicate, or have the worker re-validate the target before notifying.

### I5. Worker timeout does not cover contact resolution or DB finalization
- **File:** `internal/platform/scheduler/reminder_worker.go` (`dispatchReminder`)
- **Problem:** `dispatchTimeout` is applied only to `notifier.Notify`. `resolver.Resolve` and the finalize transaction can block indefinitely.
- **Fix:** Wrap the entire `Resolve → Notify → finalize` sequence in the timeout context.

### I6. Nested list endpoints silently drop reminders
- **File:** `internal/platform/httpapi/reminder_handlers.go` (`ListOperationReminders`, `ListRecurringOperationReminders`, `ListLeaseReminders`)
- **Problem:** They fetch up to 1000 owner reminders and filter in Go. Owners with more than 1000 reminders will see missing items.
- **Fix:** Add repository/application methods `ListByOperation`, `ListByRecurringOperation`, `ListByLease` that filter in SQL.

### I7. `CreateRecurringOperationReminder` returns a single arbitrary reminder after batch creation
- **File:** `internal/platform/httpapi/reminder_handlers.go` (`CreateRecurringOperationReminder`)
- **Problem:** The endpoint creates concrete reminders for all future generated operations but returns `created[0]`.
- **Fix:** Change the response schema to `RemindersResponse` and return the full list, or redesign the endpoint to return recurring-operation-level metadata.

### I8. `GET /reminders` lacks pagination
- **Files:** `internal/platform/httpapi/reminder_handlers.go` (`ListReminders`), `api/openapi/openapi.yaml`
- **Problem:** Hardcoded `Limit: 100, Offset: 0` with no query parameters.
- **Fix:** Add optional `limit`/`offset` query parameters and pass them through `ListFilter`.

### I9. `event_date` added without NOT NULL or design doc alignment
- **File:** `db/migrations/000009_reminders_event_date.up.sql`
- **Problem:** The column is nullable and not described in the design doc, creating latent data-integrity risk.
- **Fix:** Either make it `NOT NULL` after backfill, or remove it and derive the date from `scheduled_at`.

### I10. `GetReminderByID` lacks owner scoping
- **File:** `db/queries/notifications.sql` (`GetReminderByID`)
- **Problem:** Reading by ID only can leak across owners if the repository layer ever calls it without an ownership check.
- **Fix:** Add `owner_id` parameter: `WHERE id = $1 AND owner_id = $2`. If an unscoped lookup is needed for the worker, create a separate internal query.

### I11. `reminder_offset_days` lacks CHECK constraint
- **File:** `db/migrations/000013_recurring_operations_reminder_offset.up.sql`
- **Problem:** `INT` with no constraint allows negative or huge values.
- **Fix:** Add `CHECK (reminder_offset_days >= 0)` and clarify nullability semantics.

### I12. `HasReminderForLeaseEvent` semantics are unclear
- **File:** `db/queries/notifications.sql` (`HasReminderForLeaseEvent`)
- **Problem:** It checks `status != 'cancelled'`, which includes `failed`, blocking re-creation despite failed reminders being terminal.
- **Fix:** Define the intended set explicitly, e.g. `status IN ('pending', 'sending', 'sent')`.

### I13. `reminders` table bypasses project `updated_at` trigger convention
- **File:** `db/migrations/000008_notifications.up.sql`
- **Problem:** Other tables use `trg_*_updated_at`; `reminders` sets `updated_at` manually in each query.
- **Fix:** Add `trg_reminders_updated_at` and remove manual `updated_at = NOW()` from queries.

### I14. API paths diverge from the approved design doc
- **File:** `api/openapi/openapi.yaml`
- **Problem:** Design doc specifies flat `/operations/{id}/reminders` and `/recurring-operations/{id}/reminders`; implementation nests under `/properties/{propertyId}/...`.
- **Fix:** Align OpenAPI with the design doc, or update the design doc if the nesting was intentional.

### I15. Recurring-reminder offset logic lives in the HTTP handler
- **File:** `internal/platform/httpapi/reminder_handlers.go` (`CreateRecurringOperationReminder`)
- **Problem:** The handler filters operations, computes `offsetDays`, and calls `notificationsdomain.ReminderOffset` directly, coupling transport to domain packages.
- **Fix:** Introduce an application command `CreateRecurringReminder` that encapsulates the logic; the handler should only decode, authorize, call, and map.

---

## Minor Issues

1. **`sending` status exposed externally but not in design doc.** Either map `sending` to `pending` in API responses or update the design doc.
2. **`idx_leases_open_past_end` is not selective.** Make it partial on `status IN ('awaiting_start','active')`.
3. **Migration `000011_add_sending_status.down.sql` is not reversible.** Document the no-op or recreate the enum if full reversibility is required.
4. **Migration `000015_fix_reminder_unique.down.sql` can fail if duplicates exist.** Document hazard or deduplicate first.
5. **`ReminderRepository.Update` can mutate ownership/target keys.** Remove if unused or restrict to mutable fields.
6. **Redundant SMS-sender constructor alias** in `internal/platform/notifications/sms_sender.go`.
7. **`buildOperationReminder` formats amount via `float64`.** Use integer division/modulo for kopecks formatting.
8. **Duplicate `leaseIDPtr` helper.** Move to a shared leases package.
9. **`MarkSendingReminderPending` hardcodes retry interval.** Accept `next_attempt_at` as a parameter.
10. **`ReminderResponse` omits useful fields** (`property_id`, `event_date`, `failed_attempts`, `next_attempt_at`).
11. **Filter helpers pre-allocate slices without capacity.** Use `make([]T, 0, len(src))`.
12. **`ErrConcurrentUpdate` not handled in HTTP layer.** Map to 409 and add OpenAPI response.
13. **Not-found responses leak raw error strings.** Return stable generic messages.
14. **No HTTP-level tests for new endpoints.** Add handler tests with fakes.
15. **`sent_sms_reminders` stores PII in plaintext.** Flag for future at-rest encryption.

---

## Verification Commands Used

```bash
cd apps/backend && go build ./... && go vet ./...
cd ../.. && make backend-lint
```

Both passed at the time of review (`0 issues`).

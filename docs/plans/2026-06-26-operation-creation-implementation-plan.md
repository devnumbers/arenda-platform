# Operation Creation Page Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a 3-step wizard at `/finance/create-operation` that lets the owner create a single operation or a recurring operation (monthly/yearly) with an optional SMS reminder, while adding the missing `name` field to the backend Operation and RecurringOperation models.

**Architecture:** Extend the backend leases domain with a `name` column, expose it through OpenAPI, and implement a client-side wizard that reuses existing Hero UI patterns and TanStack Query hooks. The wizard decides whether to call the single-operation or recurring-operation endpoint based on the selected frequency.

**Tech Stack:** Next.js 16 + React 19 + TypeScript 5 + Hero UI v3, Go 1.26 + sqlc + oapi-codegen, PostgreSQL.

---

## Pre-requisites

Read before starting:
- `docs/plans/2026-06-26-operation-creation-design.md`
- `docs/service-functionality/operations/sozdanie-operacii.md`
- `docs/service-functionality/operations/reguljarnye-operacii.md`
- `docs/service-functionality/reminders/sozdanie-napominanija.md`
- `docs/adr/0012-operation-status-model.md`

Branch: work in the current `master` checkout (no worktree per project rules).

---

## Backend: add `name` to Operation

### Task 1: Database migration for operation name

**Files:**
- Create: `apps/backend/db/migrations/000040_add_operation_name.up.sql`
- Create: `apps/backend/db/migrations/000040_add_operation_name.down.sql`

**Step 1: Write up migration**

```sql
ALTER TABLE operations ADD COLUMN name TEXT NOT NULL DEFAULT '';
```

**Step 2: Write down migration**

```sql
ALTER TABLE operations DROP COLUMN name;
```

**Step 3: Run migrations locally**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
make local-infra-up
# apply migrations via your normal migration tool or backend startup
```

**Step 4: Commit**

```bash
git add apps/backend/db/migrations/000040_add_operation_name.*
git commit -m "feat: add name column to operations table"
```

---

### Task 2: Add `name` to domain Operation

**Files:**
- Modify: `apps/backend/internal/leases/domain/operation.go`

**Step 1: Add Name field to Operation struct**

```go
type Operation struct {
    ID                   uuid.UUID
    OwnerID              uuid.UUID
    PropertyID           uuid.UUID
    LeaseID              uuid.UUID
    RecurringOperationID uuid.UUID
    Type                 OperationType
    Category             OperationCategory
    Status               OperationStatus
    Name                 string
    AmountKopecks        int64
    OperationDate        time.Time
    Comment              string
    IsException          bool
    DeletedAt            *time.Time
    CreatedAt            time.Time
    UpdatedAt            time.Time
}
```

**Step 2: Build to verify**

```bash
cd apps/backend && go build ./internal/leases/domain/...
```

**Step 3: Commit**

```bash
git add apps/backend/internal/leases/domain/operation.go
git commit -m "feat: add Name field to domain Operation"
```

---

### Task 3: Update sqlc queries for operations

**Files:**
- Modify: `apps/backend/db/queries/operations.sql`

**Step 1: Update CreateOperation query**

```sql
-- name: CreateOperation :one
INSERT INTO operations (
    owner_id, property_id, lease_id, recurring_operation_id,
    type, category, name, amount_kopecks, operation_date, comment, is_exception, status
)
VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;
```

**Step 2: Update UpdateOperation query**

```sql
-- name: UpdateOperation :one
UPDATE operations
SET type = $3,
    category = $4,
    name = $5,
    amount_kopecks = $6,
    operation_date = $7,
    comment = $8,
    lease_id = $9,
    status = $10,
    is_exception = true
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL
RETURNING *;
```

**Step 3: Update all SELECT queries that list columns**

Add `name` after `category` in the column lists of:
- `ListOperationsByLease`
- `ListOperationsByRecurringOperation`
- `ListFutureOperationsByLease`
- `ListOperationsByProperty`
- `ListOperationsByPropertyWithStatuses`
- `GetOperationByIDAndOwner`
- `GetOperationByIDAndOwnerForUpdate`

**Step 4: Regenerate sqlc**

```bash
cd apps/backend
sqlc generate
```

**Step 5: Build**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add apps/backend/db/queries/operations.sql apps/backend/internal/platform/generated/postgres/operations.sql.go apps/backend/internal/platform/generated/postgres/querier.go
git commit -m "feat: include name in operation sqlc queries and generated code"
```

---

### Task 4: Update OperationService commands

**Files:**
- Modify: `apps/backend/internal/leases/application/operation_service.go`

**Step 1: Add Name to CreateOperationCommand**

```go
type CreateOperationCommand struct {
    PropertyID    uuid.UUID
    Type          string
    Category      string
    Name          string
    AmountKopecks int64
    OperationDate time.Time
    Comment       *string
    LeaseID       *uuid.UUID
}
```

**Step 2: Add Name to UpdateOperationCommand**

```go
type UpdateOperationCommand struct {
    Type          *string
    Category      *string
    Name          *string
    AmountKopecks *int64
    OperationDate *time.Time
    Comment       *string
    LeaseID       *uuid.UUID
}
```

**Step 3: Use Name in CreateOperation**

In `CreateOperation`, set `op.Name = cmd.Name` (trimmed, max 50 runes).

```go
op := domain.Operation{
    ...
    Name:          strings.TrimSpace(cmd.Name),
    ...
}
```

**Step 4: Use Name in UpdateOperation**

```go
if cmd.Name != nil {
    op.Name = strings.TrimSpace(*cmd.Name)
}
```

**Step 5: Validate Name in CreateOperation**

After parsing type/category, validate:

```go
if strings.TrimSpace(cmd.Name) == "" {
    return domain.Operation{}, newInvalidInputError("name is required")
}
if len([]rune(strings.TrimSpace(cmd.Name))) > 50 {
    return domain.Operation{}, newInvalidInputError("name must be at most 50 characters")
}
```

**Step 6: Build and test**

```bash
cd apps/backend
go build ./...
go test ./internal/leases/application/...
```

**Step 7: Commit**

```bash
git add apps/backend/internal/leases/application/operation_service.go
git commit -m "feat: handle name in operation service commands"
```

---

### Task 5: Update OpenAPI operation schemas

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Add name to OperationCreateRequest**

```yaml
    OperationCreateRequest:
      type: object
      required: [type, category, name, amount_kopecks, operation_date]
      properties:
        type:
          $ref: '#/components/schemas/OperationType'
        category:
          $ref: '#/components/schemas/OperationCategory'
        name:
          type: string
          maxLength: 50
        amount_kopecks:
          type: integer
          minimum: 0
        operation_date:
          type: string
          format: date
        comment:
          type: string
          maxLength: 2000
        lease_id:
          type: string
          format: uuid
```

**Step 2: Add name to OperationUpdateRequest**

```yaml
    OperationUpdateRequest:
      type: object
      properties:
        type:
          $ref: '#/components/schemas/OperationType'
        category:
          $ref: '#/components/schemas/OperationCategory'
        name:
          type: string
          maxLength: 50
        amount_kopecks:
          type: integer
        operation_date:
          type: string
          format: date
        comment:
          type: string
          maxLength: 2000
        lease_id:
          type: string
          format: uuid
```

**Step 3: Add name to OperationResponse**

```yaml
    OperationResponse:
      type: object
      required: [id, owner_id, property_id, type, category, name, amount_kopecks, operation_date, status, is_exception, created_at, updated_at]
      properties:
        ...
        category:
          $ref: '#/components/schemas/OperationCategory'
        name:
          type: string
        amount_kopecks:
          type: integer
        ...
```

**Step 4: Regenerate Go server interface**

```bash
cd apps/backend
make generate-openapi  # or your project command
```

**Step 5: Build**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add apps/backend/api/openapi/openapi.yaml apps/backend/internal/platform/openapi/generated.gen.go
git commit -m "feat: add name to operation openapi schemas"
```

---

### Task 6: Update operation handlers mapping

**Files:**
- Modify: `apps/backend/internal/platform/httpapi/operation_handlers.go`

**Step 1: Map name in CreateOperation**

```go
cmd := leasesapp.CreateOperationCommand{
    PropertyID:    propertyId,
    Type:          string(body.Type),
    Category:      string(body.Category),
    Name:          body.Name,
    AmountKopecks: int64(body.AmountKopecks),
    OperationDate: body.OperationDate.Time,
    LeaseID:       uuidPtrFromOpenAPI(body.LeaseId),
}
```

**Step 2: Map name in UpdateOperation**

```go
cmd := leasesapp.UpdateOperationCommand{
    Type:     ptrString(body.Type),
    Category: ptrString(body.Category),
    Name:     body.Name,
    Comment:  body.Comment,
    LeaseID:  uuidPtrFromOpenAPI(body.LeaseId),
}
```

**Step 3: Map name in operationResponse**

```go
resp := openapi.OperationResponse{
    ...
    Category:      openapi.OperationCategory(op.Category),
    Name:          op.Name,
    AmountKopecks: int(op.AmountKopecks),
    ...
}
```

**Step 4: Build and test**

```bash
cd apps/backend
go build ./...
go test ./internal/platform/httpapi/...
```

**Step 5: Commit**

```bash
git add apps/backend/internal/platform/httpapi/operation_handlers.go
git commit -m "feat: map operation name in http handlers"
```

---

## Backend: add `name` to RecurringOperation

### Task 7: Database migration for recurring operation name

**Files:**
- Create: `apps/backend/db/migrations/000041_add_recurring_operation_name.up.sql`
- Create: `apps/backend/db/migrations/000041_add_recurring_operation_name.down.sql`

**Step 1: Write migrations**

```sql
-- up
ALTER TABLE recurring_operations ADD COLUMN name TEXT NOT NULL DEFAULT '';

-- down
ALTER TABLE recurring_operations DROP COLUMN name;
```

**Step 2: Commit**

```bash
git add apps/backend/db/migrations/000041_add_recurring_operation_name.*
git commit -m "feat: add name column to recurring_operations table"
```

---

### Task 8: Add `name` to domain RecurringOperation

**Files:**
- Modify: `apps/backend/internal/leases/domain/operation.go`

**Step 1: Add Name field**

```go
type RecurringOperation struct {
    ID                 uuid.UUID
    OwnerID            uuid.UUID
    PropertyID         uuid.UUID
    LeaseID            uuid.UUID
    Type               OperationType
    Category           OperationCategory
    Name               string
    AmountKopecks      int64
    StartDate          time.Time
    PaymentDay         int
    EndDate            *time.Time
    ReminderOffsetDays *int
    Periodicity        RecurringOperationPeriodicity
    Status             RecurringOperationStatus
    Comment            string
    CreatedAt          time.Time
    UpdatedAt          time.Time
}
```

**Step 2: Commit**

```bash
git add apps/backend/internal/leases/domain/operation.go
git commit -m "feat: add Name field to domain RecurringOperation"
```

---

### Task 9: Update sqlc queries for recurring operations

**Files:**
- Modify: `apps/backend/db/queries/recurring_operations.sql`

**Step 1: Add `name` to all INSERT/UPDATE/SELECT queries**

Follow the same pattern as Task 3. Add `name` after `category` in column lists.

**Step 2: Regenerate sqlc**

```bash
cd apps/backend
sqlc generate
```

**Step 3: Build and commit**

```bash
go build ./...
git add apps/backend/db/queries/recurring_operations.sql apps/backend/internal/platform/generated/postgres/recurring_operations.sql.go apps/backend/internal/platform/generated/postgres/querier.go
git commit -m "feat: include name in recurring operation sqlc queries"
```

---

### Task 10: Update RecurringOperationService commands

**Files:**
- Modify: `apps/backend/internal/leases/application/recurring_operation_service.go`

**Step 1: Add Name to commands**

```go
type CreateRecurringOperationCommand struct {
    PropertyID    uuid.UUID
    Type          string
    Category      string
    Name          string
    AmountKopecks int64
    StartDate     time.Time
    PaymentDay    int
    EndDate       *time.Time
    Comment       *string
}

type UpdateRecurringOperationCommand struct {
    Type          *string
    Category      *string
    Name          *string
    AmountKopecks *int64
    StartDate     *time.Time
    PaymentDay    *int
    EndDate       *time.Time
    Comment       *string
}
```

**Step 2: Use Name in CreateRecurringOperation**

```go
rec := domain.RecurringOperation{
    ...
    Name:          strings.TrimSpace(cmd.Name),
    ...
}
```

Validate name is non-empty and ≤ 50 runes.

**Step 3: Use Name in UpdateRecurringOperation**

```go
if cmd.Name != nil {
    rec.Name = strings.TrimSpace(*cmd.Name)
}
```

**Step 4: Propagate name to generated operations**

In `buildOperations`, set:

```go
ops = append(ops, domain.Operation{
    ...
    Name:    rec.Name,
    Comment: rec.Comment,
    ...
})
```

**Step 5: Build and test**

```bash
cd apps/backend
go build ./...
go test ./internal/leases/application/...
```

**Step 6: Commit**

```bash
git add apps/backend/internal/leases/application/recurring_operation_service.go
git commit -m "feat: handle name in recurring operation service"
```

---

### Task 11: Update OpenAPI recurring operation schemas

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Add name to RecurringOperationCreateRequest, RecurringOperationUpdateRequest, RecurringOperationResponse**

```yaml
    RecurringOperationCreateRequest:
      type: object
      required: [type, category, name, amount_kopecks, start_date, payment_day]
      properties:
        type:
          $ref: '#/components/schemas/OperationType'
        category:
          $ref: '#/components/schemas/OperationCategory'
        name:
          type: string
          maxLength: 50
        ...
```

**Step 2: Add `yearly` to periodicity enum**

```yaml
        periodicity:
          type: string
          enum: [monthly, yearly]
```

**Step 3: Regenerate Go server interface**

```bash
cd apps/backend
make generate-openapi
```

**Step 4: Build**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add apps/backend/api/openapi/openapi.yaml apps/backend/internal/platform/openapi/generated.gen.go
git commit -m "feat: add name and yearly to recurring operation openapi schemas"
```

---

### Task 12: Update recurring operation handlers mapping

**Files:**
- Modify: `apps/backend/internal/platform/httpapi/recurring_operation_handlers.go`

**Step 1: Map name and periodicity in CreateRecurringOperation**

```go
cmd := leasesapp.CreateRecurringOperationCommand{
    PropertyID:    propertyId,
    Type:          string(body.Type),
    Category:      string(body.Category),
    Name:          body.Name,
    AmountKopecks: int64(body.AmountKopecks),
    StartDate:     body.StartDate.Time,
    PaymentDay:    int(body.PaymentDay),
    EndDate:       datePtrFromOpenAPI(body.EndDate),
    Comment:       body.Comment,
}
```

**Step 2: Map name and periodicity in UpdateRecurringOperation**

```go
cmd := leasesapp.UpdateRecurringOperationCommand{
    Type:     ptrString(body.Type),
    Category: ptrString(body.Category),
    Name:     body.Name,
    ...
}
```

**Step 3: Map name and periodicity in recurringOperationResponse**

```go
resp := openapi.RecurringOperationResponse{
    ...
    Name:          rec.Name,
    Periodicity:   openapi.RecurringOperationPeriodicity(rec.Periodicity),
    ...
}
```

**Step 4: Build and test**

```bash
cd apps/backend
go build ./...
go test ./internal/platform/httpapi/...
```

**Step 5: Commit**

```bash
git add apps/backend/internal/platform/httpapi/recurring_operation_handlers.go
git commit -m "feat: map name and yearly periodicity in recurring operation handlers"
```

---

### Task 13: Add yearly periodicity to domain

**Files:**
- Modify: `apps/backend/internal/leases/domain/operation.go`
- Modify: `apps/backend/internal/leases/domain/rent_schedule.go`

**Step 1: Add Yearly periodicity constant**

```go
const (
    RecurringOperationPeriodicityMonthly RecurringOperationPeriodicity = "monthly"
    RecurringOperationPeriodicityYearly  RecurringOperationPeriodicity = "yearly"
)
```

**Step 2: Update Parse/Validate logic if any**

Search for references to `RecurringOperationPeriodicityMonthly` and add `Yearly` handling.

**Step 3: Extend GenerateDates for yearly**

Change signature to accept periodicity:

```go
func GenerateDates(start time.Time, paymentDay int, endDate *time.Time, now time.Time, periodicity RecurringOperationPeriodicity) []time.Time
```

Add `nextPaymentDateYearly` helper and switch on periodicity in the loop.

**Step 4: Update all callers of GenerateDates**

Search for `GenerateDates(` and pass `rec.Periodicity`.

**Step 5: Build and test**

```bash
cd apps/backend
go build ./...
go test ./internal/leases/...
```

**Step 6: Commit**

```bash
git add apps/backend/internal/leases/domain/operation.go apps/backend/internal/leases/domain/rent_schedule.go
git commit -m "feat: support yearly recurring operation periodicity"
```

---

### Task 14: Backend full verification

**Step 1: Run linter**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
make backend-lint
```

Expected: 0 issues.

**Step 2: Run tests**

```bash
cd apps/backend && go test ./...
```

Expected: all pass.

**Step 3: Commit any test/fake fixes**

```bash
git add -A
git commit -m "test: update operation fakes and tests for name field"
```

---

## Frontend: operation creation page

### Task 15: Create operation entity types

**Files:**
- Create: `apps/frontend/entities/operation/model/types.ts`

**Step 1: Write types**

```ts
export type OperationType = 'income' | 'expense';

export type OperationCategory =
  | 'rent'
  | 'other_income'
  | 'utilities'
  | 'repair'
  | 'tax'
  | 'other_expense';

export type OperationFrequency =
  | 'once'
  | 'monthly'
  | 'yearly';

export const incomeCategories: { value: OperationCategory; label: string }[] = [
  { value: 'rent', label: 'Арендная плата' },
  { value: 'other_income', label: 'Прочий доход' },
];

export const expenseCategories: { value: OperationCategory; label: string }[] = [
  { value: 'utilities', label: 'ЖКХ' },
  { value: 'repair', label: 'Ремонт' },
  { value: 'tax', label: 'Налог' },
  { value: 'other_expense', label: 'Прочее' },
];
```

**Step 2: Commit**

```bash
git add apps/frontend/entities/operation/model/types.ts
git commit -m "feat: add operation entity types"
```

---

### Task 16: Add operation API hooks

**Files:**
- Create: `apps/frontend/features/operations/api/hooks.ts`
- Create: `apps/frontend/features/operations/api/index.ts`

**Step 1: Implement hooks**

```ts
export function useCreateOperation() {
  // wraps generated POST /properties/{propertyId}/operations
}

export function useCreateRecurringOperation() {
  // wraps generated POST /properties/{propertyId}/recurring-operations
}

export function useCreateOperationReminder() {
  // wraps generated POST /properties/{propertyId}/operations/{operationId}/reminders
}

export function useCreateRecurringOperationReminder() {
  // wraps generated POST /properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders
}
```

Use existing patterns from `features/properties/api/hooks.ts`.

**Step 2: Regenerate frontend API types**

```bash
cd apps/frontend
npm run generate-api  # or your project command
```

**Step 3: Build TypeScript**

```bash
npx tsc --noEmit
```

**Step 4: Commit**

```bash
git add apps/frontend/features/operations apps/frontend/shared/api/generated.ts
git commit -m "feat: add operation creation tanstack query hooks"
```

---

### Task 17: Create reusable select widgets

**Files:**
- Create: `apps/frontend/widgets/operations/ui/CategorySelect.tsx` + `.module.css`
- Create: `apps/frontend/widgets/operations/ui/PropertySelect.tsx` + `.module.css`
- Create: `apps/frontend/widgets/operations/ui/FrequencySelect.tsx` + `.module.css`

**Step 1: CategorySelect**

Accept `type: OperationType`, render Popover + Listbox with categories filtered by type.

**Step 2: PropertySelect**

Use `useProperties()` and render Popover + Listbox. Handle loading and empty state.

**Step 3: FrequencySelect**

Render 3 buttons/options: «Напомнить один раз», «Каждый месяц», «Каждый год».

**Step 4: Commit**

```bash
git add apps/frontend/widgets/operations/ui/CategorySelect.* apps/frontend/widgets/operations/ui/PropertySelect.* apps/frontend/widgets/operations/ui/FrequencySelect.*
git commit -m "feat: add operation select widgets"
```

---

### Task 18: Create wizard step widgets

**Files:**
- Create: `apps/frontend/widgets/operations/ui/OperationBasicInfoStep.tsx` + `.module.css`
- Create: `apps/frontend/widgets/operations/ui/OperationScheduleStep.tsx` + `.module.css`
- Create: `apps/frontend/widgets/operations/ui/OperationReminderStep.tsx` + `.module.css`
- Create: `apps/frontend/widgets/operations/ui/OperationSuccessScreen.tsx` + `.module.css`

**Step 1: BasicInfoStep**

Fields: amount, name, category, property, comment. Validates inline.

**Step 2: ScheduleStep**

Fields: frequency, operation/start date, payment day (for recurring), end date (optional).

**Step 3: ReminderStep**

Checkbox «Добавить SMS-напоминание», offset selector (1/3/7 days).

**Step 4: SuccessScreen**

Title «Платёж создан», buttons to `/finance` and property detail.

**Step 5: Commit**

```bash
git add apps/frontend/widgets/operations/ui/OperationBasicInfoStep.* apps/frontend/widgets/operations/ui/OperationScheduleStep.* apps/frontend/widgets/operations/ui/OperationReminderStep.* apps/frontend/widgets/operations/ui/OperationSuccessScreen.*
git commit -m "feat: add operation creation wizard step widgets"
```

---

### Task 19: Create OperationCreateWizard

**Files:**
- Create: `apps/frontend/widgets/operations/ui/OperationCreateWizard.tsx` + `.module.css`
- Modify: `apps/frontend/widgets/operations/index.ts`

**Step 1: Implement wizard state machine**

```ts
type Step = 'basic' | 'schedule' | 'reminder' | 'success';
```

Store form data across steps.

**Step 2: Submit logic**

```ts
async function handleSubmit() {
  if (frequency === 'once') {
    const op = await createOperation.mutateAsync({ ... });
    if (reminderEnabled) {
      await createOperationReminder.mutateAsync({ propertyId, operationId: op.id, offsetDays });
    }
  } else {
    const rec = await createRecurringOperation.mutateAsync({ ... });
    if (reminderEnabled) {
      await createRecurringOperationReminder.mutateAsync({ ... });
    }
  }
  setStep('success');
}
```

**Step 3: Export wizard from index**

```ts
export { OperationCreateWizard } from './ui/OperationCreateWizard';
```

**Step 4: Commit**

```bash
git add apps/frontend/widgets/operations/ui/OperationCreateWizard.* apps/frontend/widgets/operations/index.ts
git commit -m "feat: add OperationCreateWizard"
```

---

### Task 20: Create page

**Files:**
- Create: `apps/frontend/app/(cabinet)/finance/create-operation/page.tsx`
- Create: `apps/frontend/app/(cabinet)/finance/create-operation/page.module.css`

**Step 1: Implement page**

```tsx
export const metadata = {
  title: 'Создание платежа — Arenda Platform',
  description: 'Создание операции дохода или расхода',
};

export default async function CreateOperationPage({
  searchParams,
}: {
  searchParams: Promise<{ type?: string }>;
}) {
  const { type } = await searchParams;
  const operationType = type === 'income' || type === 'expense' ? type : 'expense';

  return (
    <div className={styles.root}>
      <OperationCreateWizard type={operationType} />
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add apps/frontend/app/(cabinet)/finance/create-operation
git commit -m "feat: add create operation page"
```

---

### Task 21: Frontend verification

**Step 1: Lint**

```bash
cd apps/frontend && npm run lint
```

Expected: 0 errors.

**Step 2: Type check**

```bash
npx tsc --noEmit
```

Expected: no errors.

**Step 3: Build**

```bash
npm run build
```

Expected: success.

**Step 4: Commit fixes**

```bash
git add -A
git commit -m "fix: frontend lint and type issues"
```

---

## Final integration

### Task 22: End-to-end verification

**Step 1: Start backend**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
make local-infra-up
make backend-run
```

**Step 2: Start frontend**

```bash
cd apps/frontend && npm run dev
```

**Step 3: Manual smoke test**

- Open `/finance/create-operation?type=expense`.
- Fill step 1, proceed to step 2.
- Select «Напомнить один раз», pick date.
- Skip reminder, submit.
- Verify operation created via backend or network tab.

**Step 4: Update CHANGELOG.md**

Add product-friendly entries for:
- new operation creation page;
- name field on operations;
- yearly recurring operation support.

**Step 5: Final commit**

```bash
git add CHANGELOG.md
git commit -m "docs: update changelog for operation creation"
```

---

## Notes

- Tests: the project convention for this task is not explicitly stated; follow existing patterns and add/update tests where backend interfaces change.
- If `yearly` turns out to require too much domain change, limit the UI to «once» and «monthly» and remove yearly tasks after confirming with the product owner.

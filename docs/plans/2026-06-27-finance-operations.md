# Finance/Operations Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the finance/operations cabinet section (dashboard, operations/payments lists, operation detail/edit, recurring operations tab, P&L report) and the required backend endpoints.

**Architecture:** Extend the Go leases domain with global list/report endpoints and a global recurring-operations list; build Next.js cabinet pages using the existing FSD structure, HeroUI v3, React Query, and the existing operation wizard.

**Tech Stack:** Go 1.26, PostgreSQL, sqlc, oapi-codegen, Next.js 16, TypeScript, HeroUI v3, React Query, CSS Modules.

**Note:** The user explicitly asked not to write new tests. Each task is verified with existing test runs, lint, and build/type-check instead.

---

## Task 1: Add global operations list query + repository port

**Files:**
- Modify: `apps/backend/db/queries/operations.sql`
- Modify: `apps/backend/internal/leases/application/ports.go`
- Modify: `apps/backend/internal/leases/adapters/postgres/repository.go`

**Step 1: Add SQL query**

Append to `apps/backend/db/queries/operations.sql`:

```sql
-- name: ListOperationsByOwner :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
WHERE owner_id = $1
  AND deleted_at IS NULL
  AND (sqlc.arg('types')::text[] = '{}'::text[] OR type = ANY(sqlc.arg('types')::text[]))
  AND (sqlc.arg('statuses')::text[] = '{}'::text[] OR status = ANY(sqlc.arg('statuses')::text[]))
  AND (sqlc.arg('categories')::text[] = '{}'::text[] OR category = ANY(sqlc.arg('categories')::text[]))
  AND (sqlc.arg('property_id')::uuid IS NULL OR property_id = sqlc.arg('property_id')::uuid)
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
  AND (sqlc.arg('recurring_operation_id')::uuid IS NULL OR recurring_operation_id = sqlc.arg('recurring_operation_id')::uuid)
ORDER BY operation_date DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;
```

**Step 2: Add port method**

In `apps/backend/internal/leases/application/ports.go`, add to `OperationRepository`:

```go
ListByOwner(ctx context.Context, ownerID uuid.UUID, filter OperationFilter) ([]domain.Operation, error)
```

Add the filter type above:

```go
type OperationFilter struct {
	Types                []domain.OperationType
	Statuses             []domain.OperationStatus
	Categories           []domain.OperationCategory
	PropertyID           uuid.UUID
	FromDate             *time.Time
	ToDate               *time.Time
	RecurringOperationID uuid.UUID
	Limit                int
	Offset               int
}
```

**Step 3: Implement repository method**

In `apps/backend/internal/leases/adapters/postgres/repository.go`, add to `OperationRepository`:

```go
func (r *OperationRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID, filter application.OperationFilter) ([]domain.Operation, error) {
	types := make([]string, 0, len(filter.Types))
	for _, t := range filter.Types {
		types = append(types, string(t))
	}
	statuses := make([]string, 0, len(filter.Statuses))
	for _, s := range filter.Statuses {
		statuses = append(statuses, string(s))
	}
	categories := make([]string, 0, len(filter.Categories))
	for _, c := range filter.Categories {
		categories = append(categories, string(c))
	}

	var fromDate, toDate pgtype.Date
	if filter.FromDate != nil {
		fromDate = pgconv.DateToPgtype(*filter.FromDate)
	}
	if filter.ToDate != nil {
		toDate = pgconv.DateToPgtype(*filter.ToDate)
	}

	limit := int32(filter.Limit)
	if limit == 0 {
		limit = 100
	}

	rows, err := r.q().ListOperationsByOwner(ctx, postgres.ListOperationsByOwnerParams{
		OwnerID:              pgconv.UUIDToPgtype(ownerID),
		Types:                types,
		Statuses:             statuses,
		Categories:           categories,
		PropertyID:           pgconv.UUIDToPgtype(filter.PropertyID),
		FromDate:             fromDate,
		ToDate:               toDate,
		RecurringOperationID: pgconv.UUIDToPgtype(filter.RecurringOperationID),
		Limit:                limit,
		Offset:               int32(filter.Offset),
	})
	if err != nil {
		return nil, err
	}

	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}
```

**Step 4: Regenerate sqlc**

Run:

```bash
cd apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Expected: generated file `internal/platform/generated/postgres/operations.sql.go` contains `ListOperationsByOwner`.

**Step 5: Run backend checks**

```bash
cd apps/backend && go build ./...
```

Expected: no compile errors.

---

## Task 2: Add `GET /operations` service + handler + OpenAPI

**Files:**
- Modify: `apps/backend/internal/leases/application/operation_service.go`
- Modify: `apps/backend/internal/platform/httpapi/operation_handlers.go`
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Add service method**

In `apps/backend/internal/leases/application/operation_service.go`, add:

```go
// ListOperations returns all operations for the owner filtered by the provided criteria.
func (s *OperationService) ListOperations(ctx context.Context, ownerID uuid.UUID, filter OperationFilter) ([]domain.Operation, error) {
	return s.operations.ListByOwner(ctx, ownerID, filter)
}
```

**Step 2: Add handler method**

In `apps/backend/internal/platform/httpapi/operation_handlers.go`, add:

```go
// ListOperations implements GET /operations.
func (h *OperationHandlers) ListOperations(w http.ResponseWriter, r *http.Request, params openapi.ListOperationsParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	filter := leasesapp.OperationFilter{}
	if params.Type != nil {
		for _, t := range *params.Type {
			filter.Types = append(filter.Types, domain.OperationType(t))
		}
	}
	if params.Status != nil {
		for _, s := range *params.Status {
			filter.Statuses = append(filter.Statuses, domain.OperationStatus(s))
		}
	}
	if params.Category != nil {
		for _, c := range *params.Category {
			filter.Categories = append(filter.Categories, domain.OperationCategory(c))
		}
	}
	if params.PropertyId != nil {
		filter.PropertyID = *params.PropertyId
	}
	if params.From != nil {
		filter.FromDate = &params.From.Time
	}
	if params.To != nil {
		filter.ToDate = &params.To.Time
	}
	if params.RecurringOperationId != nil {
		filter.RecurringOperationID = *params.RecurringOperationId
	}
	if params.Limit != nil {
		filter.Limit = *params.Limit
	}
	if params.Offset != nil {
		filter.Offset = *params.Offset
	}

	ops, err := h.svc.ListOperations(r.Context(), ownerID, filter)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	items := make([]openapi.OperationResponse, 0, len(ops))
	for _, op := range ops {
		items = append(items, operationResponse(op))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.OperationsResponse{Items: items})
}
```

**Step 3: Add OpenAPI path**

In `apps/backend/api/openapi/openapi.yaml`, before `/operations/{id}` add:

```yaml
  /operations:
    get:
      operationId: listOperations
      security:
        - sessionCookie: []
      parameters:
        - name: type
          in: query
          required: false
          style: form
          explode: true
          schema:
            type: array
            items:
              $ref: '#/components/schemas/OperationType'
        - name: status
          in: query
          required: false
          style: form
          explode: true
          schema:
            type: array
            items:
              $ref: '#/components/schemas/OperationStatus'
        - name: category
          in: query
          required: false
          style: form
          explode: true
          schema:
            type: array
            items:
              $ref: '#/components/schemas/OperationCategory'
        - name: property_id
          in: query
          required: false
          schema:
            type: string
            format: uuid
        - name: from
          in: query
          required: false
          schema:
            type: string
            format: date
        - name: to
          in: query
          required: false
          schema:
            type: string
            format: date
        - name: recurring_operation_id
          in: query
          required: false
          schema:
            type: string
            format: uuid
        - name: limit
          in: query
          required: false
          schema:
            type: integer
            default: 100
        - name: offset
          in: query
          required: false
          schema:
            type: integer
            default: 0
      responses:
        '200':
          description: Operations list
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OperationsResponse'
        '401':
          $ref: '#/components/responses/Unauthorized'
```

**Step 4: Regenerate**

Run:

```bash
cd apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

Expected: generated server interface includes `ListOperations`.

**Step 5: Verify build**

```bash
cd apps/backend && go build ./...
```

Expected: compiles.

---

## Task 3: Extend `GET /properties/:id/operations` with date/category/type filters

**Files:**
- Modify: `apps/backend/db/queries/operations.sql`
- Modify: `apps/backend/internal/leases/application/ports.go`
- Modify: `apps/backend/internal/leases/application/operation_service.go`
- Modify: `apps/backend/internal/leases/adapters/postgres/repository.go`
- Modify: `apps/backend/internal/platform/httpapi/operation_handlers.go`
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Replace per-property queries**

Replace `ListOperationsByProperty` and `ListOperationsByPropertyWithStatuses` in `apps/backend/db/queries/operations.sql` with one filtered query:

```sql
-- name: ListOperationsByProperty :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
WHERE owner_id = $1 AND property_id = $2
  AND deleted_at IS NULL
  AND (sqlc.arg('types')::text[] = '{}'::text[] OR type = ANY(sqlc.arg('types')::text[]))
  AND (sqlc.arg('statuses')::text[] = '{}'::text[] OR status = ANY(sqlc.arg('statuses')::text[]))
  AND (sqlc.arg('categories')::text[] = '{}'::text[] OR category = ANY(sqlc.arg('categories')::text[]))
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
ORDER BY operation_date DESC;
```

Delete the old `ListOperationsByPropertyWithStatuses` block.

**Step 2: Update port**

Change `OperationRepository` methods to:

```go
ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID, filter OperationFilter) ([]domain.Operation, error)
```

Remove `ListByPropertyWithStatuses`.

**Step 3: Update repository implementation**

Replace the existing `ListByProperty` and `ListByPropertyWithStatuses` methods with:

```go
func (r *OperationRepository) ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID, filter application.OperationFilter) ([]domain.Operation, error) {
	types := make([]string, 0, len(filter.Types))
	for _, t := range filter.Types {
		types = append(types, string(t))
	}
	statuses := make([]string, 0, len(filter.Statuses))
	for _, s := range filter.Statuses {
		statuses = append(statuses, string(s))
	}
	categories := make([]string, 0, len(filter.Categories))
	for _, c := range filter.Categories {
		categories = append(categories, string(c))
	}

	var fromDate, toDate pgtype.Date
	if filter.FromDate != nil {
		fromDate = pgconv.DateToPgtype(*filter.FromDate)
	}
	if filter.ToDate != nil {
		toDate = pgconv.DateToPgtype(*filter.ToDate)
	}

	rows, err := r.q().ListOperationsByProperty(ctx, postgres.ListOperationsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Types:      types,
		Statuses:   statuses,
		Categories: categories,
		FromDate:   fromDate,
		ToDate:     toDate,
	})
	if err != nil {
		return nil, err
	}

	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}
```

**Step 4: Update service**

Change `ListOperationsByProperty` signature to accept the filter:

```go
func (s *OperationService) ListOperationsByProperty(ctx context.Context, ownerID, propertyID uuid.UUID, filter OperationFilter) ([]domain.Operation, error) {
	if err := validateProperty(ctx, s.properties, ownerID, propertyID); err != nil {
		return nil, err
	}
	ops, err := s.operations.ListByProperty(ctx, ownerID, propertyID, filter)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	return ops, nil
}
```

**Step 5: Update handler**

In `ListOperationsByProperty`, build the filter from `params.Type`, `params.Status`, `params.Category`, `params.From`, `params.To` and call the new service method.

**Step 6: Update OpenAPI**

Add `type`, `category`, `from`, `to` query parameters to `GET /properties/{propertyId}/operations`. Keep `status`.

**Step 7: Regenerate and build**

Run the sqlc/oapi-codegen commands and `go build ./...`.

Expected: compiles.

---

## Task 4: Add P&L report endpoint

**Files:**
- Create: `apps/backend/db/queries/finance.sql`
- Modify: `apps/backend/internal/leases/application/ports.go`
- Modify: `apps/backend/internal/leases/adapters/postgres/repository.go`
- Modify: `apps/backend/internal/leases/application/operation_service.go`
- Create or modify: `apps/backend/internal/platform/httpapi/finance_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/server.go`
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Add finance SQL queries**

Create `apps/backend/db/queries/finance.sql`:

```sql
-- name: GetFinanceReportTotals :one
SELECT
  COALESCE(SUM(CASE WHEN type = 'income' THEN amount_kopecks ELSE 0 END), 0)::bigint AS income_kopecks,
  COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_kopecks ELSE 0 END), 0)::bigint AS expense_kopecks
FROM operations
WHERE owner_id = $1
  AND operation_date >= $2 AND operation_date <= $3
  AND deleted_at IS NULL;

-- name: GetFinanceReportByProperty :many
SELECT
  property_id,
  COALESCE(SUM(CASE WHEN type = 'income' THEN amount_kopecks ELSE 0 END), 0)::bigint AS income_kopecks,
  COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_kopecks ELSE 0 END), 0)::bigint AS expense_kopecks
FROM operations
WHERE owner_id = $1
  AND operation_date >= $2 AND operation_date <= $3
  AND deleted_at IS NULL
GROUP BY property_id
ORDER BY income_kopecks - expense_kopecks DESC;

-- name: GetFinanceReportByCategory :many
SELECT
  type,
  category,
  COALESCE(SUM(amount_kopecks), 0)::bigint AS total_kopecks
FROM operations
WHERE owner_id = $1
  AND operation_date >= $2 AND operation_date <= $3
  AND deleted_at IS NULL
GROUP BY type, category
ORDER BY total_kopecks DESC;

-- name: GetFinanceReportByMonth :many
SELECT
  date_trunc('month', operation_date)::date AS month,
  COALESCE(SUM(CASE WHEN type = 'income' THEN amount_kopecks ELSE 0 END), 0)::bigint AS income_kopecks,
  COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_kopecks ELSE 0 END), 0)::bigint AS expense_kopecks
FROM operations
WHERE owner_id = $1
  AND operation_date >= $2 AND operation_date <= $3
  AND deleted_at IS NULL
GROUP BY month
ORDER BY month;
```

**Step 2: Add port and result types**

In `apps/backend/internal/leases/application/ports.go`, add to `OperationRepository`:

```go
GetFinanceReportTotals(ctx context.Context, ownerID uuid.UUID, from, to time.Time) (FinanceReportTotals, error)
GetFinanceReportByProperty(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]FinanceReportPropertyRow, error)
GetFinanceReportByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]FinanceReportCategoryRow, error)
GetFinanceReportByMonth(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]FinanceReportMonthRow, error)
```

Add result types:

```go
type FinanceReportTotals struct {
	IncomeKopecks  int64
	ExpenseKopecks int64
}

type FinanceReportPropertyRow struct {
	PropertyID     uuid.UUID
	IncomeKopecks  int64
	ExpenseKopecks int64
}

type FinanceReportCategoryRow struct {
	Type           domain.OperationType
	Category       domain.OperationCategory
	TotalKopecks   int64
}

type FinanceReportMonthRow struct {
	Month          time.Time
	IncomeKopecks  int64
	ExpenseKopecks int64
}
```

**Step 3: Implement repository methods**

In `apps/backend/internal/leases/adapters/postgres/repository.go`, implement the four methods mapping sqlc rows to the application types.

**Step 4: Add service method**

In `apps/backend/internal/leases/application/operation_service.go`:

```go
func (s *OperationService) GetFinanceReport(ctx context.Context, ownerID uuid.UUID, from, to time.Time) (FinanceReport, error) {
	totals, err := s.operations.GetFinanceReportTotals(ctx, ownerID, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report totals: %w", err)
	}
	byProperty, err := s.operations.GetFinanceReportByProperty(ctx, ownerID, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report by property: %w", err)
	}
	byCategory, err := s.operations.GetFinanceReportByCategory(ctx, ownerID, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report by category: %w", err)
	}
	byMonth, err := s.operations.GetFinanceReportByMonth(ctx, ownerID, from, to)
	if err != nil {
		return FinanceReport{}, fmt.Errorf("report by month: %w", err)
	}
	return FinanceReport{
		Totals:     totals,
		ByProperty: byProperty,
		ByCategory: byCategory,
		ByMonth:    byMonth,
	}, nil
}
```

Add `FinanceReport` struct in the same file.

**Step 5: Add handler**

Create `apps/backend/internal/platform/httpapi/finance_handlers.go`:

```go
package httpapi

import (
	"net/http"

	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type FinanceHandlers struct {
	svc *leasesapp.OperationService
}

func NewFinanceHandlers(svc *leasesapp.OperationService) *FinanceHandlers {
	return &FinanceHandlers{svc: svc}
}

func (h *FinanceHandlers) GetFinanceReport(w http.ResponseWriter, r *http.Request, params openapi.GetFinanceReportParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	report, err := h.svc.GetFinanceReport(r.Context(), ownerID, params.From.Time, params.To.Time)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	resp := openapi.FinanceReportResponse{
		Period: openapi.FinanceReportPeriod{
			From: openapi_types.Date{Time: params.From.Time},
			To:   openapi_types.Date{Time: params.To.Time},
		},
		Totals: openapi.FinanceReportTotals{
			IncomeKopecks:  int(report.Totals.IncomeKopecks),
			ExpenseKopecks: int(report.Totals.ExpenseKopecks),
			ProfitKopecks:  int(report.Totals.IncomeKopecks - report.Totals.ExpenseKopecks),
		},
	}
	// map by_property, by_category, by_month slices...
	writeJSON(r.Context(), w, http.StatusOK, resp)
}
```

**Step 6: Wire handler**

In `apps/backend/internal/platform/httpapi/server.go`:

- Add `Finance *leasesapp.FinanceService` or reuse `Operations` in `Deps` if needed. Actually `FinanceHandlers` only needs `OperationService`, which is already in `Deps.Operations`.
- Instantiate `financeHandlers := NewFinanceHandlers(deps.Operations)`.
- Add `*FinanceHandlers` to `composedHandler`.

**Step 7: Add OpenAPI path and schemas**

Add `GET /finance/report` path and `FinanceReportResponse`, `FinanceReportPeriod`, `FinanceReportTotals`, `FinanceReportPropertyRow`, `FinanceReportCategoryRow`, `FinanceReportMonthRow` schemas.

**Step 8: Regenerate and build**

Run sqlc/oapi-codegen and `go build ./...`.

Expected: compiles.

---

## Task 5: Add global recurring-operations list

**Files:**
- Modify: `apps/backend/db/queries/recurring_operations.sql`
- Modify: `apps/backend/internal/leases/application/ports.go`
- Modify: `apps/backend/internal/leases/adapters/postgres/repository.go`
- Modify: `apps/backend/internal/leases/application/recurring_operation_service.go`
- Modify: `apps/backend/internal/platform/httpapi/recurring_operation_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/server.go`
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Add SQL query**

Append to `apps/backend/db/queries/recurring_operations.sql`:

```sql
-- name: ListRecurringOperationsByOwner :many
SELECT * FROM recurring_operations
WHERE owner_id = $1
ORDER BY created_at DESC;
```

**Step 2: Add port and repository method**

Add `ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.RecurringOperation, error)` to `RecurringOperationRepository` and implement it in the postgres adapter.

**Step 3: Add service method**

```go
func (s *RecurringOperationService) ListRecurringOperations(ctx context.Context, ownerID uuid.UUID) ([]domain.RecurringOperation, error) {
	recs, err := s.recurringOps.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list recurring operations: %w", err)
	}
	return recs, nil
}
```

**Step 4: Add handler**

In `apps/backend/internal/platform/httpapi/recurring_operation_handlers.go`:

```go
func (h *RecurringOperationHandlers) ListRecurringOperations(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	recs, err := h.svc.ListRecurringOperations(r.Context(), ownerID)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	items := make([]openapi.RecurringOperationResponse, 0, len(recs))
	for _, rec := range recs {
		items = append(items, recurringOperationResponse(rec))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.RecurringOperationsResponse{Items: items})
}
```

**Step 5: Add OpenAPI path**

Add `GET /recurring-operations` before `/recurring-operations/{id}`.

**Step 6: Regenerate and build**

Run sqlc/oapi-codegen and `go build ./...`.

Expected: compiles.

---

## Task 6: Add finance route constants and regenerate frontend API types

**Files:**
- Modify: `apps/frontend/shared/config/routes.ts`
- Run: `apps/frontend/package.json` script

**Step 1: Add routes**

```ts
export const ROUTES = {
  // ... existing routes
  finance: '/finance',
  financeOperations: '/finance/operations',
  financePayments: '/finance/payments',
  financeCreateOperation: '/finance/create-operation',
  financeOperation: (id: string) => `/finance/operations/${id}`,
  financeOperationEdit: (id: string) => `/finance/operations/${id}/edit`,
  propertyOperations: (id: string) => `/properties/${id}/operations`,
} as const;
```

**Step 2: Regenerate API types**

```bash
cd apps/frontend
npm run generate:api
```

Expected: `apps/frontend/shared/api/generated.ts` updated with new endpoints.

**Step 3: Type-check**

```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no type errors from generated types.

---

## Task 7: Add operation domain libs

**Files:**
- Create: `apps/frontend/entities/operation/lib/statuses.ts`
- Create: `apps/frontend/entities/operation/lib/categories.ts`
- Create: `apps/frontend/entities/operation/lib/formatMoney.ts`
- Create: `apps/frontend/entities/operation/lib/dates.ts`

**Step 1: statuses.ts**

```ts
import type { components } from '@/shared/api/generated';

type OperationStatus = components['schemas']['OperationStatus'];

export const operationStatusOptions: { value: OperationStatus; label: string; variant: 'warning' | 'danger' | 'success' | 'default' }[] = [
  { value: 'pending', label: 'Запланирована', variant: 'warning' },
  { value: 'overdue', label: 'Просрочена', variant: 'danger' },
  { value: 'paid', label: 'Оплачена', variant: 'success' },
  { value: 'received', label: 'Получена', variant: 'success' },
];

export function getOperationStatusLabel(status: OperationStatus): string {
  return operationStatusOptions.find((o) => o.value === status)?.label ?? status;
}
```

**Step 2: categories.ts**

```ts
import type { components } from '@/shared/api/generated';

type OperationCategory = components['schemas']['OperationCategory'];
type OperationType = components['schemas']['OperationType'];

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

export function getCategoriesByType(type: OperationType) {
  return type === 'income' ? incomeCategories : expenseCategories;
}

export function getCategoryLabel(category: OperationCategory): string {
  const all = [...incomeCategories, ...expenseCategories];
  return all.find((c) => c.value === category)?.label ?? category;
}
```

**Step 3: formatMoney.ts**

```ts
export function formatMoneyKopecks(kopecks: number): string {
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    maximumFractionDigits: 0,
  }).format(kopecks / 100);
}
```

**Step 4: dates.ts**

```ts
export function formatOperationDate(dateString: string): string {
  return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }).format(new Date(dateString));
}

export function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

export function endOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth() + 1, 0);
}
```

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

---

## Task 8: Add finance API hooks

**Files:**
- Modify: `apps/frontend/features/operations/api/hooks.ts`
- Modify: `apps/frontend/features/operations/api/keys.ts`
- Create: `apps/frontend/features/finance/api/hooks.ts`
- Create: `apps/frontend/features/finance/api/keys.ts`
- Modify: `apps/frontend/features/recurring-operations/api/hooks.ts`
- Modify: `apps/frontend/features/recurring-operations/api/keys.ts`

**Step 1: operation hooks**

Add to `apps/frontend/features/operations/api/keys.ts`:

```ts
operations: (filters: Record<string, string | undefined>) =>
  ['operations', filters] as const,
```

Add to `apps/frontend/features/operations/api/hooks.ts`:

```ts
export type OperationsFilters = {
  type?: 'income' | 'expense';
  status?: string;
  category?: string;
  property_id?: string;
  from?: string;
  to?: string;
  recurring_operation_id?: string;
};

export function useOperations(filters: OperationsFilters = {}) {
  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    Object.entries(filters).forEach(([key, value]) => {
      if (value) params.set(key, value);
    });
    return params.toString();
  }, [filters]);

  return useQuery({
    queryKey: operationKeys.operations({ ...filters }),
    queryFn: () => apiClient<OperationsResponse>(`/operations${queryString ? `?${queryString}` : ''}`),
    enabled: true,
  });
}

export function useCompleteOperation() {
  const queryClient = useQueryClient();
  return useMutation<OperationResponse, ApiError, { id: string; propertyId?: string }>({
    mutationFn: ({ id }) =>
      apiClient<OperationResponse>(`/operations/${id}/complete`, { method: 'POST' }),
    onSuccess: (_, { id, propertyId }) => {
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      queryClient.invalidateQueries({ queryKey: ['operations'] });
      if (propertyId) {
        queryClient.invalidateQueries({ queryKey: operationKeys.byProperty(propertyId) });
        queryClient.invalidateQueries({ queryKey: operationKeys.summary(propertyId) });
      }
    },
  });
}
```

**Step 2: finance report hooks**

Create `apps/frontend/features/finance/api/keys.ts`:

```ts
export const financeKeys = {
  report: (from: string, to: string) => ['finance', 'report', from, to] as const,
};
```

Create `apps/frontend/features/finance/api/hooks.ts`:

```ts
'use client';

import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { financeKeys } from './keys';
import type { components } from '@/shared/api/generated';

type FinanceReportResponse = components['schemas']['FinanceReportResponse'];

export function useFinanceReport(from: string, to: string) {
  return useQuery({
    queryKey: financeKeys.report(from, to),
    queryFn: () => apiClient<FinanceReportResponse>(`/finance/report?from=${from}&to=${to}`),
    enabled: Boolean(from) && Boolean(to),
  });
}
```

**Step 3: recurring operations global hook**

Add to `apps/frontend/features/recurring-operations/api/keys.ts`:

```ts
recurringOperations: () => ['recurring-operations'] as const,
```

Add to `apps/frontend/features/recurring-operations/api/hooks.ts`:

```ts
export function useRecurringOperations() {
  return useQuery({
    queryKey: recurringOperationKeys.recurringOperations(),
    queryFn: () => apiClient<RecurringOperationsResponse>('/recurring-operations'),
  });
}
```

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

---

## Task 9: Build finance dashboard page

**Files:**
- Create: `apps/frontend/widgets/finance/ui/FinanceDashboard.tsx`
- Create: `apps/frontend/widgets/finance/ui/FinanceSummaryCards.tsx`
- Create: `apps/frontend/widgets/finance/ui/FinancePeriodSelect.tsx`
- Create: `apps/frontend/app/(cabinet)/finance/page.tsx`

**Step 1: FinancePeriodSelect.tsx**

A simple component that lets the user pick Month/Quarter/Year and updates `from`/`to` dates.

```tsx
'use client';

import { useState } from 'react';
import { Button } from '@/shared/ui/button';

type Period = 'month' | 'quarter' | 'year';

export function FinancePeriodSelect({ onChange }: { onChange: (from: string, to: string) => void }) {
  const [period, setPeriod] = useState<Period>('month');

  const handleSelect = (next: Period) => {
    setPeriod(next);
    const now = new Date();
    let from: Date;
    let to: Date;
    if (next === 'month') {
      from = new Date(now.getFullYear(), now.getMonth(), 1);
      to = new Date(now.getFullYear(), now.getMonth() + 1, 0);
    } else if (next === 'quarter') {
      const q = Math.floor(now.getMonth() / 3);
      from = new Date(now.getFullYear(), q * 3, 1);
      to = new Date(now.getFullYear(), q * 3 + 3, 0);
    } else {
      from = new Date(now.getFullYear(), 0, 1);
      to = new Date(now.getFullYear(), 11, 31);
    }
    onChange(from.toISOString().split('T')[0], to.toISOString().split('T')[0]);
  };

  return (
    <div>
      {(['month', 'quarter', 'year'] as Period[]).map((p) => (
        <Button key={p} variant={period === p ? 'primary' : 'secondary'} size="small" onClick={() => handleSelect(p)}>
          {p === 'month' ? 'Месяц' : p === 'quarter' ? 'Квартал' : 'Год'}
        </Button>
      ))}
    </div>
  );
}
```

**Step 2: FinanceSummaryCards.tsx**

```tsx
'use client';

import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import styles from './FinanceSummaryCards.module.css';

type Props = {
  incomeKopecks: number;
  expenseKopecks: number;
  pendingIncomeKopecks: number;
  pendingExpenseKopecks: number;
  overdueCount: number;
};

export function FinanceSummaryCards({ incomeKopecks, expenseKopecks, pendingIncomeKopecks, pendingExpenseKopecks, overdueCount }: Props) {
  const profit = incomeKopecks - expenseKopecks;
  return (
    <div className={styles.grid}>
      <div className={styles.card}>
        <span className={styles.label}>Доход</span>
        <span className={styles.amount}>{formatMoneyKopecks(incomeKopecks)}</span>
      </div>
      <div className={styles.card}>
        <span className={styles.label}>Расход</span>
        <span className={styles.amount}>{formatMoneyKopecks(expenseKopecks)}</span>
      </div>
      <div className={styles.card}>
        <span className={styles.label}>Прибыль</span>
        <span className={styles.amount}>{formatMoneyKopecks(profit)}</span>
      </div>
      <div className={styles.card}>
        <span className={styles.label}>Ожидается</span>
        <span className={styles.amount}>{formatMoneyKopecks(pendingIncomeKopecks - pendingExpenseKopecks)}</span>
      </div>
      <div className={styles.card}>
        <span className={styles.label}>Просрочено</span>
        <span className={styles.amount}>{overdueCount}</span>
      </div>
    </div>
  );
}
```

Create matching CSS modules with the project tokens.

**Step 3: FinanceDashboard.tsx**

```tsx
'use client';

import { useMemo, useState } from 'react';
import { useOperations } from '@/features/operations/api/hooks';
import { startOfMonth, endOfMonth } from '@/entities/operation/lib/dates';
import { FinancePeriodSelect } from './FinancePeriodSelect';
import { FinanceSummaryCards } from './FinanceSummaryCards';
import { OperationsList } from '@/widgets/operations/ui/OperationsList';
import { FinanceEmptyState } from './FinanceEmptyState';
import { FinanceErrorState } from './FinanceErrorState';
import { FinanceLoading } from './FinanceLoading';
import styles from './FinanceDashboard.module.css';

export function FinanceDashboard() {
  const now = new Date();
  const [period, setPeriod] = useState({
    from: startOfMonth(now).toISOString().split('T')[0],
    to: endOfMonth(now).toISOString().split('T')[0],
  });

  const { data, isLoading, isError, refetch } = useOperations({ from: period.from, to: period.to });

  const summary = useMemo(() => {
    let income = 0;
    let expense = 0;
    let pendingIncome = 0;
    let pendingExpense = 0;
    let overdue = 0;
    data?.items.forEach((op) => {
      if (op.status === 'paid' || op.status === 'received') {
        if (op.type === 'income') income += op.amount_kopecks;
        else expense += op.amount_kopecks;
      } else if (op.status === 'pending') {
        if (op.type === 'income') pendingIncome += op.amount_kopecks;
        else pendingExpense += op.amount_kopecks;
      } else if (op.status === 'overdue') {
        overdue += 1;
        if (op.type === 'income') pendingIncome += op.amount_kopecks;
        else pendingExpense += op.amount_kopecks;
      }
    });
    return { income, expense, pendingIncome, pendingExpense, overdue };
  }, [data]);

  if (isLoading) return <FinanceLoading />;
  if (isError) return <FinanceErrorState onRetry={refetch} />;
  const isEmpty = !data || data.items.length === 0;

  return (
    <div className={styles.root}>
      <h1 className={styles.title}>Финансы</h1>
      <FinancePeriodSelect onChange={(from, to) => setPeriod({ from, to })} />
      <FinanceSummaryCards
        incomeKopecks={summary.income}
        expenseKopecks={summary.expense}
        pendingIncomeKopecks={summary.pendingIncome}
        pendingExpenseKopecks={summary.pendingExpense}
        overdueCount={summary.overdue}
      />
      {isEmpty ? (
        <FinanceEmptyState />
      ) : (
        <OperationsList items={data.items.slice(0, 5)} showProperty />
      )}
    </div>
  );
}
```

**Step 4: page.tsx**

```tsx
import type { Metadata } from 'next';
import { Suspense } from 'react';
import { FinanceDashboard } from '@/widgets/finance/ui/FinanceDashboard';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Финансы — Arenda Platform',
};

export default function FinancePage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <FinanceDashboard />
    </Suspense>
  );
}
```

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

---

## Task 10: Build operations list page with tabs and filters

**Files:**
- Create: `apps/frontend/widgets/operations/ui/OperationsList.tsx`
- Create: `apps/frontend/widgets/operations/ui/OperationListItem.tsx`
- Create: `apps/frontend/widgets/operations/ui/OperationFilters.tsx`
- Create: `apps/frontend/app/(cabinet)/finance/operations/page.tsx`

**Step 1: OperationListItem.tsx**

```tsx
'use client';

import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import { formatOperationDate } from '@/entities/operation/lib/dates';
import { getCategoryLabel } from '@/entities/operation/lib/categories';
import { getOperationStatusLabel } from '@/entities/operation/lib/statuses';
import type { components } from '@/shared/api/generated';
import styles from './OperationListItem.module.css';

type Operation = components['schemas']['OperationResponse'];

export function OperationListItem({ operation, showProperty }: { operation: Operation; showProperty?: boolean }) {
  const sign = operation.type === 'income' ? '+' : '-';
  return (
    <NextLink href={ROUTES.financeOperation(operation.id)} className={styles.root}>
      <div className={styles.row}>
        <span className={styles.date}>{formatOperationDate(operation.operation_date)}</span>
        <span className={styles.status}>{getOperationStatusLabel(operation.status)}</span>
      </div>
      <div className={styles.row}>
        <span className={styles.name}>{operation.name}</span>
        <span className={styles.amount}>{sign}{formatMoneyKopecks(operation.amount_kopecks)}</span>
      </div>
      <div className={styles.row}>
        <span className={styles.category}>{getCategoryLabel(operation.category)}</span>
        {showProperty && <span className={styles.property}>{operation.property_id}</span>}
      </div>
    </NextLink>
  );
}
```

**Step 2: OperationsList.tsx**

```tsx
'use client';

import { OperationListItem } from './OperationListItem';
import type { components } from '@/shared/api/generated';
import styles from './OperationsList.module.css';

type Operation = components['schemas']['OperationResponse'];

export function OperationsList({ items, showProperty }: { items: Operation[]; showProperty?: boolean }) {
  return (
    <ul className={styles.list}>
      {items.map((operation) => (
        <li key={operation.id}>
          <OperationListItem operation={operation} showProperty={showProperty} />
        </li>
      ))}
    </ul>
  );
}
```

**Step 3: OperationFilters.tsx**

Reuse the pattern from `PropertiesToolbar`. Provide filter chips for period, type, status, category, property.

**Step 4: page.tsx**

```tsx
import type { Metadata } from 'next';
import { Suspense } from 'react';
import { OperationsPage } from '@/widgets/operations/ui/OperationsPage';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Операции — Arenda Platform',
};

export default function FinanceOperationsRoutePage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <OperationsPage />
    </Suspense>
  );
}
```

Create `apps/frontend/widgets/operations/ui/OperationsPage.tsx` with tab navigation (Все / Доходы / Расходы / Регулярные / Прибыль) and URL state.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 11: Build payments page

**Files:**
- Create: `apps/frontend/app/(cabinet)/finance/payments/page.tsx`
- Create: `apps/frontend/widgets/operations/ui/PaymentsPage.tsx`

**Step 1: PaymentsPage.tsx**

```tsx
'use client';

import { useOperations } from '@/features/operations/api/hooks';
import { OperationsList } from './OperationsList';
import { FinanceEmptyState } from '@/widgets/finance/ui/FinanceEmptyState';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';

export function PaymentsPage() {
  const { data, isLoading, isError, refetch } = useOperations({ status: 'pending,overdue' });

  if (isLoading) return <FinanceLoading />;
  if (isError) return <FinanceErrorState onRetry={refetch} />;
  if (!data || data.items.length === 0) return <FinanceEmptyState title="Нет платежей" subtitle="Запланированные и просроченные операции появятся здесь." />;

  return (
    <div>
      <h1>Платежи</h1>
      <OperationsList items={data.items} showProperty />
    </div>
  );
}
```

**Step 2: page.tsx**

Wrap in Suspense.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 12: Build operation detail page

**Files:**
- Create: `apps/frontend/widgets/operations/ui/OperationDetailPage.tsx`
- Create: `apps/frontend/app/(cabinet)/finance/operations/[id]/page.tsx`

**Step 1: OperationDetailPage.tsx**

```tsx
'use client';

import { useParams, useRouter } from 'next/navigation';
import { useOperation, useDeleteOperation, useCompleteOperation } from '@/features/operations/api/hooks';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import { formatOperationDate } from '@/entities/operation/lib/dates';
import { getCategoryLabel } from '@/entities/operation/lib/categories';
import { getOperationStatusLabel } from '@/entities/operation/lib/statuses';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';

export function OperationDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { data, isLoading, isError, refetch } = useOperation(id);
  const complete = useCompleteOperation();
  const remove = useDeleteOperation();

  if (isLoading) return <FinanceLoading />;
  if (isError || !data) return <FinanceErrorState onRetry={refetch} />;

  const canComplete = data.status === 'pending' || data.status === 'overdue';

  return (
    <div>
      <h1>{data.name}</h1>
      <p>{getOperationStatusLabel(data.status)}</p>
      <p>{formatMoneyKopecks(data.amount_kopecks)}</p>
      <p>{formatOperationDate(data.operation_date)}</p>
      <p>{getCategoryLabel(data.category)}</p>
      {data.comment && <p>{data.comment}</p>}

      {canComplete && (
        <Button
          onClick={() => complete.mutate({ id: data.id, propertyId: data.property_id })}
          loading={complete.isPending}
        >
          {data.type === 'income' ? 'Отметить полученной' : 'Отметить оплаченной'}
        </Button>
      )}

      <Button
        variant="secondary"
        onClick={() => router.push(ROUTES.financeOperationEdit(data.id))}
      >
        Редактировать
      </Button>

      <Button
        variant="danger"
        onClick={() => {
          if (confirm('Удалить операцию?')) {
            remove.mutate({ id: data.id, propertyId: data.property_id }, { onSuccess: () => router.push(ROUTES.financeOperations) });
          }
        }}
        loading={remove.isPending}
      >
        Удалить
      </Button>
    </div>
  );
}
```

**Step 2: page.tsx**

```tsx
import type { Metadata } from 'next';
import { Suspense } from 'react';
import { OperationDetailPage } from '@/widgets/operations/ui/OperationDetailPage';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Операция — Arenda Platform',
};

export default function FinanceOperationRoutePage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <OperationDetailPage />
    </Suspense>
  );
}
```

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 13: Build operation edit page

**Files:**
- Create: `apps/frontend/widgets/operations/ui/OperationEditForm.tsx`
- Create: `apps/frontend/app/(cabinet)/finance/operations/[id]/edit/page.tsx`

**Step 1: OperationEditForm.tsx**

A single-page form (not wizard) that pre-fills from `useOperation` and calls `useUpdateOperation`. Fields: name, type, category, amount, operation_date, comment.

**Step 2: page.tsx**

Wrap in Suspense.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 14: Build recurring operations tab

**Files:**
- Create: `apps/frontend/widgets/operations/ui/RecurringOperationsTab.tsx`

Use `useRecurringOperations`, list templates with property/category/amount/periodicity/status. Add Pause/Resume buttons using existing hooks. Link to edit if needed.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 15: Build profit report tab

**Files:**
- Create: `apps/frontend/widgets/operations/ui/ProfitReport.tsx`

Use `useFinanceReport`. Display totals, by-property table, by-category list, and a simple month-by-month bar chart using divs.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 16: Build property finance page

**Files:**
- Create: `apps/frontend/app/(cabinet)/properties/[id]/operations/page.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyOperationsPage.tsx`

Use `usePropertyOperationsSummary(id)` and `useOperationsByProperty(id)` with optional date filters. Link to add operation with preselected property.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 17: Add finance empty/loading/error states

**Files:**
- Create: `apps/frontend/widgets/finance/ui/FinanceEmptyState.tsx`
- Create: `apps/frontend/widgets/finance/ui/FinanceLoading.tsx`
- Create: `apps/frontend/widgets/finance/ui/FinanceErrorState.tsx`

Follow the existing `EmptyState`, `PropertiesLoading`, `PropertiesErrorState` patterns. Use project tokens.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 18: Handle readonly subscription mode

**Files:**
- Modify: `apps/frontend/widgets/finance/ui/FinanceDashboard.tsx`
- Modify: `apps/frontend/widgets/operations/ui/OperationsPage.tsx`
- Modify: `apps/frontend/widgets/operations/ui/OperationDetailPage.tsx`

Use the existing subscription hook/store to detect readonly mode. Hide or disable create/edit/complete/delete buttons and show a banner with a link to `/profile/tariff`.

If no existing subscription hook exists, add a minimal `useSubscriptionStatus` hook in `features/subscription/api/hooks.ts` calling `GET /subscription`.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit
```

---

## Task 19: Polish create-operation route

**Files:**
- Modify: `apps/frontend/app/(cabinet)/finance/create-operation/page.tsx`
- Modify: `apps/frontend/widgets/operations/ui/OperationCreateWizard.tsx`

- Replace hard-coded `/finance` navigation with `ROUTES.finance`.
- Add proper loading/error/empty-property states.
- Rename UI labels from "платёж" to "операция" where appropriate.

**Verification**

```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

---

## Task 20: Final verification

**Step 1: Backend checks**

```bash
make backend-lint
cd apps/backend && go test ./...
cd apps/backend && go vet ./...
```

Expected: all pass.

**Step 2: Frontend checks**

```bash
cd apps/frontend && npm run lint
cd apps/frontend && npm run build
```

Expected: lint clean, build succeeds.

**Step 3: Update CHANGELOG.md**

Add a product-friendly entry under today's date describing the new finance section.

---

## Execution handoff

Plan complete and saved to `docs/plans/2026-06-27-finance-operations.md`.

**Two execution options:**

1. **Subagent-Driven (this session)** — I dispatch a fresh subagent per task, review between tasks, and iterate quickly. Requires `superpowers:subagent-driven-development`.
2. **Parallel Session (separate)** — Open a new session with `superpowers:executing-plans` and run tasks in batches with checkpoints.

Which approach do you prefer?

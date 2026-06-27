# Страница «Просмотр объекта» — план реализации

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Реализовать страницу объекта `app/(cabinet)/properties/[id]/page.tsx` и необходимые backend endpoint’ы, чтобы отображать все состояния аренды, платежей и переходы по объектам 1:1 с макетами Figma.

**Architecture:** Модульный FSD-слой `widgets/property-detail` составляет страницу из независимых секций. Backend получает новые endpoint’ы для аренд объекта, агрегатов операций и возврата залога. Frontend переиспользует существующие entity/feature-хуки и генерирует типы из OpenAPI.

**Tech Stack:** Next.js 16 (App Router), React 19, TypeScript, TanStack Query, CSS Modules, Tailwind v4, HeroUI; Go, Chi, sqlc, pgx, oapi-codegen.

---

## Перед началом

- [ ] Убедиться, что local infra поднята: `make local-infra-up`.
- [ ] Убедиться, что `.env` заполнен и бэкенд запускается: `make backend-run`.
- [ ] Убедиться, что frontend зависимости установлены: `cd apps/frontend && npm install`.
- [ ] **Важно:** не выполнять `git commit` без явного подтверждения пользователя (политика репозитория). Шаги с коммитами в плане — подготовить индекс и сообщение, затем спросить.

---

## Task 1: Backend — SQL-запросы для новых endpoint’ов

**Files:**
- Modify: `apps/backend/db/queries/leases.sql`
- Modify: `apps/backend/db/queries/operations.sql`

**Step 1: Добавить запрос списка аренд по объекту**

В `apps/backend/db/queries/leases.sql` добавить:

```sql
-- name: ListLeasesByProperty :many
SELECT * FROM leases
WHERE property_id = $1 AND owner_id = $2
ORDER BY updated_at DESC;
```

**Step 2: Добавить агрегирующий запрос по операциям**

В `apps/backend/db/queries/operations.sql` добавить:

```sql
-- name: GetPropertyOperationsSummary :one
SELECT
    COALESCE(SUM(CASE WHEN type = 'income' AND status IN ('received') THEN amount_kopecks ELSE 0 END), 0) -
    COALESCE(SUM(CASE WHEN type = 'expense' AND status IN ('paid') THEN amount_kopecks ELSE 0 END), 0) AS all_time_profit_kopecks,
    COALESCE(SUM(CASE
        WHEN type = 'income' AND status IN ('received')
            AND operation_date >= date_trunc('month', CURRENT_DATE)
            AND operation_date < date_trunc('month', CURRENT_DATE) + interval '1 month'
        THEN amount_kopecks ELSE 0 END), 0) -
    COALESCE(SUM(CASE
        WHEN type = 'expense' AND status IN ('paid')
            AND operation_date >= date_trunc('month', CURRENT_DATE)
            AND operation_date < date_trunc('month', CURRENT_DATE) + interval '1 month'
        THEN amount_kopecks ELSE 0 END), 0) AS monthly_profit_kopecks,
    COUNT(CASE WHEN status = 'overdue' AND type = 'income' AND category = 'rent' THEN 1 END) AS overdue_rent_count,
    COUNT(CASE WHEN status = 'overdue' THEN 1 END) AS overdue_total_count,
    MIN(CASE WHEN status IN ('pending', 'overdue') AND type = 'income' AND category = 'rent' THEN operation_date END) AS next_payment_date
FROM operations
WHERE owner_id = $1 AND property_id = $2 AND deleted_at IS NULL;
```

**Step 3: Сгенерировать sqlc-код**

Run:
```bash
cd apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
```

Expected: `internal/platform/generated/postgres/*.go` обновлены, появились методы `ListLeasesByProperty` и `GetPropertyOperationsSummary`.

**Step 4: Проверить компиляцию backend**

Run:
```bash
cd apps/backend && go build ./...
```

Expected: `ok` / no errors.

**Step 5: Stage**

```bash
git add apps/backend/db/queries/leases.sql apps/backend/db/queries/operations.sql apps/backend/internal/platform/generated/postgres/
```

---

## Task 2: Backend — репозитории и порты

**Files:**
- Modify: `apps/backend/internal/leases/application/ports.go`
- Modify: `apps/backend/internal/leases/adapters/postgres/repository.go`
- Create: `apps/backend/internal/leases/application/summary.go` (тип summary)

**Step 1: Добавить методы в интерфейсы**

В `apps/backend/internal/leases/application/ports.go` в `LeaseRepository` добавить:

```go
ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Lease, error)
```

В `OperationRepository` добавить:

```go
GetPropertyOperationsSummary(ctx context.Context, ownerID, propertyID uuid.UUID) (OperationsSummary, error)
```

**Step 2: Создать тип summary**

Создать `apps/backend/internal/leases/application/summary.go`:

```go
package application

import "time"

type OperationsSummary struct {
	MonthlyProfitKopecks int64
	AllTimeProfitKopecks int64
	OverdueRentCount     int
	OverdueTotalCount    int
	NextPaymentDate      *time.Time
}
```

**Step 3: Реализовать `ListByProperty` в postgres**

В `apps/backend/internal/leases/adapters/postgres/repository.go` добавить метод к `LeaseRepository`:

```go
func (r *LeaseRepository) ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Lease, error) {
	rows, err := r.q().ListLeasesByProperty(ctx, postgres.ListLeasesByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return nil, err
	}
	leases := make([]domain.Lease, 0, len(rows))
	for _, row := range rows {
		lease, err := leaseFromRow(row)
		if err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	return leases, nil
}
```

**Step 4: Реализовать `GetPropertyOperationsSummary` в postgres**

В `apps/backend/internal/leases/adapters/postgres/repository.go` добавить метод к `OperationRepository`:

```go
func (r *OperationRepository) GetPropertyOperationsSummary(ctx context.Context, ownerID, propertyID uuid.UUID) (application.OperationsSummary, error) {
	row, err := r.q().GetPropertyOperationsSummary(ctx, postgres.GetPropertyOperationsSummaryParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return application.OperationsSummary{}, err
	}
	return application.OperationsSummary{
		MonthlyProfitKopecks: row.MonthlyProfitKopecks,
		AllTimeProfitKopecks: row.AllTimeProfitKopecks,
		OverdueRentCount:     int(row.OverdueRentCount),
		OverdueTotalCount:    int(row.OverdueTotalCount),
		NextPaymentDate:      pgconv.DatePtrFromPgtype(row.NextPaymentDate),
	}, nil
}
```

**Step 5: Build**

Run:
```bash
cd apps/backend && go build ./...
```

Expected: no errors.

**Step 6: Stage**

```bash
git add apps/backend/internal/leases/application/ports.go apps/backend/internal/leases/application/summary.go apps/backend/internal/leases/adapters/postgres/repository.go
```

---

## Task 3: Backend — сервисные методы

**Files:**
- Modify: `apps/backend/internal/properties/application/service.go`
- Modify: `apps/backend/internal/leases/application/service.go`
- Modify: `apps/backend/internal/leases/application/operation_service.go`

**Step 1: `PropertyService.ListPropertyLeases`**

В `apps/backend/internal/properties/application/service.go` добавить:

```go
func (s *PropertyService) ListPropertyLeases(ctx context.Context, ownerID, propertyID uuid.UUID) ([]leasesdomain.Lease, error) {
	if _, err := s.GetProperty(ctx, ownerID, propertyID); err != nil {
		return nil, err
	}
	leases, err := s.leaseRepo.ListByProperty(ctx, ownerID, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list property leases: %w", err)
	}
	result := make([]leasesdomain.Lease, 0, len(leases))
	for _, lease := range leases {
		lease.Status = lease.EffectiveStatus(s.clock.Now())
		result = append(result, lease)
	}
	return result, nil
}
```

> `leaseRepo` нужно будет добавить в `PropertyService` как зависимость. Обновить конструктор и `NewPropertyService`.

**Step 2: `LeaseService.ReturnDeposit`**

В `apps/backend/internal/leases/application/service.go` добавить:

```go
func (s *LeaseService) ReturnDeposit(ctx context.Context, ownerID, leaseID uuid.UUID) (domain.Lease, domain.Operation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txLeases := s.leases.WithTx(tx)
	txOps := s.operations.WithTx(tx)

	lease, err := txLeases.GetByIDAndOwnerForUpdate(ctx, leaseID, ownerID)
	if err != nil {
		return domain.Lease{}, domain.Operation{}, err
	}
	if lease.DepositAmountKopecks <= 0 {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("%w: no deposit to return", ErrInvalidInput)
	}

	opID, err := uuid.NewRandom()
	if err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("generate operation id: %w", err)
	}
	comment := "Возврат залога"
	op := domain.Operation{
		ID:            opID,
		OwnerID:       ownerID,
		PropertyID:    lease.PropertyID,
		LeaseID:       lease.ID,
		Type:          domain.OperationTypeExpense,
		Category:      domain.OperationCategoryOtherExpense,
		Status:        domain.OperationStatusPaid,
		AmountKopecks: lease.DepositAmountKopecks,
		OperationDate: timeutil.Date(s.clock.Now()),
		Comment:       comment,
		IsException:   true,
		CreatedAt:     s.clock.Now(),
		UpdatedAt:     s.clock.Now(),
	}
	if err := op.ValidateStatusForType(); err != nil {
		return domain.Lease{}, domain.Operation{}, err
	}
	createdOp, err := txOps.Create(ctx, op)
	if err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("create deposit return operation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Lease{}, domain.Operation{}, fmt.Errorf("commit tx: %w", err)
	}

	lease.Status = lease.EffectiveStatus(s.clock.Now())
	return lease, createdOp, nil
}
```

**Step 3: `OperationService.GetPropertyOperationsSummary`**

В `apps/backend/internal/leases/application/operation_service.go` добавить:

```go
func (s *OperationService) GetPropertyOperationsSummary(ctx context.Context, ownerID, propertyID uuid.UUID) (OperationsSummary, error) {
	if err := validateProperty(ctx, s.properties, ownerID, propertyID); err != nil {
		return OperationsSummary{}, err
	}
	return s.operations.GetPropertyOperationsSummary(ctx, ownerID, propertyID)
}
```

**Step 4: Build**

Run:
```bash
cd apps/backend && go build ./...
```

Expected: no errors.

**Step 5: Stage**

```bash
git add apps/backend/internal/properties/application/service.go apps/backend/internal/leases/application/service.go apps/backend/internal/leases/application/operation_service.go
```

---

## Task 4: Backend — OpenAPI контракт

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Добавить endpoint’ы**

После `/properties/{id}/unarchive` (строка ~262) добавить:

```yaml
  /properties/{id}/leases:
    get:
      operationId: listPropertyLeases
      security:
        - sessionCookie: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: Property leases
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PropertyLeasesResponse'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '404':
          $ref: '#/components/responses/NotFound'
  /properties/{id}/operations/summary:
    get:
      operationId: getPropertyOperationsSummary
      security:
        - sessionCookie: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: Property operations summary
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PropertyOperationsSummaryResponse'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '404':
          $ref: '#/components/responses/NotFound'
```

После `/leases/{id}/complete` (строка ~861) добавить:

```yaml
  /leases/{id}/deposit-return:
    post:
      operationId: returnLeaseDeposit
      security:
        - sessionCookie: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: Deposit returned
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LeaseResponse'
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '404':
          $ref: '#/components/responses/NotFound'
        '403':
          $ref: '#/components/responses/SubscriptionBlocked'
```

**Step 2: Добавить схемы**

После `PropertyPhotosResponse` добавить:

```yaml
    PropertyLeasesResponse:
      type: object
      required: [items]
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/LeaseResponse'

    PropertyOperationsSummaryResponse:
      type: object
      required: [monthly_profit_kopecks, all_time_profit_kopecks, overdue_rent_count, overdue_total_count]
      properties:
        monthly_profit_kopecks:
          type: integer
        all_time_profit_kopecks:
          type: integer
        overdue_rent_count:
          type: integer
        overdue_total_count:
          type: integer
        next_payment_date:
          type: string
          format: date
          nullable: true
```

**Step 3: Проверить валидность OpenAPI**

Run:
```bash
cd apps/backend
npx @apidevtools/swagger-cli validate api/openapi/openapi.yaml
```

Expected: `api/openapi/openapi.yaml is valid`.

**Step 4: Stage**

```bash
git add apps/backend/api/openapi/openapi.yaml
```

---

## Task 5: Backend — HTTP handlers и wiring

**Files:**
- Modify: `apps/backend/internal/platform/httpapi/property_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/lease_handlers.go`
- Modify: `apps/backend/internal/platform/openapi/generated.gen.go` (regenerated)
- Modify: `apps/backend/internal/platform/httpapi/server.go` (if manual wiring)

**Step 1: Сгенерировать backend OpenAPI код**

Run:
```bash
cd apps/backend
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest --config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

Expected: `internal/platform/openapi/generated.gen.go` обновлён, появились методы `ListPropertyLeases`, `GetPropertyOperationsSummary`, `ReturnLeaseDeposit` в server interface.

**Step 2: Реализовать `PropertyHandlers.ListPropertyLeases`**

В `apps/backend/internal/platform/httpapi/property_handlers.go` добавить:

```go
func (h *PropertyHandlers) ListPropertyLeases(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	leases, err := h.svc.ListPropertyLeases(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.LeaseResponse, 0, len(leases))
	contacts, err := h.tenantContactIDs(r.Context(), ownerID, leases)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}
	for _, lease := range leases {
		resp, err := h.leaseResponse(r.Context(), ownerID, lease, contacts)
		if err != nil {
			h.handlePropertyError(w, r, err)
			return
		}
		items = append(items, resp)
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.PropertyLeasesResponse{Items: items})
}
```

> Потребуется внедрить `tenantContactSvc` и `leaseResponse` в `PropertyHandlers`, либо создать отдельный helper. В плане предполагается, что `PropertyHandlers` получит `tenantContactSvc` и метод `leaseResponse` (можно скопировать из `LeaseHandlers`).

**Step 3: Реализовать `PropertyHandlers.GetPropertyOperationsSummary`**

```go
func (h *PropertyHandlers) GetPropertyOperationsSummary(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	summary, err := h.opSvc.GetPropertyOperationsSummary(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.PropertyOperationsSummaryResponse{
		MonthlyProfitKopecks: summary.MonthlyProfitKopecks,
		AllTimeProfitKopecks: summary.AllTimeProfitKopecks,
		OverdueRentCount:     summary.OverdueRentCount,
		OverdueTotalCount:    summary.OverdueTotalCount,
		NextPaymentDate:      datePtrToOpenAPI(summary.NextPaymentDate),
	})
}
```

**Step 4: Реализовать `LeaseHandlers.ReturnLeaseDeposit`**

```go
func (h *LeaseHandlers) ReturnLeaseDeposit(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	lease, _, err := h.leaseSvc.ReturnDeposit(r.Context(), ownerID, id)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	resp, err := h.leaseResponse(r.Context(), ownerID, lease, nil)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}
```

**Step 5: Build**

Run:
```bash
cd apps/backend && go build ./...
```

Expected: no errors.

**Step 6: Stage**

```bash
git add apps/backend/internal/platform/httpapi/property_handlers.go apps/backend/internal/platform/httpapi/lease_handlers.go apps/backend/internal/platform/openapi/generated.gen.go
```

---

## Task 6: Backend — тесты

**Files:**
- Create/Modify: `apps/backend/internal/properties/application/service_test.go`
- Create/Modify: `apps/backend/internal/leases/application/service_test.go`
- Create/Modify: `apps/backend/internal/platform/httpapi/property_handlers_test.go`
- Create/Modify: `apps/backend/internal/platform/httpapi/lease_handlers_test.go`

**Step 1: Написать тесты для `ListPropertyLeases`**

Пример:

```go
func TestPropertyService_ListPropertyLeases(t *testing.T) {
	// arrange: создать объект и две аренды
	// act: ListPropertyLeases
	// assert: вернулись обе, статусы effective
}
```

**Step 2: Написать тесты для `ReturnDeposit`**

Проверить: создание операции-расхода, сумма равна залогу, ошибка при отсутствии залога.

**Step 3: Написать тесты для `GetPropertyOperationsSummary`**

Проверить: прибыль, количество просроченных, next_payment_date.

**Step 4: Написать HTTP-тесты**

Проверить `GET /properties/{id}/leases`, `GET /properties/{id}/operations/summary`, `POST /leases/{id}/deposit-return` возвращают 200/404/401.

**Step 5: Run backend tests**

Run:
```bash
cd apps/backend && go test ./internal/properties/... ./internal/leases/... ./internal/platform/httpapi/...
```

Expected: PASS.

**Step 6: Stage**

```bash
git add apps/backend/internal/properties/application/service_test.go apps/backend/internal/leases/application/service_test.go apps/backend/internal/platform/httpapi/property_handlers_test.go apps/backend/internal/platform/httpapi/lease_handlers_test.go
```

---

## Task 7: Frontend — генерация типов

**Files:**
- Modify: `apps/frontend/shared/api/generated.ts` (regenerated)

**Step 1: Сгенерировать TypeScript клиент**

Run:
```bash
cd apps/frontend && npm run generate:api
```

Expected: `shared/api/generated.ts` обновлён, появились `PropertyLeasesResponse`, `PropertyOperationsSummaryResponse`, endpoint’ы `/properties/{id}/leases` и т.д.

**Step 2: Проверить TS компиляцию**

Run:
```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

**Step 3: Stage**

```bash
git add apps/frontend/shared/api/generated.ts
```

---

## Task 8: Frontend — feature hooks

**Files:**
- Modify: `apps/frontend/features/leases/api/keys.ts`
- Modify: `apps/frontend/features/leases/api/hooks.ts`
- Modify: `apps/frontend/features/operations/api/keys.ts`
- Modify: `apps/frontend/features/operations/api/hooks.ts`

**Step 1: Добавить ключи**

В `apps/frontend/features/leases/api/keys.ts`:

```ts
export const leaseKeys = {
  all: ['leases'] as const,
  detail: (id: string) => [...leaseKeys.all, id] as const,
  byProperty: (propertyId: string) => [...leaseKeys.all, 'property', propertyId] as const,
  reminders: (id: string) => [...leaseKeys.all, id, 'reminders'] as const,
};
```

В `apps/frontend/features/operations/api/keys.ts`:

```ts
export const operationKeys = {
  byProperty: (propertyId: string) => ['properties', propertyId, 'operations'] as const,
  detail: (id: string) => ['operations', id] as const,
  reminders: (propertyId: string, operationId: string) =>
    ['properties', propertyId, 'operations', operationId, 'reminders'] as const,
  summary: (propertyId: string) => ['properties', propertyId, 'operations', 'summary'] as const,
};
```

**Step 2: Добавить хуки**

В `apps/frontend/features/leases/api/hooks.ts`:

```ts
type PropertyLeasesResponse = components['schemas']['PropertyLeasesResponse'];

export function usePropertyLeases(propertyId: string): UseQueryResult<PropertyLeasesResponse, ApiError> {
  return useQuery({
    queryKey: leaseKeys.byProperty(propertyId),
    queryFn: () => apiClient<PropertyLeasesResponse>(`/properties/${propertyId}/leases`),
    enabled: Boolean(propertyId),
  });
}

export function useReturnDeposit(): UseMutationResult<
  LeaseResponse,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id) =>
      apiClient<LeaseResponse>(`/leases/${id}/deposit-return`, { method: 'POST' }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.detail(id) });
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
    },
  });
}
```

В `apps/frontend/features/operations/api/hooks.ts`:

```ts
type PropertyOperationsSummaryResponse =
  components['schemas']['PropertyOperationsSummaryResponse'];

export function usePropertyOperationsSummary(
  propertyId: string,
): UseQueryResult<PropertyOperationsSummaryResponse, ApiError> {
  return useQuery({
    queryKey: operationKeys.summary(propertyId),
    queryFn: () =>
      apiClient<PropertyOperationsSummaryResponse>(
        `/properties/${propertyId}/operations/summary`,
      ),
    enabled: Boolean(propertyId),
  });
}
```

**Step 3: Lint**

Run:
```bash
cd apps/frontend && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/features/leases/api/keys.ts apps/frontend/features/leases/api/hooks.ts apps/frontend/features/operations/api/keys.ts apps/frontend/features/operations/api/hooks.ts
```

---

## Task 9: Frontend — status helpers для страницы объекта

**Files:**
- Create: `apps/frontend/widgets/property-detail/lib/get-property-page-status.ts`
- Create: `apps/frontend/widgets/property-detail/lib/format-awaiting-start.ts`

**Step 1: Тип состояния страницы**

```ts
export type PropertyPageStatus =
  | 'rented'
  | 'requires_action'
  | 'awaiting_start'
  | 'finished'
  | 'free'
  | 'maintenance'
  | 'archived';

export function getPropertyPageStatus(
  propertyStatus: PropertyStatus,
  leases: Lease[],
): PropertyPageStatus {
  if (propertyStatus === 'archived') return 'archived';
  if (propertyStatus === 'maintenance') return 'maintenance';

  const openLease = leases.find((l) => l.status !== 'completed' && l.status !== 'archived');
  if (openLease) {
    if (openLease.status === 'requires_action') return 'requires_action';
    if (openLease.status === 'awaiting_start') return 'awaiting_start';
    return 'rented';
  }

  const finished = leases.find((l) => l.status === 'completed');
  if (finished) return 'finished';

  return 'free';
}
```

**Step 2: Хелпер для текста «Аренда через …»**

```ts
import { formatDuration } from '@/shared/lib/format-duration';

export function formatAwaitingStart(startDate: string): string {
  return `Аренда через ${formatDuration(new Date().toISOString(), startDate)}`;
}
```

**Step 3: Stage**

```bash
git add apps/frontend/widgets/property-detail/lib/get-property-page-status.ts apps/frontend/widgets/property-detail/lib/format-awaiting-start.ts
```

---

## Task 10: Frontend — UI-kit страницы объекта

**Files:**
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailStatusBadge.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailStatusBadge.module.css`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailSection.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailSection.module.css`

**Step 1: Бейдж статуса страницы**

```tsx
'use client';

import type { JSX } from 'react';
import {
  StatusGood,
  StatusWarning,
  StatusDanger,
  StatusInfo,
  StatusDoor,
} from '@/shared/assets/icons';
import type { PropertyPageStatus } from '../lib/get-property-page-status';
import styles from './PropertyDetailStatusBadge.module.css';

const config: Record<PropertyPageStatus, { label: string; color: string; icon: React.ComponentType<{ className?: string }> }> = {
  rented: { label: 'Арендована', color: '#34C771', icon: StatusGood },
  requires_action: { label: 'Требует действия', color: '#FF4646', icon: StatusDanger },
  awaiting_start: { label: 'Аренда скоро начнётся', color: '#2B7FFF', icon: StatusInfo },
  finished: { label: 'Аренда завершена', color: '#2B7FFF', icon: StatusInfo },
  free: { label: 'Не арендована', color: '#A1A3A6', icon: StatusDoor },
  maintenance: { label: 'На ремонте', color: '#EBB800', icon: StatusWarning },
  archived: { label: 'В архиве', color: '#A1A3A6', icon: StatusDoor },
};

export function PropertyDetailStatusBadge({ status, text }: { readonly status: PropertyPageStatus; readonly text?: string }): JSX.Element {
  const item = config[status];
  const Icon = item.icon;
  return (
    <span className={styles.badge} style={{ color: item.color }}>
      <Icon className={styles.icon} />
      {text ?? item.label}
    </span>
  );
}
```

**Step 2: Обёртка секции**

```tsx
import type { JSX, ReactNode } from 'react';
import styles from './PropertyDetailSection.module.css';

export function PropertyDetailSection({ children }: { readonly children: ReactNode }): JSX.Element {
  return <section className={styles.section}>{children}</section>;
}
```

CSS:

```css
.section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
```

**Step 3: Stage**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyDetailStatusBadge.tsx apps/frontend/widgets/property-detail/ui/PropertyDetailStatusBadge.module.css apps/frontend/widgets/property-detail/ui/PropertyDetailSection.tsx apps/frontend/widgets/property-detail/ui/PropertyDetailSection.module.css
```

---

## Task 11: Frontend — Header, Gallery, Action Menu

**Files:**
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailHeader.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailHeader.module.css`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyGallery.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyGallery.module.css`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyActionMenu.tsx`

**Step 1: Header**

Использовать `IconLink` (назад), `IconButton` (more), `Popover` из HeroUI.

```tsx
'use client';

import { IconLink } from '@/shared/ui/icon-link';
import { IconButton } from '@/shared/ui/icon-button';
import { ArrowLeft, Menu } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import styles from './PropertyDetailHeader.module.css';

export function PropertyDetailHeader({ onOpenMenu }: { readonly onOpenMenu: () => void }) {
  return (
    <header className={styles.root}>
      <IconLink href={ROUTES.properties} aria-label="Назад">
        <ArrowLeft />
      </IconLink>
      <h1 className={styles.title}>Мой объект</h1>
      <IconButton aria-label="Действия" onPress={onOpenMenu}>
        <Menu />
      </IconButton>
    </header>
  );
}
```

**Step 2: Gallery**

Показывать первое фото из `property.photos`, стрелки — заглушкой (без навигации, если фото одно).

```tsx
export function PropertyGallery({ photos }: { readonly photos?: { url: string }[] }) {
  const src = photos?.[0]?.url;
  return (
    <div className={styles.root}>
      {src ? <img src={src} alt="" className={styles.image} /> : <div className={styles.placeholder} />}
    </div>
  );
}
```

**Step 3: Action Menu**

HeroUI `Popover`/`Drawer` с пунктами: Редактировать объект, На ремонт, Завершить аренду, Перевести в архив. Принимает колбэки.

```tsx
export function PropertyActionMenu({
  isOpen,
  onClose,
  onEdit,
  onMaintenance,
  onEndLease,
  onArchive,
}: PropertyActionMenuProps) { /* ... */ }
```

**Step 4: Stage**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyDetailHeader.tsx apps/frontend/widgets/property-detail/ui/PropertyDetailHeader.module.css apps/frontend/widgets/property-detail/ui/PropertyGallery.tsx apps/frontend/widgets/property-detail/ui/PropertyGallery.module.css apps/frontend/widgets/property-detail/ui/PropertyActionMenu.tsx
```

---

## Task 12: Frontend — секция «Аренда»

**Files:**
- Create: `apps/frontend/widgets/property-detail/ui/PropertyLeaseCard.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyLeaseCard.module.css`

**Step 1: Реализовать карточку**

Принимает `lease: Lease | undefined`, `overdueRentCount: number`, `status: PropertyPageStatus`.

- Заголовок «Аренда» + стрелка.
- Под-плашка «N просроченных платежа» если `overdueRentCount > 0`.
- Сумма, срок, `LeaseProgressBar`, арендатор, месяц.
- Кнопки в зависимости от статуса:
  - `rented`/`requires_action`: «Оплатить аренду», «Все операции».
  - `finished`: «Продлить», «Завершить».
  - `awaiting_start`: «Начать аренду».
  - `free`: «Создать аренду».
  - `archived`: disabled.

**Step 2: Stage**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyLeaseCard.tsx apps/frontend/widgets/property-detail/ui/PropertyLeaseCard.module.css
```

---

## Task 13: Frontend — секции «Арендатор», «Платежи», «Операции объекта», «Информация об объекте»

**Files:**
- Create: `apps/frontend/widgets/property-detail/ui/PropertyTenantCard.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyTenantCard.module.css`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyPaymentsCard.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyPaymentsCard.module.css`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyOperationsCard.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyOperationsCard.module.css`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyInfoCard.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyInfoCard.module.css`

**Step 1: Tenant card**

Показывает `lease.tenantContact?.name` + фамилию/отчество или CTA «Добавить арендатора».

**Step 2: Payments card**

Список из `useOperationsByProperty` (последние 3-5). Под-плашка просрочки. Кнопки «Внести платёж» / «Запланировать» или «Добавить платежи» в пустом состоянии.

**Step 3: Operations card (P&L)**

Использует `usePropertyOperationsSummary`. Показывает название объекта, прибыль за текущий месяц / всё время. Пустое состояние — текст из макета.

**Step 4: Info card**

Показывает описание объекта или CTA «Добавить описание».

**Step 5: Stage**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyTenantCard.tsx apps/frontend/widgets/property-detail/ui/PropertyTenantCard.module.css apps/frontend/widgets/property-detail/ui/PropertyPaymentsCard.tsx apps/frontend/widgets/property-detail/ui/PropertyPaymentsCard.module.css apps/frontend/widgets/property-detail/ui/PropertyOperationsCard.tsx apps/frontend/widgets/property-detail/ui/PropertyOperationsCard.module.css apps/frontend/widgets/property-detail/ui/PropertyInfoCard.tsx apps/frontend/widgets/property-detail/ui/PropertyInfoCard.module.css
```

---

## Task 14: Frontend — модальные окна

**Files:**
- Create: `apps/frontend/widgets/property-detail/ui/PropertyBlockedModal.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyEndLeaseModal.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDepositReturnModal.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertySuccessBanner.tsx`

**Step 1: BlockedModal**

Текст: «Статус нельзя изменить, пока есть незавершённая аренда». Кнопки: «Отменить», «Продолжить» (колбэк на завершение аренды).

**Step 2: EndLeaseModal**

Текст: «Завершить аренду? Информацию по этой аренде можно будет посмотреть в разделе «Аренда»».

**Step 3: DepositReturnModal**

Показывает сумму залога, кнопки «Отменить», «Продолжить».

**Step 4: SuccessBanner**

Inline-баннер «Аренда завершена» с кнопкой «Открыть аренду».

**Step 5: Stage**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyBlockedModal.tsx apps/frontend/widgets/property-detail/ui/PropertyEndLeaseModal.tsx apps/frontend/widgets/property-detail/ui/PropertyDepositReturnModal.tsx apps/frontend/widgets/property-detail/ui/PropertySuccessBanner.tsx
```

---

## Task 15: Frontend — Loading / Error

**Files:**
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailLoading.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailLoading.module.css`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailError.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailError.module.css`

**Step 1: Loading**

Использовать `Skeleton` из HeroUI для всех секций.

**Step 2: Error**

Сообщение + кнопка «Повторить» с `refetch`.

**Step 3: Stage**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyDetailLoading.tsx apps/frontend/widgets/property-detail/ui/PropertyDetailLoading.module.css apps/frontend/widgets/property-detail/ui/PropertyDetailError.tsx apps/frontend/widgets/property-detail/ui/PropertyDetailError.module.css
```

---

## Task 16: Frontend — корневой виджет страницы

**Files:**
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailPage.tsx`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailPage.module.css`
- Create: `apps/frontend/widgets/property-detail/index.ts`

**Step 1: PropertyDetailPage**

```tsx
'use client';

import { useParams } from 'next/navigation';
import { useProperty } from '@/features/properties/api/hooks';
import { usePropertyLeases } from '@/features/leases/api/hooks';
import { usePropertyOperationsSummary, useOperationsByProperty } from '@/features/operations/api/hooks';
import { PropertyDetailHeader } from './PropertyDetailHeader';
import { PropertyGallery } from './PropertyGallery';
import { PropertyDetailStatusBadge } from './PropertyDetailStatusBadge';
import { PropertyLeaseCard } from './PropertyLeaseCard';
import { PropertyTenantCard } from './PropertyTenantCard';
import { PropertyPaymentsCard } from './PropertyPaymentsCard';
import { PropertyOperationsCard } from './PropertyOperationsCard';
import { PropertyInfoCard } from './PropertyInfoCard';
import { PropertyActionMenu } from './PropertyActionMenu';
import { PropertyBlockedModal } from './PropertyBlockedModal';
import { PropertyEndLeaseModal } from './PropertyEndLeaseModal';
import { PropertyDepositReturnModal } from './PropertyDepositReturnModal';
import { PropertyDetailLoading } from './PropertyDetailLoading';
import { PropertyDetailError } from './PropertyDetailError';
import { getPropertyPageStatus } from '../lib/get-property-page-status';
import styles from './PropertyDetailPage.module.css';

export function PropertyDetailPage() {
  const { id } = useParams<{ id: string }>();
  const propertyQuery = useProperty(id);
  const leasesQuery = usePropertyLeases(id);
  const summaryQuery = usePropertyOperationsSummary(id);
  const operationsQuery = useOperationsByProperty(id);

  if (propertyQuery.isPending || leasesQuery.isPending) return <PropertyDetailLoading />;
  if (propertyQuery.isError) return <PropertyDetailError error={propertyQuery.error} retry={propertyQuery.refetch} />;

  const property = propertyQuery.data;
  const leases = leasesQuery.data?.items ?? [];
  const pageStatus = getPropertyPageStatus(property.status, leases);

  return (
    <div className={styles.root}>
      <PropertyDetailHeader onOpenMenu={() => setMenuOpen(true)} />
      <PropertyGallery photos={property.photos} />
      <div className={styles.meta}>
        <PropertyDetailStatusBadge status={pageStatus} />
        <h2 className={styles.title}>{property.name}</h2>
        <p className={styles.address}>{property.address}</p>
      </div>
      <PropertyLeaseCard lease={leases[0]} status={pageStatus} overdueRentCount={summaryQuery.data?.overdue_rent_count ?? 0} />
      <PropertyTenantCard lease={leases[0]} />
      <PropertyPaymentsCard operations={operationsQuery.data?.items ?? []} overdueCount={summaryQuery.data?.overdue_total_count ?? 0} />
      <PropertyOperationsCard propertyName={property.name} summary={summaryQuery.data} />
      <PropertyInfoCard description={property.description} />
      <PropertyActionMenu ... />
      <PropertyBlockedModal ... />
      <PropertyEndLeaseModal ... />
      <PropertyDepositReturnModal ... />
    </div>
  );
}
```

> Добавить состояния модалок/меню, обработчики действий, использование `useUpdateProperty`, `useArchiveProperty`, `useCompleteLease`, `useReturnDeposit`, `useUpdateLease`.

**Step 2: Index**

```ts
export { PropertyDetailPage } from './ui/PropertyDetailPage';
```

**Step 3: Stage**

```bash
git add apps/frontend/widgets/property-detail/ui/PropertyDetailPage.tsx apps/frontend/widgets/property-detail/ui/PropertyDetailPage.module.css apps/frontend/widgets/property-detail/index.ts
```

---

## Task 17: Frontend — route и layout

**Files:**
- Create: `apps/frontend/app/(cabinet)/properties/[id]/page.tsx`
- Modify: `apps/frontend/shared/config/routes.ts`
- Modify: `apps/frontend/widgets/properties/ui/PropertyCard.tsx` (ссылка на объект)

**Step 1: Route page**

```tsx
import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PropertyDetailPage } from '@/widgets/property-detail';
import { PropertyDetailLoading } from '@/widgets/property-detail/ui/PropertyDetailLoading';

export const metadata: Metadata = {
  title: 'Мой объект — Arenda Platform',
  description: 'Просмотр объекта недвижимости',
};

export default function PropertyDetailRoutePage({ params }: { params: Promise<{ id: string }> }) {
  return (
    <Suspense fallback={<PropertyDetailLoading />}>
      <PropertyDetailPage />
    </Suspense>
  );
}
```

**Step 2: Routes**

```ts
export const ROUTES = {
  ...,
  property: (id: string) => `/properties/${id}`,
} as const;
```

**Step 3: Ссылка в PropertyCard**

Обернуть заголовок/карточку в `NextLink` на `ROUTES.property(property.id)`. Сохранить существующие действия.

**Step 4: Stage**

```bash
git add apps/frontend/app/(cabinet)/properties/[id]/page.tsx apps/frontend/shared/config/routes.ts apps/frontend/widgets/properties/ui/PropertyCard.tsx
```

---

## Task 18: Frontend — status helpers и мелкие правки

**Files:**
- Modify: `apps/frontend/features/properties/lib/property-statuses.ts` (при необходимости)
- Modify: `apps/frontend/widgets/properties/ui/PropertyStatusBadge.tsx` (добавить новые статусы, если используется)

**Step 1: Расширить display statuses**

Если `PropertyStatusBadge` будет использоваться на странице, добавить `requires_action` и `awaiting_start` в `DisplayStatus` и `config`.

**Step 2: Stage**

```bash
git add apps/frontend/features/properties/lib/property-statuses.ts apps/frontend/widgets/properties/ui/PropertyStatusBadge.tsx
```

---

## Task 19: Frontend — тесты и качество

**Files:**
- Create: `apps/frontend/widgets/property-detail/lib/get-property-page-status.test.ts`
- Create: `apps/frontend/widgets/property-detail/ui/PropertyDetailPage.test.tsx` (опционально)

**Step 1: Unit-тест для `getPropertyPageStatus`**

```ts
import { describe, it, expect } from 'vitest';
import { getPropertyPageStatus } from './get-property-page-status';

describe('getPropertyPageStatus', () => {
  it('returns rented for active lease', () => {
    expect(getPropertyPageStatus('active', [{ status: 'active' } as any])).toBe('rented');
  });
  it('returns archived for archived property', () => {
    expect(getPropertyPageStatus('archived', [])).toBe('archived');
  });
});
```

> Проверить, что в проекте используется vitest/jest. Если тестового раннера нет, пропустить и покрыть позже.

**Step 2: Lint**

Run:
```bash
cd apps/frontend && npm run lint
```

Expected: no errors.

**Step 3: Build**

Run:
```bash
cd apps/frontend && npm run build
```

Expected: Build succeeded.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/property-detail/lib/get-property-page-status.test.ts
```

---

## Task 20: Интеграция и финальная проверка

**Step 1: Поднять backend**

Run:
```bash
make backend-run
```

**Step 2: Поднять frontend**

Run:
```bash
cd apps/frontend && npm run dev
```

**Step 3: Проверить страницу вручную**

- Открыть `/properties` → клик по карточке → `/properties/{id}`.
- Проверить статусы: арендована, не арендована, на ремонте, в архиве, аренда завершена, аренда не началась, требует действия.
- Проверить модалки: на ремонт/архив при активной аренде, завершение аренды, возврат залога.
- Проверить пустые и загрузочные состояния.

**Step 4: Backend lint**

Run:
```bash
make backend-lint
```

Expected: no issues.

**Step 5: Подготовить итоговый индекс**

```bash
git status
```

Expected: все новые/изменённые файлы staged.

---

## Execution handoff

**Plan complete and saved to `docs/plans/2026-06-25-property-detail-implementation-plan.md`.**

**Two execution options:**

1. **Subagent-Driven (this session)** — dispatch fresh subagent per task, review between tasks, fast iteration. REQUIRED SUB-SKILL: `@superpowers:subagent-driven-development`.
2. **Parallel Session (separate)** — open new session with `@superpowers:executing-plans`, batch execution with checkpoints.

**Which approach?**

# Operations Feed Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `/finance/operations` as a simple bank-style feed of concrete operations with all-time default viewing, property filtering, and explicit load-more pagination.

**Architecture:** Keep the existing `Operation` and `RecurringOperation` domain model. Use `GET /operations` as the only data source for the feed, add explicit pagination metadata to `OperationsResponse`, and remove recurring-template rendering from the operations list. Keep recurring-template APIs available for creation/editing flows outside the feed.

**Tech Stack:** Go 1.26, OpenAPI/oapi-codegen, sqlc, PostgreSQL; Next.js 16.2, React 19, TypeScript, TanStack Query, Playwright.

---

## File Structure

Backend contract and HTTP:

- Modify: `apps/backend/api/openapi/openapi.yaml`
  Adds pagination metadata to `OperationsResponse` and bounds `limit`/`offset` for operation list endpoints.
- Modify: `apps/backend/internal/platform/httpapi/operation_handlers.go`
  Normalizes pagination, fetches `limit + 1`, trims responses, and writes metadata.
- Create: `apps/backend/internal/platform/httpapi/operation_handlers_test.go`
  Unit tests for pagination normalization and response metadata helpers.
- Generated: `apps/backend/internal/platform/openapi/generated.gen.go`
  Regenerated from OpenAPI.

Backend persistence:

- Create: `apps/backend/db/migrations/000052_operations_owner_date_id_index.up.sql`
  Adds the owner feed index matching `ORDER BY operation_date DESC, id DESC`.
- Create: `apps/backend/db/migrations/000052_operations_owner_date_id_index.down.sql`
  Drops that index.

Frontend API:

- Modify: `apps/frontend/features/operations/api/hooks.ts`
  Adds a `useInfiniteOperations` hook and shared query-string helpers.
- Modify: `apps/frontend/features/operations/api/keys.ts`
  Keeps query keys stable for paginated operation feeds.
- Generated: `apps/frontend/shared/api/generated.ts`
  Regenerated from OpenAPI.

Frontend feed:

- Modify: `apps/frontend/widgets/operations/ui/OperationsPage.tsx`
  Uses only concrete operations, applies `property_id`, defaults to all time, and renders load more.
- Modify: `apps/frontend/widgets/operations/ui/OperationFilters.tsx`
  Removes kind filtering and adds all-time period behavior.
- Modify: `apps/frontend/widgets/operations/ui/OperationsList.tsx`
  Renders concrete operation rows only.
- Modify: `apps/frontend/widgets/operations/ui/OperationListItem.tsx`
  Accepts a property name from the list instead of casting response DTOs.
- Modify: `apps/frontend/widgets/operations/ui/OperationsPage.module.css`
  Adds layout for the load-more area.
Frontend E2E:

- Modify: `apps/frontend/e2e/operation.spec.ts`
  Adds coverage for all-time default, no recurring-template request from the feed, property filtering, and load-more behavior.

Docs:

- Modify: `CHANGELOG.md`
  Records the user-visible browsing change under `2026-06-30`.

---

## Task 1: Backend Pagination Contract

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`
- Modify: `apps/backend/internal/platform/openapi/generated.gen.go`
- Test later in Task 2: `apps/backend/internal/platform/httpapi/operation_handlers_test.go`

- [ ] **Step 1: Update the `OperationsResponse` schema**

In `apps/backend/api/openapi/openapi.yaml`, replace the current `OperationsResponse` schema with:

```yaml
    OperationsResponse:
      type: object
      required: [items, limit, offset, has_more]
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/OperationResponse'
        limit:
          type: integer
          minimum: 1
        offset:
          type: integer
          minimum: 0
        has_more:
          type: boolean
        next_offset:
          type: integer
          nullable: true
          minimum: 0
```

- [ ] **Step 2: Bound list endpoint query parameters**

In both `GET /properties/{propertyId}/operations` and `GET /operations`, change `limit` and `offset` query parameter schemas to:

```yaml
        - name: limit
          in: query
          required: false
          schema:
            type: integer
            default: 100
            minimum: 1
            maximum: 100
        - name: offset
          in: query
          required: false
          schema:
            type: integer
            default: 0
            minimum: 0
```

For `GET /properties/{propertyId}/operations`, this intentionally changes the documented default from `1000` to `100` so the response shape and pagination behavior are consistent.

- [ ] **Step 3: Regenerate backend OpenAPI types**

Run:

```bash
cd apps/backend
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

Expected: `apps/backend/internal/platform/openapi/generated.gen.go` changes and exposes `Limit`, `Offset`, `HasMore`, and nullable `NextOffset` fields on `openapi.OperationsResponse`.

- [ ] **Step 4: Verify generated code still compiles before handler changes**

Run:

```bash
cd apps/backend
go test ./internal/platform/httpapi -count=1
```

Expected: pass. The failing test is added at the start of Task 2.

---

## Task 2: Backend Pagination Helpers And Handler Wiring

**Files:**
- Create: `apps/backend/internal/platform/httpapi/operation_handlers_test.go`
- Modify: `apps/backend/internal/platform/httpapi/operation_handlers.go`

- [ ] **Step 1: Add failing tests for pagination helpers**

Create `apps/backend/internal/platform/httpapi/operation_handlers_test.go` with:

```go
package httpapi

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

func TestNormalizeOperationPagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		limit      *int
		offset     *int
		wantLimit  int
		wantOffset int
		wantFetch  int
	}{
		{name: "defaults", wantLimit: 100, wantOffset: 0, wantFetch: 101},
		{name: "custom values", limit: ptrInt(25), offset: ptrInt(50), wantLimit: 25, wantOffset: 50, wantFetch: 26},
		{name: "limit below minimum", limit: ptrInt(0), offset: ptrInt(-10), wantLimit: 1, wantOffset: 0, wantFetch: 2},
		{name: "limit above maximum", limit: ptrInt(500), offset: ptrInt(5), wantLimit: 100, wantOffset: 5, wantFetch: 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeOperationPagination(tt.limit, tt.offset)

			if got.limit != tt.wantLimit {
				t.Fatalf("limit = %d, want %d", got.limit, tt.wantLimit)
			}
			if got.offset != tt.wantOffset {
				t.Fatalf("offset = %d, want %d", got.offset, tt.wantOffset)
			}
			if got.fetchLimit != tt.wantFetch {
				t.Fatalf("fetchLimit = %d, want %d", got.fetchLimit, tt.wantFetch)
			}
		})
	}
}

func TestOperationsResponseWithPagination(t *testing.T) {
	t.Parallel()

	ownerID := uuid.New()
	propertyID := uuid.New()
	ops := []domain.Operation{
		testOperation(ownerID, propertyID, "first"),
		testOperation(ownerID, propertyID, "second"),
		testOperation(ownerID, propertyID, "extra"),
	}

	response := operationsResponse(ops, operationPagination{limit: 2, offset: 10, fetchLimit: 3})

	if len(response.Items) != 2 {
		t.Fatalf("items length = %d, want 2", len(response.Items))
	}
	if response.Limit != 2 {
		t.Fatalf("limit = %d, want 2", response.Limit)
	}
	if response.Offset != 10 {
		t.Fatalf("offset = %d, want 10", response.Offset)
	}
	if !response.HasMore {
		t.Fatal("has_more = false, want true")
	}
	if response.NextOffset == nil || *response.NextOffset != 12 {
		t.Fatalf("next_offset = %v, want 12", response.NextOffset)
	}
}

func TestOperationsResponseWithoutNextPage(t *testing.T) {
	t.Parallel()

	ownerID := uuid.New()
	propertyID := uuid.New()
	ops := []domain.Operation{
		testOperation(ownerID, propertyID, "only"),
	}

	response := operationsResponse(ops, operationPagination{limit: 2, offset: 0, fetchLimit: 3})

	if len(response.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(response.Items))
	}
	if response.HasMore {
		t.Fatal("has_more = true, want false")
	}
	if response.NextOffset != nil {
		t.Fatalf("next_offset = %v, want nil", *response.NextOffset)
	}
}

func ptrInt(v int) *int {
	return &v
}

func testOperation(ownerID, propertyID uuid.UUID, name string) domain.Operation {
	now := time.Date(2026, time.June, 30, 12, 0, 0, 0, time.UTC)
	return domain.Operation{
		ID:            uuid.New(),
		OwnerID:       ownerID,
		PropertyID:    propertyID,
		Type:          domain.OperationTypeIncome,
		Category:      domain.OperationCategoryOtherIncome,
		Status:        domain.OperationStatusReceived,
		Name:          name,
		AmountKopecks: 1000,
		OperationDate: now,
		IsException:   true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

var _ = openapi.OperationsResponse{}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd apps/backend
go test ./internal/platform/httpapi -run 'TestNormalizeOperationPagination|TestOperationsResponse' -count=1
```

Expected: fail with undefined names `normalizeOperationPagination`, `operationPagination`, and `operationsResponse`.

- [ ] **Step 3: Add pagination helpers to the handler**

In `apps/backend/internal/platform/httpapi/operation_handlers.go`, add near the top of the file after the `OperationHandlers` type:

```go
const (
	defaultOperationListLimit = 100
	maxOperationListLimit     = 100
	minOperationListLimit     = 1
)

type operationPagination struct {
	limit      int
	offset     int
	fetchLimit int
}

func normalizeOperationPagination(limitParam, offsetParam *int) operationPagination {
	limit := defaultOperationListLimit
	if limitParam != nil {
		limit = *limitParam
	}
	if limit < minOperationListLimit {
		limit = minOperationListLimit
	}
	if limit > maxOperationListLimit {
		limit = maxOperationListLimit
	}

	offset := 0
	if offsetParam != nil {
		offset = *offsetParam
	}
	if offset < 0 {
		offset = 0
	}

	return operationPagination{
		limit:      limit,
		offset:     offset,
		fetchLimit: limit + 1,
	}
}

func operationsResponse(ops []domain.Operation, pagination operationPagination) openapi.OperationsResponse {
	hasMore := len(ops) > pagination.limit
	if hasMore {
		ops = ops[:pagination.limit]
	}

	items := make([]openapi.OperationResponse, 0, len(ops))
	for _, op := range ops {
		items = append(items, operationResponse(op))
	}

	resp := openapi.OperationsResponse{
		Items:   items,
		Limit:   pagination.limit,
		Offset:  pagination.offset,
		HasMore: hasMore,
	}
	if hasMore {
		nextOffset := pagination.offset + len(items)
		resp.NextOffset = &nextOffset
	}

	return resp
}
```

- [ ] **Step 4: Wire helpers into `ListOperationsByProperty`**

In `ListOperationsByProperty`, replace the existing `Limit`/`Offset` handling and response assembly with:

```go
	pagination := normalizeOperationPagination(params.Limit, params.Offset)
	filter.Limit = pagination.fetchLimit
	filter.Offset = pagination.offset

	ops, err := h.svc.ListOperationsByProperty(r.Context(), ownerID, propertyId, filter)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, operationsResponse(ops, pagination))
```

- [ ] **Step 5: Wire helpers into `ListOperations`**

In `ListOperations`, replace the existing `Limit`/`Offset` handling and response assembly with:

```go
	pagination := normalizeOperationPagination(params.Limit, params.Offset)
	filter.Limit = pagination.fetchLimit
	filter.Offset = pagination.offset

	ops, err := h.svc.ListOperations(r.Context(), ownerID, filter)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, operationsResponse(ops, pagination))
```

- [ ] **Step 6: Run focused backend tests**

Run:

```bash
cd apps/backend
go test ./internal/platform/httpapi -run 'TestNormalizeOperationPagination|TestOperationsResponse' -count=1
```

Expected: pass.

- [ ] **Step 7: Run generated-code compile check**

Run:

```bash
cd apps/backend
go test ./internal/platform/httpapi -count=1
```

Expected: pass.

---

## Task 3: PostgreSQL Feed Index

**Files:**
- Create: `apps/backend/db/migrations/000052_operations_owner_date_id_index.up.sql`
- Create: `apps/backend/db/migrations/000052_operations_owner_date_id_index.down.sql`

- [ ] **Step 1: Add the up migration**

Create `apps/backend/db/migrations/000052_operations_owner_date_id_index.up.sql`:

```sql
CREATE INDEX IF NOT EXISTS idx_operations_owner_operation_date_id_not_deleted
  ON operations(owner_id, operation_date DESC, id DESC)
  WHERE deleted_at IS NULL;
```

- [ ] **Step 2: Add the down migration**

Create `apps/backend/db/migrations/000052_operations_owner_date_id_index.down.sql`:

```sql
DROP INDEX IF EXISTS idx_operations_owner_operation_date_id_not_deleted;
```

- [ ] **Step 3: Verify migration files are syntactically plain SQL**

Run:

```bash
sed -n '1,40p' apps/backend/db/migrations/000052_operations_owner_date_id_index.up.sql
sed -n '1,40p' apps/backend/db/migrations/000052_operations_owner_date_id_index.down.sql
```

Expected: the two SQL statements above print exactly.

---

## Task 4: Frontend API Hooks For Load More

**Files:**
- Modify: `apps/frontend/shared/api/generated.ts`
- Modify: `apps/frontend/features/operations/api/hooks.ts`
- Modify: `apps/frontend/features/operations/api/keys.ts`

- [ ] **Step 1: Regenerate frontend API types**

Run:

```bash
cd apps/frontend
npm run generate:api
```

Expected: `apps/frontend/shared/api/generated.ts` changes and `OperationsResponse` includes `limit`, `offset`, `has_more`, and `next_offset`.

- [ ] **Step 2: Update operation query key typing**

In `apps/frontend/features/operations/api/keys.ts`, make the shared filter type include optional pagination fields as strings:

```ts
type OperationKeyFilters = {
  type?: string | string[];
  status?: string | string[];
  category?: string | string[];
  property_id?: string;
  from?: string;
  to?: string;
  recurring_operation_id?: string;
  lease_id?: string;
  limit?: string;
  offset?: string;
};

export const operationKeys = {
  byProperty: (
    propertyId: string,
    filters?: Omit<OperationKeyFilters, 'property_id' | 'lease_id'>,
  ) => ['properties', propertyId, 'operations', filters ?? {}] as const,
  detail: (id: string) => ['operations', id] as const,
  operations: (filters: OperationKeyFilters) => ['operations', filters] as const,
  infiniteOperations: (filters: Omit<OperationKeyFilters, 'offset'>) =>
    ['operations', 'infinite', filters] as const,
  summary: (propertyId: string) =>
    ['properties', propertyId, 'operations', 'summary'] as const,
  recurringByProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
};
```

- [ ] **Step 3: Extract query-string helpers in operation hooks**

In `apps/frontend/features/operations/api/hooks.ts`, update the imports:

```ts
import { useMemo } from 'react';
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type InfiniteData,
  type UseInfiniteQueryResult,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
```

Add these helpers above `useOperations`:

```ts
type NormalizedOperationsFilters = Record<string, string | string[]>;

function normalizeOperationsFilters(filters: OperationsFilters): NormalizedOperationsFilters {
  const result: NormalizedOperationsFilters = {};
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') {
      return;
    }
    if (typeof value === 'number') {
      result[key] = String(value);
      return;
    }
    if (Array.isArray(value)) {
      const filtered = value.filter((v) => v !== '');
      if (filtered.length > 0) {
        result[key] = filtered;
      }
      return;
    }
    result[key] = value;
  });
  return result;
}

function operationsQueryString(filters: NormalizedOperationsFilters): string {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (Array.isArray(value)) {
      value.forEach((v) => params.append(key, v));
      return;
    }
    params.set(key, value);
  });
  return params.toString();
}
```

- [ ] **Step 4: Refactor `useOperations` to use the helpers**

Replace the local normalization and query-string blocks inside `useOperations` with:

```ts
  const normalized = useMemo(() => normalizeOperationsFilters(filters), [filters]);
  const queryString = useMemo(() => operationsQueryString(normalized), [normalized]);
```

Keep the existing `useQuery` return.

- [ ] **Step 5: Refactor `useOperationsByProperty` to use the helpers**

Replace the local normalization and query-string blocks inside `useOperationsByProperty` with:

```ts
  const normalized = useMemo(
    () => normalizeOperationsFilters(filters ?? {}),
    [filters],
  );
  const queryString = useMemo(() => operationsQueryString(normalized), [normalized]);
```

Keep the existing `useQuery` return.

- [ ] **Step 6: Add `useInfiniteOperations`**

Add below `useOperations`:

```ts
export function useInfiniteOperations(
  filters: Omit<OperationsFilters, 'offset'> = {},
  options: { enabled?: boolean } = {},
): UseInfiniteQueryResult<InfiniteData<OperationsResponse>, ApiError> {
  const normalized = useMemo(() => normalizeOperationsFilters(filters), [filters]);

  return useInfiniteQuery({
    queryKey: operationKeys.infiniteOperations(normalized),
    initialPageParam: 0,
    queryFn: ({ pageParam }) => {
      const pageFilters = {
        ...normalized,
        offset: String(pageParam),
      };
      const queryString = operationsQueryString(pageFilters);
      return apiClient<OperationsResponse>(`/operations${queryString ? `?${queryString}` : ''}`);
    },
    getNextPageParam: (lastPage) => lastPage.next_offset ?? undefined,
    enabled: options.enabled,
  });
}
```

- [ ] **Step 7: Run frontend type check via lint**

Run:

```bash
cd apps/frontend
npm run lint
```

Expected: lint may fail because the page still uses old feed types. Continue to Task 5 before treating frontend lint as a gate.

---

## Task 5: Simplify The Operations Feed UI

**Files:**
- Modify: `apps/frontend/widgets/operations/ui/OperationsPage.tsx`
- Modify: `apps/frontend/widgets/operations/ui/OperationFilters.tsx`
- Modify: `apps/frontend/widgets/operations/ui/OperationsList.tsx`
- Modify: `apps/frontend/widgets/operations/ui/OperationListItem.tsx`
- Modify: `apps/frontend/widgets/operations/ui/OperationsPage.module.css`

- [ ] **Step 1: Update `OperationListItem` to accept property names explicitly**

In `apps/frontend/widgets/operations/ui/OperationListItem.tsx`, remove the `OperationWithPropertyName` type and add `propertyName` to props:

```ts
export type OperationListItemProps = {
  readonly operation: OperationResponse;
  readonly showProperty?: boolean;
  readonly propertyName?: string;
  readonly variant?: OperationListItemVariant;
};
```

Update the function signature:

```ts
export function OperationListItem({
  operation,
  showProperty = false,
  propertyName,
  variant = 'default',
}: OperationListItemProps): JSX.Element {
```

Replace property meta handling with:

```ts
  if (showProperty && propertyName) {
    metaItems.push(propertyName);
  }
```

- [ ] **Step 2: Simplify `OperationsList` to concrete operations**

In `apps/frontend/widgets/operations/ui/OperationsList.tsx`, replace the file body with:

```tsx
'use client';

import type { JSX, ReactNode } from 'react';
import type { components } from '@/shared/api/generated';
import { OperationListItem } from './OperationListItem';
import styles from './OperationsList.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

export type OperationsListProps = {
  readonly operations: ReadonlyArray<OperationResponse>;
  readonly propertyNameById?: ReadonlyMap<string, string>;
  readonly showProperty?: boolean;
  readonly emptyState?: ReactNode;
};

export function OperationsList({
  operations,
  propertyNameById,
  showProperty = false,
  emptyState,
}: OperationsListProps): JSX.Element {
  if (operations.length === 0 && emptyState) {
    return <div className={styles.root}>{emptyState}</div>;
  }

  return (
    <ul className={styles.root}>
      {operations.map((operation) => (
        <li key={operation.id}>
          <OperationListItem
            operation={operation}
            showProperty={showProperty}
            propertyName={propertyNameById?.get(operation.property_id)}
          />
        </li>
      ))}
    </ul>
  );
}
```

- [ ] **Step 3: Update property operations page to the new `OperationsList` API**

In `apps/frontend/widgets/property-detail/ui/PropertyOperationsPage.tsx`, replace:

```tsx
            <OperationsList
              items={operations.map((operation) => ({ kind: 'onetime', data: operation }))}
            />
```

with:

```tsx
            <OperationsList operations={operations} />
```

- [ ] **Step 4: Remove kind state from `OperationFilters`**

In `apps/frontend/widgets/operations/ui/OperationFilters.tsx`, replace the type definitions with:

```ts
export type OperationPeriod = 'all' | 'month' | 'quarter' | 'year';

export type OperationFiltersState = {
  readonly period: OperationPeriod;
  readonly from?: string;
  readonly to?: string;
  readonly status: ReadonlyArray<string>;
};

export type OperationFiltersProps = {
  readonly filters: OperationFiltersState;
  readonly onChange: (filters: OperationFiltersState) => void;
};
```

Remove the import of `OperationKind`.

- [ ] **Step 5: Add all-time period behavior**

In `OperationFilters.tsx`, replace `type Period = 'month' | 'quarter' | 'year';` and `setPeriod` with:

```ts
function setPeriod(
  period: OperationPeriod,
  onChange: (filters: OperationFiltersState) => void,
  currentFilters: OperationFiltersState,
): void {
  if (period === 'all') {
    onChange({ ...currentFilters, period, from: undefined, to: undefined });
    return;
  }

  const now = new Date();
  const range =
    period === 'month'
      ? getCurrentMonthRange()
      : period === 'quarter'
        ? getQuarterRange(now)
        : getYearRange(now);
  onChange({ ...currentFilters, period, from: range.from, to: range.to });
}
```

Update `hasActiveFilters` to:

```ts
function hasActiveFilters(filters: OperationFiltersState): boolean {
  const statusActive = filters.status.length > 0;
  const periodActive = filters.period !== 'all';
  return statusActive || periodActive;
}
```

Update reset:

```ts
  const handleReset = () => {
    onChange({ period: 'all', from: undefined, to: undefined, status: [] });
  };
```

- [ ] **Step 6: Render period chips and remove kind chips**

In `OperationFilters.tsx`, replace the period button group with:

```tsx
        <div className={styles.periodGroup}>
          {([
            ['all', 'Все время'],
            ['month', 'Месяц'],
            ['quarter', 'Квартал'],
            ['year', 'Год'],
          ] as const).map(([period, label]) => (
            <Button
              key={period}
              variant={filters.period === period ? 'secondary' : 'icon-black'}
              size="small"
              onClick={() => setPeriod(period, onChange, filters)}
            >
              {label}
            </Button>
          ))}
        </div>
```

Delete the `KIND_CHIPS` block and the `onKindChange` usage.

- [ ] **Step 7: Refactor `OperationsPage` imports**

In `apps/frontend/widgets/operations/ui/OperationsPage.tsx`:

- remove `type { components }`;
- remove `useRecurringOperations`;
- remove `formatDateForApi`, `startOfMonth`, `endOfMonth`;
- import `useInfiniteOperations` instead of `useOperations`;
- import `type OperationPeriod` from `./OperationFilters` if it is exported.

The resulting operations API import should be:

```ts
import { useInfiniteOperations } from '@/features/operations/api/hooks';
```

- [ ] **Step 8: Remove kind href helpers and unified item types**

Delete from `OperationsPage.tsx`:

```ts
export type OperationKind = 'all' | 'onetime' | 'recurring';
```

Delete `getCurrentMonthRange`, `buildKindHref`, and `UnifiedOperationItem`.

- [ ] **Step 9: Read URL filters in `OperationsPage`**

Inside `OperationsPage`, replace the current `kind`, `from`, and `to` setup with:

```ts
  const propertyId = searchParams.get('property_id') ?? undefined;
  const fromParam = searchParams.get('from') ?? undefined;
  const toParam = searchParams.get('to') ?? undefined;
  const period = (searchParams.get('period') as OperationPeriod | null) ?? 'all';
```

Update the operation filters memo:

```ts
  const filters = useMemo(
    () => ({
      type: type ?? undefined,
      status,
      property_id: propertyId,
      from: fromParam,
      to: toParam,
      limit: 50,
    }),
    [type, status, propertyId, fromParam, toParam],
  );
```

- [ ] **Step 10: Use infinite operations query**

Replace the `useOperations` and `useRecurringOperations` calls with:

```ts
  const {
    data: operationsPages,
    isLoading: operationsLoading,
    isFetching: operationsFetching,
    isFetchingNextPage,
    isError: operationsError,
    refetch: refetchOperations,
    fetchNextPage,
    hasNextPage,
  } = useInfiniteOperations(filters);
```

Set loading and error booleans to:

```ts
  const isLoading = operationsLoading || propertiesLoading;
  const isError = operationsError || propertiesError;
  const isFetching = operationsFetching;
```

Flatten operations:

```ts
  const operations = useMemo(
    () => operationsPages?.pages.flatMap((page) => page.items) ?? [],
    [operationsPages],
  );
```

- [ ] **Step 11: Build property-name map**

In `OperationsPage.tsx`, add:

```ts
  const propertyNameById = useMemo(() => {
    const map = new Map<string, string>();
    properties?.forEach((property) => {
      map.set(property.id, property.name);
    });
    return map;
  }, [properties]);
```

- [ ] **Step 12: Update filter URL writer**

Replace `handleFilterChange` with:

```ts
  const handleFilterChange = (nextFilters: {
    period: OperationPeriod;
    from?: string;
    to?: string;
    status: ReadonlyArray<string>;
  }) => {
    const params = new URLSearchParams(searchParams.toString());

    if (nextFilters.period === 'all') {
      params.delete('period');
      params.delete('from');
      params.delete('to');
    } else {
      params.set('period', nextFilters.period);
      if (nextFilters.from) {
        params.set('from', nextFilters.from);
      }
      if (nextFilters.to) {
        params.set('to', nextFilters.to);
      }
    }

    params.delete('status');
    nextFilters.status.forEach((value) => params.append('status', value));

    const query = params.toString();
    router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false });
  };
```

Delete `handleKindChange`.

- [ ] **Step 13: Update error retry block**

In the error state, replace:

```tsx
          onRetry={() => {
            refetchOperations();
            refetchRecurring();
          }}
```

with:

```tsx
          onRetry={() => {
            refetchOperations();
          }}
```

- [ ] **Step 14: Update filter and list rendering**

Replace the `OperationFilters` call with:

```tsx
        <OperationFilters
          filters={{ period, from: fromParam, to: toParam, status }}
          onChange={handleFilterChange}
        />
```

Replace `OperationsList` rendering with:

```tsx
        <OperationsList
          operations={operations}
          showProperty
          propertyNameById={propertyNameById}
          emptyState={
            <FinanceEmptyState
              title={status.length === 0 && !type && !propertyId && period === 'all' ? 'Нет операций' : 'Нет совпадений'}
              subtitle={
                status.length === 0 && !type && !propertyId && period === 'all'
                  ? 'Добавьте первую операцию, чтобы увидеть её в списке'
                  : 'Попробуйте изменить фильтры'
              }
              actionHref={readonly || status.length > 0 || type || propertyId || period !== 'all' ? undefined : ROUTES.financeCreateOperation}
              actionText={readonly || status.length > 0 || type || propertyId || period !== 'all' ? undefined : 'Добавить операцию'}
            />
          }
        />
```

- [ ] **Step 15: Add the load-more button**

Below `OperationsList`, render:

```tsx
        {hasNextPage && (
          <div className={styles.loadMore}>
            <Button
              variant="secondary"
              size="medium"
              loading={isFetchingNextPage}
              onClick={() => {
                fetchNextPage();
              }}
            >
              Показать ещё
            </Button>
          </div>
        )}
```

Add the `Button` import:

```ts
import { Button } from '@/shared/ui/button';
```

- [ ] **Step 16: Add load-more CSS**

In `apps/frontend/widgets/operations/ui/OperationsPage.module.css`, add:

```css
.loadMore {
  display: flex;
  justify-content: center;
}
```

- [ ] **Step 17: Run frontend lint**

Run:

```bash
cd apps/frontend
npm run lint
```

Expected: pass. If it fails, fix only issues introduced by this task.

---

## Task 6: E2E Coverage For The New Feed

**Files:**
- Modify: `apps/frontend/e2e/operation.spec.ts`

- [ ] **Step 1: Add DB row helper imports**

At the top of `operation.spec.ts`, change:

```ts
import { assertDbState } from './helpers/db';
```

to:

```ts
import { assertDbState, queryValue } from './helpers/db';
```

- [ ] **Step 2: Add test for all-time property feed and load more**

Append inside `test.describe('Operation lifecycle', () => { ... })`:

```ts
  test('operations feed is all-time, property-filtered, and can load more', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);
    const { propertyId, propertyName } = await createProperty(page, user.id, 'E2E Feed Property');

    const insertResult = queryValue(`
      WITH inserted AS (
        INSERT INTO operations (
          owner_id,
          property_id,
          type,
          category,
          name,
          amount_kopecks,
          operation_date,
          status,
          is_exception
        )
        SELECT
          '${user.id}',
          '${propertyId}',
          'income',
          'other_income',
          'Feed Load ' || gs::text,
          1000 + gs,
          DATE '2026-01-01' + gs,
          'received',
          true
        FROM generate_series(1, 65) AS gs
        RETURNING 1
      )
      SELECT count(*)::text FROM inserted
    `);
    expect(insertResult).not.toBeNull();

    await page.goto(`/finance/operations?property_id=${propertyId}`);
    await page.waitForLoadState('networkidle');

    await expect(page.getByText('Feed Load 65', { exact: true })).toBeVisible();
    await expect(page.getByText(propertyName).first()).toBeVisible();
    await expect(page.getByText('Feed Load 1', { exact: true })).toHaveCount(0);

    await page.getByRole('button', { name: /показать ещё/i }).click();
    await expect(page.getByText('Feed Load 1', { exact: true })).toBeVisible();

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });
```

- [ ] **Step 3: Add test that recurring-generated operations render as normal rows**

Append inside the same describe block:

```ts
  test('operations feed does not render recurring templates as feed items', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);
    const { propertyId, propertyName } = await createProperty(page, user.id, 'E2E Recurring Feed Property');

    await page.goto(`/finance/create-operation?type=income&propertyId=${propertyId}`);
    await page.waitForLoadState('networkidle');

    const operationName = `E2E Feed Recurring ${Date.now()}`;
    await page.getByLabel(/сумма операции/i).fill('12000');
    await page.getByLabel(/название операции/i).fill(operationName);
    await page.getByLabel(/категория дохода/i).click();
    await page.getByRole('option', { name: /арендная плата/i }).click();
    await page.getByRole('button', { name: /далее/i }).click();
    await page.getByRole('radio', { name: /каждый месяц/i }).click();
    await page.getByLabel(/дата первого повтора/i).fill('2026-07-05');
    await page.getByLabel(/день операции/i).fill('5');
    await page.getByRole('button', { name: /далее/i }).click();
    await page.getByRole('button', { name: /создать операцию/i }).click();

    await assertDbState(
      `SELECT count(*)::text FROM operations WHERE property_id = '${propertyId}' AND name = '${operationName}' AND recurring_operation_id IS NOT NULL`,
      (value) => Number(value ?? '0') > 0,
      'Recurring operation should generate concrete operations'
    );

    const recurringRequests: string[] = [];
    page.on('request', (request) => {
      if (request.url().includes('/recurring-operations')) {
        recurringRequests.push(request.url());
      }
    });

    await page.goto(`/finance/operations?property_id=${propertyId}`);
    await page.waitForLoadState('networkidle');

    await expect(page.getByText(operationName).first()).toBeVisible();
    await expect(page.getByText(propertyName).first()).toBeVisible();
    await expect(page.getByText(/изменить серию/i)).toHaveCount(0);
    await expect(page.getByText(/приостановить/i)).toHaveCount(0);
    expect(recurringRequests).toHaveLength(0);

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });
```

- [ ] **Step 4: Run the operation E2E spec when local E2E services are running**

Run:

```bash
cd apps/frontend
npx playwright test e2e/operation.spec.ts
```

Expected: pass when the local frontend, backend, and Postgres E2E environment are running. If local E2E services are not running, record that in the final implementation report and still run lint/build plus backend tests.

---

## Task 7: Changelog And Full Verification

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add changelog entry**

Under `## 2026-06-30`, in `### Изменено`, add:

```md
- Раздел «Операции» стал простой лентой денежных записей: операции из ручного ввода, аренды и регулярных шаблонов показываются одинаково, без карточек серий в общем списке; по умолчанию доступна история за всё время и догрузка следующих записей.
```

- [ ] **Step 2: Run backend generated-code and unit verification**

Run:

```bash
make backend-lint
cd apps/backend && go test ./...
cd apps/backend && go vet ./...
```

Expected: all pass.

- [ ] **Step 3: Run frontend verification**

Run:

```bash
cd apps/frontend && npm run lint
cd apps/frontend && npm run build
```

Expected: both pass.

- [ ] **Step 4: Inspect final diff for recurring-template feed regressions**

Run:

```bash
rg -n "useRecurringOperations|RecurringOperationListItem|kind|onetime|recurring" apps/frontend/widgets/operations/ui/OperationsPage.tsx apps/frontend/widgets/operations/ui/OperationsList.tsx apps/frontend/widgets/operations/ui/OperationFilters.tsx
```

Expected: no matches in those three files. Matches in creation/editing files are allowed.

- [ ] **Step 5: Inspect git status**

Run:

```bash
git status --short
```

Expected: only files from this plan are modified.

- [ ] **Step 6: Commit implementation**

Run:

```bash
git add CHANGELOG.md apps/backend apps/frontend
git commit -m "feat: simplify operations feed browsing"
```

Expected: commit succeeds.

---

## Self-Review Notes

- Spec coverage:
  - Concrete operations-only feed: Tasks 5 and 6.
  - All-time default: Tasks 5 and 6.
  - No recurring template cards in feed: Tasks 5 and 6.
  - `property_id` support: Tasks 5 and 6.
  - Load-more pagination: Tasks 1, 2, 4, 5, and 6.
  - Backend pagination metadata: Tasks 1 and 2.
  - PostgreSQL feed index: Task 3.
  - Changelog and verification: Task 7.
- Placeholder scan: no placeholder tokens or unspecified test steps.
- Type consistency:
  - `has_more` in OpenAPI becomes `has_more` in generated TypeScript and `HasMore` in generated Go.
  - `next_offset` in OpenAPI becomes `next_offset` in generated TypeScript and `NextOffset` in generated Go.
  - Frontend uses generated `next_offset`; backend sets generated `NextOffset`.

# Tenants List Page Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the `/tenants` list page that splits tenant contacts into active (with an open lease) and past (no open lease) sections, reusing the project's existing design system and backend-first data model.

**Architecture:** Backend extends `GET /tenant-contacts` to return each contact enriched with `is_active`, `active_lease` and `last_lease`. Frontend maps the response to a new `TenantContact` entity and renders a list page with two sections, card links, loading/error/empty states.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, TanStack Query, CSS Modules, Hero UI v3; Go backend with sqlc, pgx, oapi-codegen.

---

## Notes before starting

- Tests are out of scope for this task. Do not write or modify test files.
- Work in the current checkout (no worktree) per repository rules.
- Do not run `git commit` without explicit user confirmation.
- Required skills to invoke during implementation: `$go`, `$next-best-practices`, `$vercel-react-best-practices`.
- Run generation commands from `apps/backend` after OpenAPI/SQL changes.
- Frontend API types are generated from `apps/frontend` via `npm run generate:api`.

---

## Task 1: Add enriched tenant contacts SQL query

**Files:**
- Modify: `apps/backend/db/queries/tenant_contacts.sql`

**Step 1: Append the enriched list query**

Append to `apps/backend/db/queries/tenant_contacts.sql`:

```sql
-- name: ListTenantContactsWithLeaseStatus :many
SELECT
    tc.id,
    tc.owner_id,
    tc.name,
    tc.surname,
    tc.patronymic,
    tc.phone,
    tc.email,
    tc.comment,
    tc.created_at,
    tc.updated_at,
    l.id AS lease_id,
    l.property_id AS lease_property_id,
    l.status AS lease_status,
    l.start_date AS lease_start_date,
    l.end_date AS lease_end_date,
    l.rent_amount_kopecks AS lease_rent_amount_kopecks,
    l.deposit_amount_kopecks AS lease_deposit_amount_kopecks,
    l.payment_day AS lease_payment_day,
    l.comment AS lease_comment,
    l.created_at AS lease_created_at,
    l.updated_at AS lease_updated_at,
    ROW_NUMBER() OVER (
        PARTITION BY tc.id
        ORDER BY
            CASE WHEN l.status IN ('awaiting_start', 'active', 'requires_action') THEN 0 ELSE 1 END,
            l.updated_at DESC
    ) AS rn
FROM tenant_contacts tc
LEFT JOIN leases l ON l.tenant_contact_id = tc.id
WHERE tc.owner_id = $1;
```

**Step 2: Regenerate sqlc code**

Run:
```bash
cd apps/backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Expected: `apps/backend/internal/platform/generated/postgres/tenant_contacts.sql.go` contains `ListTenantContactsWithLeaseStatus`.

**Step 3: Stage**

```bash
git add apps/backend/db/queries/tenant_contacts.sql apps/backend/internal/platform/generated/postgres/tenant_contacts.sql.go
```

---

## Task 2: Define enriched tenant contact domain type

**Files:**
- Modify: `apps/backend/internal/leases/application/ports.go`

**Step 1: Add a port method**

In `TenantContactRepository` add:

```go
ListWithLeaseStatus(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContactWithLeases, error)
```

**Step 2: Create the domain type**

Create `apps/backend/internal/leases/domain/tenant_contact_with_leases.go`:

```go
package domain

type TenantContactWithLeases struct {
	TenantContact
	ActiveLease *Lease
	LastLease   *Lease
}
```

**Step 3: Stage**

```bash
git add apps/backend/internal/leases/application/ports.go apps/backend/internal/leases/domain/tenant_contact_with_leases.go
```

---

## Task 3: Implement repository method

**Files:**
- Modify: `apps/backend/internal/leases/adapters/postgres/repository.go`

**Step 1: Add `ListWithLeaseStatus` to TenantContactRepository**

Insert after `ListByOwner`:

```go
func (r *TenantContactRepository) ListWithLeaseStatus(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContactWithLeases, error) {
	rows, err := r.q().ListTenantContactsWithLeaseStatus(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}

	contacts := make(map[uuid.UUID]*domain.TenantContactWithLeases, len(rows))
	for _, row := range rows {
		contact, ok := contacts[row.ID.Bytes]
		if !ok {
			contact = &domain.TenantContactWithLeases{
				TenantContact: tenantContactFromRow(row),
			}
			contacts[row.ID.Bytes] = contact
		}

		if row.LeaseID.Valid {
			lease := leaseFromRow(mapLeaseRow(row))
			if lease.Status.IsOpen() && contact.ActiveLease == nil {
				contact.ActiveLease = &lease
			}
			if !lease.Status.IsOpen() && contact.LastLease == nil {
				contact.LastLease = &lease
			}
		}
	}

	result := make([]domain.TenantContactWithLeases, 0, len(contacts))
	for _, contact := range contacts {
		result = append(result, *contact)
	}
	return result, nil
}
```

Wait — `mapLeaseRow(row)` does not exist. We need a helper that builds a postgres lease row from the aliased columns. Instead, inline the mapping:

```go
lease := domain.Lease{
	ID:                   row.LeaseID.Bytes,
	OwnerID:              row.LeaseOwnerID.Bytes, // if selected
	PropertyID:           row.LeasePropertyID.Bytes,
	TenantContactID:      &contact.ID,
	Status:               domain.LeaseStatus(row.LeaseStatus),
	StartDate:            row.LeaseStartDate.Time,
	EndDate:              pgconv.DatePtrFromPgtype(row.LeaseEndDate),
	RentAmountKopecks:    row.LeaseRentAmountKopecks,
	DepositAmountKopecks: row.LeaseDepositAmountKopecks,
	PaymentDay:           int(row.LeasePaymentDay),
	Comment:              pgconv.TextToString(row.LeaseComment),
	CreatedAt:            row.LeaseCreatedAt.Time,
	UpdatedAt:            row.LeaseUpdatedAt.Time,
}
```

Adjust the SQL query to include only the columns needed for `domain.Lease`. If `OwnerID` is required, add `l.owner_id AS lease_owner_id` to the query.

**Step 2: Build backend**

Run:
```bash
cd apps/backend && go build ./...
```

Expected: no errors.

**Step 3: Stage**

```bash
git add apps/backend/internal/leases/adapters/postgres/repository.go
```

---

## Task 4: Add service method

**Files:**
- Modify: `apps/backend/internal/leases/application/tenant_contact_service.go`

**Step 1: Add `ListTenantContactsWithLeaseStatus`**

Insert after `ListTenantContacts`:

```go
// ListTenantContactsWithLeaseStatus returns all tenant contacts for the owner,
// each enriched with the active lease (if any) and the most recent terminal lease.
func (s *TenantContactService) ListTenantContactsWithLeaseStatus(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContactWithLeases, error) {
	contacts, err := s.repo.ListWithLeaseStatus(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list tenant contacts with lease status: %w", err)
	}
	return contacts, nil
}
```

**Step 2: Build**

Run:
```bash
cd apps/backend && go build ./...
```

Expected: no errors.

**Step 3: Stage**

```bash
git add apps/backend/internal/leases/application/tenant_contact_service.go
```

---

## Task 5: Update OpenAPI spec

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`

**Step 1: Modify `TenantContactResponse`**

Replace the existing `TenantContactResponse` block (lines ~2098-2139) with:

```yaml
    TenantContactResponse:
      type: object
      required: [id, owner_id, name, surname, patronymic, phone, email, comment, is_active, created_at, updated_at]
      properties:
        id:
          type: string
          format: uuid
        owner_id:
          type: string
          format: uuid
        name:
          type: string
        surname:
          type: string
          nullable: true
        patronymic:
          type: string
          nullable: true
        phone:
          type: string
          nullable: true
        email:
          type: string
          nullable: true
        comment:
          type: string
          nullable: true
        is_active:
          type: boolean
        active_lease:
          $ref: '#/components/schemas/LeaseResponse'
          nullable: true
        last_lease:
          $ref: '#/components/schemas/LeaseResponse'
          nullable: true
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
```

**Step 2: Validate OpenAPI**

Run:
```bash
cd apps/backend
npx @apidevtools/swagger-cli validate api/openapi/openapi.yaml
```

Expected: `api/openapi/openapi.yaml is valid`.

**Step 3: Stage**

```bash
git add apps/backend/api/openapi/openapi.yaml
```

---

## Task 6: Update HTTP handler and regenerate generated code

**Files:**
- Modify: `apps/backend/internal/platform/httpapi/lease_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/lease_presenter.go`
- Modify: `apps/backend/internal/platform/openapi/generated.gen.go` (regenerated)

**Step 1: Regenerate OpenAPI Go code**

Run:
```bash
cd apps/backend
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml
```

Expected: `generated.gen.go` contains `IsActive`, `ActiveLease`, `LastLease` fields in `TenantContactResponse`.

**Step 2: Update `ListTenantContacts` handler**

Find `ListTenantContacts` in `lease_handlers.go` and replace the body with:

```go
func (h *LeaseHandlers) ListTenantContacts(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	contacts, err := h.tenantContactSvc.ListTenantContactsWithLeaseStatus(r.Context(), ownerID)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	items := make([]openapi.TenantContactResponse, 0, len(contacts))
	for _, contact := range contacts {
		resp := tenantContactResponse(contact.TenantContact)
		resp.IsActive = contact.ActiveLease != nil
		if contact.ActiveLease != nil {
			leaseResp, err := h.presenter.leaseResponse(r.Context(), ownerID, *contact.ActiveLease, nil)
			if err != nil {
				h.handleLeaseError(w, r, err)
				return
			}
			resp.ActiveLease = &leaseResp
		}
		if contact.LastLease != nil {
			leaseResp, err := h.presenter.leaseResponse(r.Context(), ownerID, *contact.LastLease, nil)
			if err != nil {
				h.handleLeaseError(w, r, err)
				return
			}
			resp.LastLease = &leaseResp
		}
		items = append(items, resp)
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.TenantContactsResponse{Items: items})
}
```

**Step 3: Build backend**

Run:
```bash
cd apps/backend && go build ./...
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/backend/internal/platform/httpapi/lease_handlers.go apps/backend/internal/platform/httpapi/lease_presenter.go apps/backend/internal/platform/openapi/generated.gen.go
```

---

## Task 7: Regenerate frontend API types

**Files:**
- Modify: `apps/frontend/shared/api/generated.ts` (regenerated)

**Step 1: Regenerate TypeScript client**

Run:
```bash
cd apps/frontend && npm run generate:api
```

Expected: `shared/api/generated.ts` updated; `TenantContactResponse` now has `is_active`, `active_lease`, `last_lease`.

**Step 2: Type-check**

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

## Task 8: Create tenant-contact entity

**Files:**
- Create: `apps/frontend/entities/tenant-contact/model/types.ts`
- Create: `apps/frontend/entities/tenant-contact/model/mappers.ts`

**Step 1: Create directory**

```bash
mkdir -p apps/frontend/entities/tenant-contact/model
```

**Step 2: Write types**

`apps/frontend/entities/tenant-contact/model/types.ts`:

```ts
import type { Lease } from '@/entities/lease/model/types';

export type TenantContact = {
  readonly id: string;
  readonly ownerId: string;
  readonly name: string;
  readonly surname: string | null;
  readonly patronymic: string | null;
  readonly phone: string | null;
  readonly email: string | null;
  readonly comment: string | null;
  readonly isActive: boolean;
  readonly activeLease?: Lease;
  readonly lastLease?: Lease;
  readonly createdAt: string;
  readonly updatedAt: string;
};
```

**Step 3: Write mapper**

`apps/frontend/entities/tenant-contact/model/mappers.ts`:

```ts
import type { components } from '@/shared/api/generated';
import { mapLeaseResponse } from '@/entities/lease/model/mappers';
import type { TenantContact } from './types';

type TenantContactResponse = components['schemas']['TenantContactResponse'];

export function mapTenantContactResponse(dto: TenantContactResponse): TenantContact {
  return {
    id: dto.id,
    ownerId: dto.owner_id,
    name: dto.name,
    surname: dto.surname,
    patronymic: dto.patronymic,
    phone: dto.phone,
    email: dto.email,
    comment: dto.comment,
    isActive: dto.is_active,
    activeLease: dto.active_lease ? mapLeaseResponse(dto.active_lease) : undefined,
    lastLease: dto.last_lease ? mapLeaseResponse(dto.last_lease) : undefined,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}
```

**Step 4: Verify types**

Run:
```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

**Step 5: Stage**

```bash
git add apps/frontend/entities/tenant-contact/model/types.ts apps/frontend/entities/tenant-contact/model/mappers.ts
```

---

## Task 9: Update tenant-contacts API hook

**Files:**
- Modify: `apps/frontend/features/tenant-contacts/api/hooks.ts`
- Modify: `apps/frontend/features/tenant-contacts/api/index.ts` (if barrel exists)

**Step 1: Update `useTenantContacts`**

Replace the existing `useTenantContacts` in `hooks.ts` with:

```ts
import { mapTenantContactResponse } from '@/entities/tenant-contact/model/mappers';
import type { TenantContact } from '@/entities/tenant-contact/model/types';

export function useTenantContacts(): UseQueryResult<TenantContact[], ApiError> {
  return useQuery({
    queryKey: tenantContactKeys.all,
    queryFn: async () => {
      const response = await apiClient<TenantContactsResponse>('/tenant-contacts');
      return response.items.map(mapTenantContactResponse);
    },
  });
}
```

**Step 2: Type-check and lint**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 3: Stage**

```bash
git add apps/frontend/features/tenant-contacts/api/hooks.ts
```

---

## Task 10: Create subtitle helper

**Files:**
- Create: `apps/frontend/widgets/tenants/lib/get-tenant-subtitle.ts`

**Step 1: Create helper**

```ts
import type { TenantContact } from '@/entities/tenant-contact/model/types';

function pluralizeMonths(n: number): string {
  const last = n % 10;
  const lastTwo = n % 100;
  if (last === 1 && lastTwo !== 11) return `${n} месяц`;
  if ([2, 3, 4].includes(last) && ![12, 13, 14].includes(lastTwo)) return `${n} месяца`;
  return `${n} месяцев`;
}

function formatAgo(date: string): string {
  const diffMs = Date.now() - new Date(date).getTime();
  const days = Math.max(0, Math.floor(diffMs / (1000 * 60 * 60 * 24)));
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  if (years > 0) {
    const remMonths = Math.floor((days % 365) / 30);
    if (remMonths === 0) return `${years} ${pluralizeYears(years)} назад`;
    return `${years} ${pluralizeYears(years)}, ${remMonths} ${pluralizeMonths(remMonths)} назад`;
  }
  if (months > 0) return `${months} ${pluralizeMonths(months)} назад`;
  return `${days} ${pluralizeDays(days)} назад`;
}

function pluralizeYears(n: number): string {
  const last = n % 10;
  const lastTwo = n % 100;
  if (last === 1 && lastTwo !== 11) return 'год';
  if ([2, 3, 4].includes(last) && ![12, 13, 14].includes(lastTwo)) return 'года';
  return 'лет';
}

function pluralizeDays(n: number): string {
  const last = n % 10;
  const lastTwo = n % 100;
  if (last === 1 && lastTwo !== 11) return 'день';
  if ([2, 3, 4].includes(last) && ![12, 13, 14].includes(lastTwo)) return 'дня';
  return 'дней';
}

export function getTenantSubtitle(contact: TenantContact): string | undefined {
  if (contact.activeLease) {
    const months = Math.floor(
      (Date.now() - new Date(contact.activeLease.startDate).getTime()) /
        (1000 * 60 * 60 * 24 * 30),
    );
    const n = Math.max(1, months);
    return `${n}-й ${n === 1 ? 'месяц' : 'месяца'} аренды`;
  }

  if (contact.lastLease?.endDate) {
    return `Аренда завершена ${formatAgo(contact.lastLease.endDate)}`;
  }

  return undefined;
}
```

**Step 2: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

**Step 3: Stage**

```bash
git add apps/frontend/widgets/tenants/lib/get-tenant-subtitle.ts
```

---

## Task 11: Create TenantCard component

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantCard.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantCard.module.css`

**Step 1: Implement component**

```tsx
'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { getTenantContactFullName } from '@/widgets/tenant-detail/lib/get-tenant-contact-full-name';
import { getTenantSubtitle } from '../lib/get-tenant-subtitle';
import type { TenantContact } from '@/entities/tenant-contact/model/types';
import styles from './TenantCard.module.css';

export type TenantCardProps = {
  readonly tenant: TenantContact;
};

export function TenantCard({ tenant }: TenantCardProps): JSX.Element {
  const subtitle = getTenantSubtitle(tenant);

  return (
    <NextLink href={ROUTES.tenant(tenant.id)} className={styles.card}>
      <span className={styles.name}>{getTenantContactFullName(tenant)}</span>
      {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
    </NextLink>
  );
}
```

**Step 2: Add styles**

```css
.card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 20px;
  border-radius: 24px;
  background: var(--color-secondary-bg);
  color: var(--color-text);
  text-decoration: none;
}

.name {
  font-size: var(--font-size-text-l);
  line-height: var(--line-height-text-l);
  font-weight: 500;
  color: var(--color-text);
}

.subtitle {
  font-size: var(--font-size-text-s);
  line-height: var(--line-height-text-s);
  color: var(--color-text-muted);
}
```

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenants/ui/TenantCard.tsx apps/frontend/widgets/tenants/ui/TenantCard.module.css
```

---

## Task 12: Create TenantSection component

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantSection.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantSection.module.css`

**Step 1: Implement component**

```tsx
'use client';

import type { JSX } from 'react';
import { TenantCard } from './TenantCard';
import type { TenantContact } from '@/entities/tenant-contact/model/types';
import styles from './TenantSection.module.css';

export type TenantSectionProps = {
  readonly title: string;
  readonly tenants: TenantContact[];
};

export function TenantSection({ title, tenants }: TenantSectionProps): JSX.Element {
  if (tenants.length === 0) {
    return <></>;
  }

  return (
    <section className={styles.section}>
      <h2 className={styles.title}>{title}</h2>
      <div className={styles.list}>
        {tenants.map((tenant) => (
          <TenantCard key={tenant.id} tenant={tenant} />
        ))}
      </div>
    </section>
  );
}
```

**Step 2: Add styles**

```css
.section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.title {
  margin: 0;
  font-size: var(--font-size-text-l);
  font-weight: 600;
  line-height: var(--line-height-text-l);
  color: var(--color-text);
}

.list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
```

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenants/ui/TenantSection.tsx apps/frontend/widgets/tenants/ui/TenantSection.module.css
```

---

## Task 13: Create loading, error and empty states

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantsLoading.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantsLoading.module.css`
- Create: `apps/frontend/widgets/tenants/ui/TenantsError.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantsError.module.css`
- Create: `apps/frontend/widgets/tenants/ui/TenantsEmptyState.tsx`

**Step 1: TenantsLoading**

```tsx
'use client';

import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './TenantsLoading.module.css';

export function TenantsLoading(): JSX.Element {
  return (
    <div className={styles.root} aria-busy="true" aria-label="Загрузка арендаторов" role="status">
      <Skeleton className={styles.sectionTitle} />
      <Skeleton className={styles.card} />
      <Skeleton className={styles.card} />
      <Skeleton className={styles.sectionTitle} />
      <Skeleton className={styles.card} />
    </div>
  );
}
```

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sectionTitle {
  height: 24px;
  width: 160px;
  border-radius: 8px;
}

.card {
  height: 72px;
  border-radius: 24px;
}
```

**Step 2: TenantsError**

```tsx
'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import styles from './TenantsError.module.css';

export type TenantsErrorProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

export function TenantsError({ onRetry, isLoading }: TenantsErrorProps): JSX.Element {
  return (
    <div className={styles.root} role="alert" aria-live="polite">
      <p className={styles.text}>Не удалось загрузить арендаторов</p>
      <Button type="button" variant="primary" size="medium" loading={isLoading} onClick={onRetry}>
        Повторить
      </Button>
    </div>
  );
}
```

```css
.root {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;
  padding: 48px 20px;
  text-align: center;
}

.text {
  margin: 0;
  font-size: var(--font-size-text-m);
  line-height: var(--line-height-text-m);
  color: var(--color-text-muted);
}
```

**Step 3: TenantsEmptyState**

```tsx
'use client';

import type { JSX } from 'react';
import { Icon } from '@/shared/ui/icon';
import { EmptyState } from '@/shared/ui/empty-state';
import { Arendators } from '@/shared/assets/icons';

export function TenantsEmptyState(): JSX.Element {
  return (
    <EmptyState
      icon={
        <Icon size="l">
          <Arendators />
        </Icon>
      }
      entities="арендаторов"
      subtitle="Добавьте первого арендатора"
      actionHref="/tenants/new"
      actionText="Добавить арендатора"
    />
  );
}
```

**Step 4: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 5: Stage**

```bash
git add apps/frontend/widgets/tenants/ui/TenantsLoading.tsx apps/frontend/widgets/tenants/ui/TenantsLoading.module.css apps/frontend/widgets/tenants/ui/TenantsError.tsx apps/frontend/widgets/tenants/ui/TenantsError.module.css apps/frontend/widgets/tenants/ui/TenantsEmptyState.tsx
```

---

## Task 14: Create TenantsPage widget

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantsPage.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantsPage.module.css`

**Step 1: Implement component**

```tsx
'use client';

import { useMemo, type JSX } from 'react';
import { useTenantContacts } from '@/features/tenant-contacts/api';
import { Button } from '@/shared/ui/button';
import { IconLink } from '@/shared/ui/icon-link';
import { ROUTES } from '@/shared/config/routes';
import { ArrowLeft } from '@/shared/assets/icons';
import { TenantSection } from './TenantSection';
import { TenantsLoading } from './TenantsLoading';
import { TenantsError } from './TenantsError';
import { TenantsEmptyState } from './TenantsEmptyState';
import styles from './TenantsPage.module.css';

export function TenantsPage(): JSX.Element {
  const query = useTenantContacts();
  const tenants = query.data ?? [];

  const { active, past } = useMemo(() => {
    const activeContacts = tenants.filter((t) => t.isActive);
    const pastContacts = tenants.filter((t) => !t.isActive);
    return { active: activeContacts, past: pastContacts };
  }, [tenants]);

  if (query.isPending) {
    return <TenantsLoading />;
  }

  if (query.isError) {
    return <TenantsError onRetry={() => query.refetch()} isLoading={query.isFetching} />;
  }

  if (tenants.length === 0) {
    return <TenantsEmptyState />;
  }

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <IconLink href={ROUTES.dashboard} aria-label="Назад" icon={<ArrowLeft />} />
        <h1 className={styles.title}>Арендаторы</h1>
        <Button
          type="button"
          variant="secondary"
          size="small"
          onClick={() => {
            // navigation handled by router in page or link wrapper
          }}
        >
          Добавить
        </Button>
      </header>

      <TenantSection title="Текущие арендаторы" tenants={active} />
      <TenantSection title="Прошлые арендаторы" tenants={past} />
    </div>
  );
}
```

Wait — the "Добавить" button should navigate to `/tenants/new`. Use `LinkButton` instead of `Button`:

```tsx
import { LinkButton } from '@/shared/ui/link-button';

<LinkButton href={ROUTES.tenantsNew} variant="secondary" size="small">
  Добавить
</LinkButton>
```

But `ROUTES.tenantsNew` does not exist yet. Add it in Task 15 or hardcode `/tenants/new`.

**Step 2: Add styles**

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 48px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 0;
}

.title {
  flex: 1;
  margin: 0;
  font-size: 20px;
  font-weight: 500;
  line-height: 22px;
  color: var(--color-text);
  text-align: center;
}
```

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenants/ui/TenantsPage.tsx apps/frontend/widgets/tenants/ui/TenantsPage.module.css
```

---

## Task 15: Add tenantsNew route constant

**Files:**
- Modify: `apps/frontend/shared/config/routes.ts`

**Step 1: Add `tenantsNew`**

```ts
export const ROUTES = {
  // ... existing routes
  tenants: '/tenants',
  tenantsNew: '/tenants/new',
  tenant: (id: string) => `/tenants/${id}`,
  // ...
} as const;
```

**Step 2: Update TenantsPage to use the constant**

Replace the hardcoded `/tenants/new` (and `ROUTES.tenantsNew` if already used) in `TenantsPage.tsx`.

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/shared/config/routes.ts apps/frontend/widgets/tenants/ui/TenantsPage.tsx
```

---

## Task 16: Update the route page

**Files:**
- Modify: `apps/frontend/app/(cabinet)/tenants/page.tsx`
- Create: `apps/frontend/app/(cabinet)/tenants/page.module.css`

**Step 1: Rewrite the page**

```tsx
import type { Metadata } from 'next';
import { TenantsPage } from '@/widgets/tenants';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Арендаторы — Arenda Platform',
  description: 'Список арендаторов',
};

export default function TenantsListPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <TenantsPage />
      </div>
    </div>
  );
}
```

**Step 2: Add styles**

```css
.root {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 84px 20px 96px;
}

.content {
  width: 100%;
  max-width: 560px;
}

@media (min-width: 768px) {
  .root {
    padding: 96px 0;
  }
}
```

**Step 3: Update widget barrel**

Modify `apps/frontend/widgets/tenants/ui/index.ts` to export:

```ts
export { TenantCreateWizard } from './TenantCreateWizard';
export { TenantsPage } from './TenantsPage';
```

**Step 4: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 5: Stage**

```bash
git add apps/frontend/app/(cabinet)/tenants/page.tsx apps/frontend/app/(cabinet)/tenants/page.module.css apps/frontend/widgets/tenants/ui/index.ts
```

---

## Task 17: Final verification

**Files:**
- None new.

**Step 1: Type-check**

Run:
```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

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

Expected: build succeeds.

**Step 4: Backend build**

Run:
```bash
cd apps/backend && go build ./...
```

Expected: no errors.

**Step 5: Manual check**

1. Start backend (`make backend-run`) and frontend (`cd apps/frontend && npm run dev`).
2. Create or use existing tenant contacts with active and completed leases.
3. Open `http://localhost:3000/tenants`.
4. Verify active tenants appear under «Текущие арендаторы» with a subtitle.
5. Verify inactive tenants appear under «Прошлые арендаторы».
6. Verify clicking a card navigates to `/tenants/[id]`.
7. Verify empty state when no contacts exist.

**Step 6: Stage final adjustments**

```bash
git add .
```

---

## Execution handoff

**Plan complete and saved to `docs/plans/2026-06-26-tenants-list-implementation-plan.md`.**

Two execution options:

1. **Subagent-Driven (this session)** — dispatch fresh subagent per task, review between tasks, fast iteration. Use `@superpowers:subagent-driven-development`.
2. **Parallel Session (separate)** — open a new session with `@superpowers:executing-plans` and batch execution with checkpoints.

Which approach do you prefer?

# Страница просмотра арендатора — план реализации

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Реализовать страницу просмотра арендатора по маршруту `/tenants/[id]` с ФИО, контактами, текущей арендой и комментарием в единой дизайн-системе кабинета.

**Architecture:** FSD-виджет `widgets/tenant-detail` составляет страницу из независимых секций (`Header`, `Info`, `Lease`, `Comment`) и переиспользует `PropertyDetailSection`, `IconLink`, `Button`, `Skeleton`. Данные получаем через существующие хуки `useTenantContact`, `useLeases` и `useProperty`.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, TanStack Query, CSS Modules, Tailwind v4, Hero UI v3.

---

## Перед началом

- [ ] Убедиться, что frontend зависимости установлены: `cd apps/frontend && npm install`.
- [ ] **Важно:** не выполнять `git commit` без явного подтверждения пользователя (политика репозитория). Шаги с коммитами в плане — подготовить индекс и сообщение, затем спросить.

---

## Task 1: Добавить маршрут арендатора в конфиг

**Files:**
- Modify: `apps/frontend/shared/config/routes.ts`

**Step 1: Добавить helper `tenant(id)`**

```ts
export const ROUTES = {
  dashboard: '/dashboard',
  properties: '/properties',
  property: (id: string) => `/properties/${id}`,
  propertyArchive: '/properties/archive',
  propertyNew: '/properties/new',
  propertyEdit: (id: string) => `/properties/${id}/edit`,
  leaseNew: '/leases/new',
  tenants: '/tenants',
  tenant: (id: string) => `/tenants/${id}`,
  finance: '/finance',
  profile: '/profile',
  support: '/support',
} as const;
```

**Step 2: Проверить типы**

Run:
```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

**Step 3: Stage**

```bash
git add apps/frontend/shared/config/routes.ts
```

---

## Task 2: Создать хелпер для статуса аренды

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/lib/lease-status-label.ts`

**Step 1: Создать директорию**

```bash
mkdir -p apps/frontend/widgets/tenant-detail/lib
```

**Step 2: Написать хелпер**

```ts
import type { components } from '@/shared/api/generated';

type LeaseStatus = components['schemas']['LeaseResponse']['status'];

const labels: Record<LeaseStatus, string> = {
  awaiting_start: 'Скоро начнётся',
  active: 'Активна',
  requires_action: 'Требует действия',
  completed: 'Завершена',
  archived: 'В архиве',
};

export function getLeaseStatusLabel(status: LeaseStatus): string {
  return labels[status];
}
```

**Step 3: Проверить типы**

Run:
```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/lib/lease-status-label.ts
```

---

## Task 3: Создать заголовок страницы

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantDetailHeader.tsx`
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantDetailHeader.module.css`

**Step 1: Создать директорию**

```bash
mkdir -p apps/frontend/widgets/tenant-detail/ui
```

**Step 2: Написать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { IconLink } from '@/shared/ui/icon-link';
import { Button } from '@/shared/ui/button';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import styles from './TenantDetailHeader.module.css';

export type TenantDetailHeaderProps = {
  readonly title: string;
};

export function TenantDetailHeader({ title }: TenantDetailHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <IconLink href={ROUTES.tenants} aria-label="Назад" icon={<ArrowLeft />} />
      <h1 className={styles.title}>{title}</h1>
      <Button
        type="button"
        variant="secondary"
        size="small"
        onClick={() => {
          // TODO: navigate to tenant edit page when implemented
        }}
      >
        Редактировать
      </Button>
    </header>
  );
}
```

**Step 3: Написать стили**

```css
.root {
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
  color: #1e1e1e;
  text-align: center;
}
```

**Step 4: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 5: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/ui/TenantDetailHeader.tsx apps/frontend/widgets/tenant-detail/ui/TenantDetailHeader.module.css
```

---

## Task 4: Создать секцию контактов

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantInfoSection.tsx`
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantInfoSection.module.css`

**Step 1: Написать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { Icon } from '@/shared/ui/icon';
import { UserSmall } from '@/shared/assets/icons';
import { PropertyDetailSection } from '@/widgets/property-detail/ui/PropertyDetailSection';
import type { components } from '@/shared/api/generated';
import styles from './TenantInfoSection.module.css';

type TenantContactResponse = components['schemas']['TenantContactResponse'];

export type TenantInfoSectionProps = {
  readonly tenant: TenantContactResponse;
};

export function TenantInfoSection({ tenant }: TenantInfoSectionProps): JSX.Element {
  const fullName = [tenant.surname, tenant.name, tenant.patronymic]
    .filter(Boolean)
    .join(' ');

  return (
    <PropertyDetailSection>
      <h2 className={styles.title}>Контакты</h2>
      <div className={styles.card}>
        <div className={styles.profile}>
          <span className={styles.avatar}>
            <Icon size="m">
              <UserSmall />
            </Icon>
          </span>
          <span className={styles.name}>{fullName}</span>
        </div>
        {tenant.phone && <p className={styles.field}>{tenant.phone}</p>}
        {tenant.email && <p className={styles.field}>{tenant.email}</p>}
      </div>
    </PropertyDetailSection>
  );
}
```

**Step 2: Написать стили**

```css
.title {
  margin: 0;
  font-size: var(--font-size-text-l);
  font-weight: 600;
  line-height: var(--line-height-text-l);
  color: var(--color-text);
}

.card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 20px;
  border-radius: 24px;
  background: var(--color-secondary-bg);
}

.profile {
  display: flex;
  align-items: center;
  gap: 12px;
}

.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--color-white);
  color: var(--color-text);
}

.name {
  font-size: var(--font-size-text-m);
  line-height: var(--line-height-text-m);
  font-weight: 500;
  color: var(--color-text);
}

.field {
  margin: 0;
  font-size: var(--font-size-text-m);
  line-height: var(--line-height-text-m);
  color: var(--color-text-muted);
}
```

**Step 3: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/ui/TenantInfoSection.tsx apps/frontend/widgets/tenant-detail/ui/TenantInfoSection.module.css
```

---

## Task 5: Создать секцию текущей аренды

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantLeaseSection.tsx`
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantLeaseSection.module.css`

**Step 1: Написать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Icon } from '@/shared/ui/icon';
import { ArrowRight } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatDuration } from '@/shared/lib/format-duration';
import { useProperty } from '@/features/properties/api';
import { PropertyDetailSection } from '@/widgets/property-detail/ui/PropertyDetailSection';
import { getLeaseStatusLabel } from '../lib/lease-status-label';
import type { components } from '@/shared/api/generated';
import styles from './TenantLeaseSection.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];

export type TenantLeaseSectionProps = {
  readonly lease: LeaseResponse;
};

export function TenantLeaseSection({ lease }: TenantLeaseSectionProps): JSX.Element {
  const propertyQuery = useProperty(lease.property_id);
  const property = propertyQuery.data;
  const endDate = lease.end_date ?? lease.start_date;
  const propertyHref = ROUTES.property(lease.property_id);

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>Текущая аренда</h2>
        <NextLink href={propertyHref} className={styles.headerLink} aria-label="Перейти к объекту">
          <Icon size="s">
            <ArrowRight />
          </Icon>
        </NextLink>
      </div>

      <NextLink href={propertyHref} className={styles.card}>
        <div className={styles.row}>
          <span className={styles.status}>{getLeaseStatusLabel(lease.status)}</span>
          <span className={styles.amount}>{formatMoneyKopecks(lease.rent_amount_kopecks)}</span>
        </div>

        <div className={styles.row}>
          <span className={styles.address}>
            {property?.address ?? 'Загрузка адреса…'}
          </span>
          <span className={styles.duration}>
            {formatDuration(lease.start_date, endDate)}
          </span>
        </div>
      </NextLink>
    </PropertyDetailSection>
  );
}
```

**Step 2: Написать стили**

```css
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.title {
  margin: 0;
  font-size: var(--font-size-text-l);
  font-weight: 600;
  line-height: var(--line-height-text-l);
  color: var(--color-text);
}

.headerLink {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text);
}

.card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 20px;
  border-radius: 24px;
  background: var(--color-secondary-bg);
  color: var(--color-text);
  text-decoration: none;
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.status {
  font-size: var(--font-size-text-s);
  line-height: var(--line-height-text-s);
  color: var(--color-text-muted);
}

.amount {
  font-size: var(--font-size-text-m);
  line-height: var(--line-height-text-m);
  font-weight: 600;
  color: var(--color-text);
}

.address {
  font-size: var(--font-size-text-m);
  line-height: var(--line-height-text-m);
  color: var(--color-text);
}

.duration {
  font-size: var(--font-size-text-s);
  line-height: var(--line-height-text-s);
  color: var(--color-text-muted);
  white-space: nowrap;
}
```

**Step 3: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/ui/TenantLeaseSection.tsx apps/frontend/widgets/tenant-detail/ui/TenantLeaseSection.module.css
```

---

## Task 6: Создать секцию комментария

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantCommentSection.tsx`
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantCommentSection.module.css`

**Step 1: Написать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { PropertyDetailSection } from '@/widgets/property-detail/ui/PropertyDetailSection';
import styles from './TenantCommentSection.module.css';

export type TenantCommentSectionProps = {
  readonly comment: string;
};

export function TenantCommentSection({ comment }: TenantCommentSectionProps): JSX.Element {
  return (
    <PropertyDetailSection>
      <h2 className={styles.title}>Комментарий</h2>
      <p className={styles.text}>{comment}</p>
    </PropertyDetailSection>
  );
}
```

**Step 2: Написать стили**

```css
.title {
  margin: 0;
  font-size: var(--font-size-text-l);
  font-weight: 600;
  line-height: var(--line-height-text-l);
  color: var(--color-text);
}

.text {
  margin: 0;
  padding: 20px;
  border-radius: 24px;
  background: var(--color-secondary-bg);
  font-size: var(--font-size-text-m);
  line-height: var(--line-height-text-m);
  color: var(--color-text);
  white-space: pre-wrap;
}
```

**Step 3: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/ui/TenantCommentSection.tsx apps/frontend/widgets/tenant-detail/ui/TenantCommentSection.module.css
```

---

## Task 7: Создать состояние загрузки

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantDetailLoading.tsx`

**Step 1: Написать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { Skeleton } from '@heroui/react';
import styles from './TenantDetailLoading.module.css';

export function TenantDetailLoading(): JSX.Element {
  return (
    <div className={styles.root}>
      <Skeleton className={styles.header} />
      <Skeleton className={styles.section} />
      <Skeleton className={styles.section} />
    </div>
  );
}
```

**Step 2: Написать стили**

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 32px;
}

.header {
  height: 56px;
  border-radius: 16px;
}

.section {
  height: 140px;
  border-radius: 24px;
}
```

**Step 3: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/ui/TenantDetailLoading.tsx apps/frontend/widgets/tenant-detail/ui/TenantDetailLoading.module.css
```

---

## Task 8: Создать состояние ошибки

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantDetailError.tsx`
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantDetailError.module.css`

**Step 1: Написать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import styles from './TenantDetailError.module.css';

export type TenantDetailErrorProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

export function TenantDetailError({ onRetry, isLoading }: TenantDetailErrorProps): JSX.Element {
  return (
    <div className={styles.root}>
      <p className={styles.text}>Не удалось загрузить данные арендатора</p>
      <Button
        type="button"
        variant="primary"
        size="medium"
        loading={isLoading}
        onClick={onRetry}
      >
        Повторить
      </Button>
    </div>
  );
}
```

**Step 2: Написать стили**

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

**Step 3: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/ui/TenantDetailError.tsx apps/frontend/widgets/tenant-detail/ui/TenantDetailError.module.css
```

---

## Task 9: Создать страничный виджет

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantDetailPage.tsx`
- Create: `apps/frontend/widgets/tenant-detail/ui/TenantDetailPage.module.css`

**Step 1: Написать компонент**

```tsx
'use client';

import { useParams } from 'next/navigation';
import { useMemo, type JSX } from 'react';
import { useTenantContact } from '@/features/tenant-contacts/api';
import { useLeases } from '@/features/leases/api';
import { useProperty } from '@/features/properties/api';
import { TenantDetailHeader } from './TenantDetailHeader';
import { TenantInfoSection } from './TenantInfoSection';
import { TenantLeaseSection } from './TenantLeaseSection';
import { TenantCommentSection } from './TenantCommentSection';
import { TenantDetailLoading } from './TenantDetailLoading';
import { TenantDetailError } from './TenantDetailError';
import styles from './TenantDetailPage.module.css';

function findCurrentLeaseByTenant(
  leases: components['schemas']['LeaseResponse'][],
  tenantId: string,
): components['schemas']['LeaseResponse'] | undefined {
  return leases.find(
    (lease) =>
      lease.tenant_contact?.id === tenantId &&
      lease.status !== 'completed' &&
      lease.status !== 'archived',
  );
}

export function TenantDetailPage(): JSX.Element {
  const params = useParams<{ id: string }>();
  const id = params.id ?? '';

  const tenantQuery = useTenantContact(id);
  const leasesQuery = useLeases();

  const currentLease = useMemo(() => {
    const leases = leasesQuery.data ?? [];
    return findCurrentLeaseByTenant(leases, id);
  }, [leasesQuery.data, id]);

  useProperty(currentLease?.property_id ?? '', {
    enabled: Boolean(currentLease?.property_id),
  });

  if (tenantQuery.isPending || leasesQuery.isPending) {
    return <TenantDetailLoading />;
  }

  if (tenantQuery.isError || leasesQuery.isError || !tenantQuery.data) {
    return (
      <TenantDetailError
        onRetry={() => {
          tenantQuery.refetch();
          leasesQuery.refetch();
        }}
        isLoading={tenantQuery.isFetching || leasesQuery.isFetching}
      />
    );
  }

  const tenant = tenantQuery.data;
  const fullName = [tenant.surname, tenant.name, tenant.patronymic]
    .filter(Boolean)
    .join(' ');

  return (
    <div className={styles.root}>
      <TenantDetailHeader title={fullName || tenant.name} />
      <TenantInfoSection tenant={tenant} />
      {currentLease && <TenantLeaseSection lease={currentLease} />}
      {tenant.comment && <TenantCommentSection comment={tenant.comment} />}
    </div>
  );
}
```

Wait — `useProperty` signature in `features/properties/api/hooks.ts` does not accept a second `options` object. Check the hook before writing this task. If it only takes `id: string`, wrap the call in a condition or accept the extra query even when id is empty. Adjust the code to match the actual hook signature.

**Step 2: Написать стили**

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 32px;
}
```

**Step 3: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/ui/TenantDetailPage.tsx apps/frontend/widgets/tenant-detail/ui/TenantDetailPage.module.css
```

---

## Task 10: Создать barrel-экспорты виджета

**Files:**
- Create: `apps/frontend/widgets/tenant-detail/ui/index.ts`
- Create: `apps/frontend/widgets/tenant-detail/index.ts`

**Step 1: Написать `ui/index.ts`**

```ts
export { TenantDetailPage } from './TenantDetailPage';
```

**Step 2: Написать корневой `index.ts`**

```ts
export { TenantDetailPage } from './ui';
```

**Step 3: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Stage**

```bash
git add apps/frontend/widgets/tenant-detail/ui/index.ts apps/frontend/widgets/tenant-detail/index.ts
```

---

## Task 11: Создать маршрут `/tenants/[id]`

**Files:**
- Create: `apps/frontend/app/(cabinet)/tenants/[id]/page.tsx`
- Create: `apps/frontend/app/(cabinet)/tenants/[id]/page.module.css`

**Step 1: Создать директорию**

```bash
mkdir -p apps/frontend/app/(cabinet)/tenants/[id]
```

**Step 2: Написать страницу**

```tsx
import type { Metadata } from 'next';
import { Suspense } from 'react';
import { TenantDetailPage, TenantDetailLoading } from '@/widgets/tenant-detail';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Арендатор — Arenda Platform',
  description: 'Просмотр данных арендатора',
};

export default async function TenantPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <Suspense fallback={<TenantDetailLoading />}>
          <TenantDetailPage />
        </Suspense>
      </div>
    </div>
  );
}
```

**Step 3: Написать стили**

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

**Step 4: Проверить типы и линтер**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 5: Stage**

```bash
git add apps/frontend/app/(cabinet)/tenants/[id]/page.tsx apps/frontend/app/(cabinet)/tenants/[id]/page.module.css
```

---

## Task 12: Финальная верификация

**Files:**
- None new.

**Step 1: Проверить типы**

Run:
```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

**Step 2: Проверить линтер**

Run:
```bash
cd apps/frontend && npm run lint
```

Expected: no errors.

**Step 3: Собрать production build**

Run:
```bash
cd apps/frontend && npm run build
```

Expected: build succeeds.

**Step 4: Ручная проверка**

Run backend:
```bash
make backend-run
```

Run frontend:
```bash
cd apps/frontend && npm run dev
```

Открыть `http://localhost:3000/tenants/<id>` (id существующего арендатора) и проверить:
- Отображается ФИО в заголовке.
- Отображаются телефон и email, если заполнены.
- Блок «Текущая аренда» отображается только если есть незавершённая аренда с этим арендатором.
- В блоке аренды видны адрес, сумма, срок, статус.
- Клик по блоку аренды ведёт на `/properties/[id]`.
- Блок «Комментарий» отображается только если comment непустой.
- Кнопка «Редактировать» видна, но пока никуда не ведёт.

**Step 5: Stage финальные правки**

```bash
git add .
```

---

## Execution handoff

**Plan complete and saved to `docs/plans/2026-06-26-tenant-detail-implementation-plan.md`.**

Two execution options:

1. **Subagent-Driven (this session)** — dispatch fresh subagent per task, review between tasks, fast iteration. Use `@superpowers:subagent-driven-development`.
2. **Parallel Session (separate)** — open a new session with `@superpowers:executing-plans` and batch execution with checkpoints.

Which approach do you prefer?

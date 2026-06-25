# Страница создания аренды — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Реализовать `/leases/new` как двухшаговый wizard создания аренды 1‑в‑1 по макетам Figma, с использованием HeroUI v3 и существующих проектных UI‑компонентов.

**Architecture:** Frontend‑мастер на одном URL с `sessionStorage`; `propertyId` приходит из query‑параметра; шаг 1 — арендная плата и залог, шаг 2 — день оплаты и даты; после submit показывается экран успеха.

**Tech Stack:** Next.js 16 + React 19 + TypeScript + CSS Modules + `@heroui/react` v3 (DatePicker, Drawer) + TanStack Query.

---

### Task 1: Маршрут и временная точка входа с карточки объекта

**Files:**
- Modify: `apps/frontend/shared/config/routes.ts`
- Modify: `apps/frontend/widgets/properties/ui/PropertyCard.tsx`

**Step 1: Добавить маршрут**

```ts
export const ROUTES = {
  dashboard: '/dashboard',
  properties: '/properties',
  propertyArchive: '/properties/archive',
  propertyNew: '/properties/new',
  leaseNew: '/leases/new',
  tenants: '/tenants',
  finance: '/finance',
  profile: '/profile',
  support: '/support',
} as const;
```

**Step 2: Добавить кнопку «Сдать» для свободных объектов**

В `getPropertyAction` добавить ветку перед `if (!lease?.endDate) return null;`:

```ts
  if (displayStatus === 'not_rented') {
    return {
      kind: 'single',
      label: 'Сдать',
      href: `${ROUTES.leaseNew}?propertyId=${property.id}`,
      variant: 'primary',
    };
  }
```

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/shared/config/routes.ts apps/frontend/widgets/properties/ui/PropertyCard.tsx
git commit -m "feat: add lease new route and entry button on property card"
```

---

### Task 2: Хук черновика аренды

**Files:**
- Create: `apps/frontend/widgets/leases/lib/use-lease-create-draft.ts`

**Step 1: Написать хук**

```ts
'use client';

import { useState, useEffect } from 'react';

export type LeaseCreateStep = 1 | 2 | 3;

export type LeaseCreateDraft = {
  step: LeaseCreateStep;
  rentAmount?: string;
  depositAmount?: string;
  paymentDay?: number;
  startDate?: string;
  endDate?: string;
};

const STORAGE_KEY = 'lease-create-draft';

export function useLeaseCreateDraft() {
  const [draft, setDraft] = useState<LeaseCreateDraft>(() => loadDraft());

  useEffect(() => {
    if (draft.step === 3) {
      sessionStorage.removeItem(STORAGE_KEY);
      return;
    }
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(draft));
  }, [draft]);

  return { draft, setDraft };
}

function loadDraft(): LeaseCreateDraft {
  if (typeof window === 'undefined') return { step: 1 };
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return { step: 1 };
    const parsed = JSON.parse(raw) as LeaseCreateDraft;
    if (parsed.step < 1 || parsed.step > 3) return { step: 1 };
    return parsed;
  } catch {
    return { step: 1 };
  }
}
```

**Step 2: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 3: Commit**

```bash
git add apps/frontend/widgets/leases/lib/use-lease-create-draft.ts
git commit -m "feat: add lease create draft sessionStorage hook"
```

---

### Task 3: Шапка мастера

**Files:**
- Create: `apps/frontend/widgets/leases/ui/LeaseCreateHeader.tsx`
- Create: `apps/frontend/widgets/leases/ui/LeaseCreateHeader.module.css`

**Step 1: Создать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { IconButton } from '@/shared/ui/icon-button';
import styles from './LeaseCreateHeader.module.css';

export type LeaseCreateHeaderProps = {
  step: 1 | 2;
  onBack: () => void;
  onCancel: () => void;
};

export function LeaseCreateHeader({ step, onBack, onCancel }: LeaseCreateHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <div className={styles.topRow}>
        <IconButton
          variant="icon-black"
          size="small"
          icon={<ArrowLeft />}
          aria-label="Назад"
          onClick={onBack}
          className={styles.iconButton}
        />
        <h1 className={styles.title}>Создание аренды</h1>
        <IconButton
          variant="icon-black"
          size="small"
          icon={<Cancel />}
          aria-label="Отменить"
          onClick={onCancel}
          className={styles.iconButton}
        />
      </div>
      <div className={styles.progressRow}>
        <span className={styles.badge}>{step} из 2</span>
        <div className={styles.progressBar} role="progressbar" aria-label={`Шаг ${step} из 2`} aria-valuenow={step} aria-valuemin={1} aria-valuemax={2}>
          <div className={`${styles.segment} ${step >= 1 ? styles.active : ''}`} />
          <div className={`${styles.segment} ${step >= 2 ? styles.active : ''}`} />
        </div>
      </div>
    </header>
  );
}
```

**Step 2: Стили**

Скопировать `PropertyCreateHeader.module.css` в `LeaseCreateHeader.module.css` без изменений.

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/leases/ui/LeaseCreateHeader.tsx apps/frontend/widgets/leases/ui/LeaseCreateHeader.module.css
git commit -m "feat: lease create header"
```

---

### Task 4: Шаг 1 — Цена и залог

**Files:**
- Create: `apps/frontend/widgets/leases/ui/LeasePriceStep.tsx`
- Create: `apps/frontend/widgets/leases/ui/LeasePriceStep.module.css`

**Step 1: Создать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import styles from './LeasePriceStep.module.css';

export type LeasePriceStepProps = {
  rentAmount: string;
  depositAmount: string;
  onRentChange: (value: string) => void;
  onDepositChange: (value: string) => void;
  onNext: () => void;
};

export function LeasePriceStep({
  rentAmount,
  depositAmount,
  onRentChange,
  onDepositChange,
  onNext,
}: LeasePriceStepProps): JSX.Element {
  const isValid = rentAmount.trim() !== '' && Number(rentAmount) > 0;

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Цена и залог</h2>
      <div className={styles.fields}>
        <TextField
          label="Арендная плата"
          placeholder="Цена, ₽ в месяц"
          type="number"
          min={0}
          required
          value={rentAmount}
          onChange={(e) => onRentChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Залог"
          placeholder=" "
          type="number"
          min={0}
          value={depositAmount}
          onChange={(e) => onDepositChange(e.currentTarget.value)}
          fullWidth
        />
      </div>
      <Button
        type="button"
        variant="primary"
        size="large"
        fullWidth
        disabled={!isValid}
        onClick={onNext}
      >
        Продолжить
      </Button>
    </div>
  );
}
```

**Step 2: Стили**

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 52px;
}

.heading {
  margin: 0;
  font-size: 28px;
  font-weight: 500;
  line-height: 36px;
  color: var(--color-text);
}

.fields {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
```

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/leases/ui/LeasePriceStep.tsx apps/frontend/widgets/leases/ui/LeasePriceStep.module.css
git commit -m "feat: lease price step"
```

---

### Task 5: Пикер дня оплаты

**Files:**
- Create: `apps/frontend/widgets/leases/ui/PaymentDayPicker.tsx`
- Create: `apps/frontend/widgets/leases/ui/PaymentDayPicker.module.css`

**Step 1: Создать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { useState } from 'react';
import {
  Drawer,
  DrawerBackdrop,
  DrawerBody,
  DrawerCloseTrigger,
  DrawerContent,
  DrawerDialog,
  DrawerHeader,
  DrawerHeading,
} from '@heroui/react/drawer';
import { Button } from '@/shared/ui/button';
import styles from './PaymentDayPicker.module.css';

const DAYS = Array.from({ length: 31 }, (_, i) => i + 1);

export type PaymentDayPickerProps = {
  value?: number;
  onChange: (day: number) => void;
};

export function PaymentDayPicker({ value, onChange }: PaymentDayPickerProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(false);

  const handleSelect = (day: number) => {
    onChange(day);
    setIsOpen(false);
  };

  return (
    <div className={styles.root}>
      <span className={styles.label}>
        День оплаты <span className={styles.required}>*</span>
      </span>
      <Button
        type="button"
        variant="secondary"
        size="large"
        fullWidth
        onClick={() => setIsOpen(true)}
        className={styles.trigger}
      >
        {value ? `${value}-е число` : 'Выбрать'}
      </Button>
      <Drawer isOpen={isOpen} onOpenChange={(open) => setIsOpen(open)}>
        <DrawerBackdrop />
        <DrawerContent placement="bottom">
          <DrawerDialog>
            <DrawerCloseTrigger aria-label="Закрыть" />
            <DrawerHeader className={styles.drawerHeader}>
              <DrawerHeading>День оплаты</DrawerHeading>
            </DrawerHeader>
            <DrawerBody className={styles.drawerBody}>
              <div className={styles.grid}>
                {DAYS.map((day) => (
                  <button
                    key={day}
                    type="button"
                    className={`${styles.day} ${value === day ? styles.dayActive : ''}`}
                    onClick={() => handleSelect(day)}
                  >
                    {day}
                  </button>
                ))}
              </div>
            </DrawerBody>
          </DrawerDialog>
        </DrawerContent>
      </Drawer>
    </div>
  );
}
```

**Step 2: Стили**

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.label {
  font-size: 16px;
  font-weight: 400;
  line-height: 20px;
  color: var(--color-text);
}

.required {
  color: #ff4646;
}

.trigger {
  justify-content: space-between;
}

.drawerHeader {
  padding: 16px 20px;
}

.drawerBody {
  padding: 16px 20px 32px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 8px;
}

.day {
  display: flex;
  align-items: center;
  justify-content: center;
  aspect-ratio: 1;
  border: none;
  border-radius: 12px;
  background: #f1f3f6;
  font-size: 16px;
  font-weight: 500;
  color: var(--color-text);
  cursor: pointer;
}

.dayActive {
  background: var(--color-primary);
  color: var(--color-white);
}
```

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/leases/ui/PaymentDayPicker.tsx apps/frontend/widgets/leases/ui/PaymentDayPicker.module.css
git commit -m "feat: payment day picker"
```

---

### Task 6: Шаг 2 — Даты аренды

**Files:**
- Create: `apps/frontend/widgets/leases/ui/LeaseDatesStep.tsx`
- Create: `apps/frontend/widgets/leases/ui/LeaseDatesStep.module.css`

**Step 1: Создать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { DatePicker } from '@heroui/react/date-picker';
import { today, getLocalTimeZone, parseDate, type DateValue } from '@internationalized/date';
import { Button } from '@/shared/ui/button';
import { PaymentDayPicker } from './PaymentDayPicker';
import styles from './LeaseDatesStep.module.css';

export type LeaseDatesStepProps = {
  paymentDay?: number;
  startDate?: string;
  endDate?: string;
  onPaymentDayChange: (day: number) => void;
  onStartDateChange: (date: string) => void;
  onEndDateChange: (date: string) => void;
  onSubmit: () => void;
  isLoading: boolean;
};

export function LeaseDatesStep({
  paymentDay,
  startDate,
  endDate,
  onPaymentDayChange,
  onStartDateChange,
  onEndDateChange,
  onSubmit,
  isLoading,
}: LeaseDatesStepProps): JSX.Element {
  const minDate = today(getLocalTimeZone());

  const handleStartChange = (value: DateValue | null) => {
    if (!value) return;
    onStartDateChange(value.toString());
  };

  const handleEndChange = (value: DateValue | null) => {
    if (!value) {
      onEndDateChange('');
      return;
    }
    onEndDateChange(value.toString());
  };

  const isValid = Boolean(paymentDay && startDate);

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Даты аренды</h2>
      <div className={styles.fields}>
        <PaymentDayPicker value={paymentDay} onChange={onPaymentDayChange} />
        <div className={styles.dateRow}>
          <DatePicker
            label="Начало аренды"
            minValue={minDate}
            value={startDate ? parseDate(startDate) : null}
            onChange={handleStartChange}
            className={styles.dateField}
          />
          <DatePicker
            label="Конец аренды"
            minValue={startDate ? parseDate(startDate) : minDate}
            value={endDate ? parseDate(endDate) : null}
            onChange={handleEndChange}
            className={styles.dateField}
          />
        </div>
      </div>
      <Button
        type="button"
        variant="primary"
        size="large"
        fullWidth
        disabled={!isValid}
        loading={isLoading}
        onClick={onSubmit}
      >
        Создать аренду
      </Button>
    </div>
  );
}
```

**Step 2: Стили**

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 52px;
}

.heading {
  margin: 0;
  font-size: 28px;
  font-weight: 500;
  line-height: 36px;
  color: var(--color-text);
}

.fields {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.dateRow {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.dateField {
  width: 100%;
}
```

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/leases/ui/LeaseDatesStep.tsx apps/frontend/widgets/leases/ui/LeaseDatesStep.module.css
git commit -m "feat: lease dates step"
```

---

### Task 7: Экран успеха

**Files:**
- Create: `apps/frontend/widgets/leases/ui/LeaseSuccessStep.tsx`
- Create: `apps/frontend/widgets/leases/ui/LeaseSuccessStep.module.css`

**Step 1: Создать компонент**

```tsx
'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { ROUTES } from '@/shared/config/routes';
import { Users } from '@/shared/assets/icons';
import styles from './LeaseSuccessStep.module.css';

export type LeaseSuccessStepProps = {
  onAddLater: () => void;
  onAddTenant: () => void;
};

export function LeaseSuccessStep({ onAddLater, onAddTenant }: LeaseSuccessStepProps): JSX.Element {
  return (
    <div className={styles.root}>
      <div className={styles.card}>
        <div className={styles.iconWrapper}>
          <svg
            className={styles.illustration}
            viewBox="0 0 20 20"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              d="M13.786 5.12335C14.0872 4.82236 14.5756 4.82227 14.8768 5.12335C15.178 5.42454 15.1778 5.91293 14.8768 6.2142L14.011 7.08105C13.7097 7.3823 13.2203 7.38232 12.9191 7.08105C12.618 6.77994 12.6183 6.29148 12.9191 5.9902L13.786 5.12335Z"
              fill="currentColor"
            />
            <path
              fillRule="evenodd"
              clipRule="evenodd"
              d="M11.8292 1.03017C12.3432 0.969077 12.7662 0.998431 13.1843 1.13966C13.5682 1.26936 13.914 1.48615 14.251 1.69211C14.9371 2.11141 15.7108 2.69085 16.5101 3.4901C17.3092 4.28916 17.8888 5.06218 18.3081 5.74813C18.5141 6.08516 18.7298 6.43192 18.8596 6.81587C19.0008 7.234 19.0312 7.6568 18.97 8.17089C18.832 9.33136 18.1988 9.92961 17.389 10.7393L17.2474 10.8759C16.7054 11.3943 16.2591 11.8229 15.8582 12.1365C15.4959 12.42 15.1323 12.6407 14.7131 12.7683L14.5303 12.8175C14.4282 12.841 14.3247 12.8599 14.2209 12.8738C13.7139 12.9413 13.2395 12.8591 12.7383 12.701C12.3705 12.585 11.9562 12.4142 11.4767 12.2108L8.37987 15.3096C8.23522 15.4541 8.03893 15.5356 7.83444 15.5356H6.87417V16.4959C6.87417 16.9218 6.52863 17.2671 6.10273 17.2673H5.14145V18.2286C5.14145 18.6546 4.79606 19 4.37001 19H1.77144C1.34538 19 1 18.6546 1 18.2286V15.63C1.00005 15.4255 1.08139 15.2292 1.22601 15.0846L7.78723 8.52145C7.58419 8.0426 7.41394 7.62822 7.29805 7.26085C7.13998 6.75967 7.05865 6.2853 7.12629 5.77826C7.14012 5.67468 7.15915 5.57174 7.18254 5.46989C7.29708 4.97131 7.53968 4.55594 7.86357 4.14199C8.17726 3.74111 8.60573 3.29474 9.12419 2.75282L9.24874 2.62325C10.0696 1.80048 10.6687 1.16822 11.8292 1.03017ZM12.011 2.56197C11.4595 2.62759 11.2184 2.83432 10.3507 3.70204L10.2392 3.81956C9.69954 4.3836 9.33443 4.76675 9.07899 5.09322C8.83374 5.40666 8.73191 5.61661 8.68624 5.81543C8.67358 5.87055 8.66259 5.92611 8.6551 5.98217C8.62813 6.18436 8.64992 6.41732 8.76961 6.79679C8.89432 7.19214 9.10192 7.67935 9.40946 8.3969C9.5336 8.68663 9.46942 9.02263 9.24673 9.24567L2.54287 15.9494V17.4571H3.59857V16.4959C3.59883 16.07 3.94412 15.7245 4.37001 15.7245H5.33129V14.7642C5.33131 14.3382 5.67669 13.9928 6.10273 13.9928H7.51502L10.7534 10.7534C10.9765 10.5303 11.3133 10.4654 11.6032 10.5896C12.3208 10.8971 12.808 11.1048 13.2034 11.2295C13.5826 11.3491 13.8148 11.3709 14.017 11.344C14.0731 11.3365 14.1287 11.3255 14.1838 11.3128C14.3823 11.2671 14.5918 11.1653 14.9054 10.9199C15.2322 10.6645 15.6151 10.2995 16.1791 9.75976L16.3086 9.63523C17.0984 8.8455 17.4315 8.51235 17.509 7.98903C17.542 7.76778 17.5282 7.5362 17.4703 7.21036C17.4002 6.82452 17.2488 6.57029 17.0873 6.29678C16.7421 5.71956 16.2456 5.04099 15.5101 4.30553C14.7746 3.57007 14.0961 3.07355 13.5188 2.72831C13.2453 2.56683 12.9909 2.41544 12.6051 2.34538C12.2793 2.28748 12.0477 2.27362 11.8265 2.30666C11.3031 2.38417 10.97 2.71719 10.1802 3.50692L10.0557 3.63743C9.51587 4.20141 9.15091 4.58432 8.8955 4.91113C8.65012 5.22472 8.54835 5.43422 8.50263 5.63269C8.48996 5.68782 8.47896 5.74338 8.47147 5.79943C8.4445 6.00162 8.46629 6.23459 8.58598 6.61406C8.71069 7.0094 8.91829 7.49661 9.22583 8.21417C9.34997 8.50389 9.28579 8.83989 9.0631 9.06294L2.54287 15.5832V17.4571H3.59857V16.4959C3.59883 16.07 3.94412 15.7245 4.37001 15.7245H5.33129V14.7642C5.33131 14.3382 5.67669 13.9928 6.10273 13.9928H7.29791L10.5698 10.7199C10.7929 10.4968 11.1299 10.4319 11.4198 10.5561C12.1374 10.8636 12.6246 11.0713 13.02 11.196C13.3992 11.3156 13.6314 11.3374 13.8336 11.3105C13.8887 11.303 13.9443 11.292 13.9994 11.2793C14.1979 11.2336 14.4074 11.1318 14.721 10.8864C15.0478 10.631 15.4307 10.266 15.9947 9.72627L16.1242 9.60174C16.914 8.81201 17.2471 8.47886 17.3246 7.95554C17.3576 7.73429 17.3438 7.50271 17.2859 7.17687C17.2158 6.79103 17.0644 6.5368 16.9029 6.26329C16.5577 5.68607 16.0612 5.0075 15.3257 4.27204C14.5902 3.53658 13.9117 3.04006 13.3345 2.69482C13.061 2.53334 12.8066 2.38195 12.4208 2.31189C12.095 2.25399 11.8634 2.24013 11.6421 2.27317C11.1188 2.35068 10.7858 2.6837 9.99599 3.47343L9.87144 3.60394C9.33169 4.16792 8.96673 4.55083 8.71132 4.87764C8.46594 5.19123 8.36417 5.40073 8.31845 5.5992C8.30578 5.65433 8.29478 5.70988 8.28729 5.76594C8.26032 5.96813 8.28211 6.2011 8.4018 6.58057C8.52651 6.97591 8.73411 7.46312 9.04165 8.18068C9.16579 8.4704 9.10161 8.8064 8.87892 9.02945L2.54287 15.3655V17.4571H3.59857V16.4959C3.59883 16.07 3.94412 15.7245 4.37001 15.7245H5.33129V14.7642C5.33131 14.3382 5.67669 13.9928 6.10273 13.9928H7.08002L10.5709 10.5008C10.794 10.2777 11.131 10.2128 11.4209 10.337C12.1385 10.6445 12.6257 10.8522 13.0211 10.9769C13.4003 11.0965 13.6325 11.1183 13.8347 11.0914C13.8898 11.0839 13.9454 11.0729 14.0005 11.0602C14.199 11.0145 14.4085 10.9127 14.7221 10.6673C15.0489 10.4119 15.4318 10.0469 15.9958 9.50717L16.1253 9.38264C16.9151 8.59291 17.2482 8.25976 17.3257 7.73644C17.3587 7.51519 17.3449 7.28361 17.287 6.95777C17.2169 6.57193 17.0655 6.3177 16.904 6.04419C16.5588 5.46697 16.0623 4.7884 15.3268 4.05294C14.5914 3.31748 13.9128 2.82096 13.3356 2.47572C13.0621 2.31424 12.8077 2.16285 12.4219 2.09279C12.096 2.03489 11.8644 2.02103 11.6432 2.05407C11.1199 2.13158 10.7869 2.4646 9.99712 3.25433L9.87257 3.38484C9.33282 3.94882 8.96786 4.33173 8.71245 4.65854C8.46707 4.97213 8.3653 5.18163 8.31958 5.3801C8.30691 5.43523 8.29591 5.49078 8.28842 5.54684C8.26145 5.74903 8.28324 5.982 8.40293 6.36147C8.52764 6.75681 8.73524 7.24402 9.04278 7.96158C9.16692 8.2513 9.10274 8.5873 8.88005 8.81035L2.54287 15.1475V17.4571H3.59857V16.4959C3.59883 16.07 3.94412 15.7245 4.37001 15.7245H5.33129V14.7642C5.33131 14.3382 5.67669 13.9928 6.10273 13.9928H6.87417V15.5356H7.83444C8.03893 15.5356 8.23522 15.4541 8.37987 15.3096L11.4767 12.2108C11.9562 12.4142 12.3705 12.585 12.7383 12.701C13.2395 12.8591 13.7139 12.9413 14.2209 12.8738C14.3247 12.8599 14.4282 12.841 14.5303 12.8175L14.7131 12.7683C15.1323 12.6407 15.4959 12.42 15.8582 12.1365C16.2591 11.8229 16.7054 11.3943 17.2474 10.8759L17.389 10.7393C18.1988 9.92961 18.832 9.33136 18.97 8.17089C19.0312 7.6568 19.0008 7.234 18.8596 6.81587C18.7298 6.43192 18.5141 6.08516 18.3081 5.74813C17.8888 5.06218 17.3092 4.28916 16.5101 3.4901C15.7108 2.69085 14.9371 2.11141 14.251 1.69211C13.914 1.48615 13.5682 1.26936 13.1843 1.13966C12.7662 0.998431 12.3432 0.969077 11.8292 1.03017Z"
              fill="currentColor"
            />
          </svg>
        </div>
        <div className={styles.text}>
          <h2 className={styles.heading}>Аренда создана</h2>
          <p className={styles.subtext}>
            Добавьте арендатора и его контакты, чтобы все данные были в одном месте
          </p>
        </div>
        <div className={styles.actions}>
          <Button
            type="button"
            variant="secondary"
            size="large"
            fullWidth
            onClick={onAddLater}
          >
            Добавить позже
          </Button>
          <Button
            type="button"
            variant="primary"
            size="large"
            fullWidth
            leftIcon={<Users />}
            onClick={onAddTenant}
          >
            Добавить арендатора
          </Button>
        </div>
      </div>
    </div>
  );
}
```

**Step 2: Стили**

Скопировать `PropertySuccessStep.module.css` в `LeaseSuccessStep.module.css` без изменений.

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint`
Expected: 0 errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/leases/ui/LeaseSuccessStep.tsx apps/frontend/widgets/leases/ui/LeaseSuccessStep.module.css
git commit -m "feat: lease success step"
```

---

### Task 8: Wizard и страница

**Files:**
- Create: `apps/frontend/widgets/leases/ui/LeaseCreateWizard.tsx`
- Create: `apps/frontend/widgets/leases/ui/LeaseCreateWizard.module.css`
- Create: `apps/frontend/widgets/leases/ui/index.ts`
- Create: `apps/frontend/app/(cabinet)/leases/new/page.tsx`
- Create: `apps/frontend/app/(cabinet)/leases/new/page.module.css`

**Step 1: Создать Wizard**

```tsx
'use client';

import { useEffect, useState, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { useProperty } from '@/features/properties/api';
import { useCreateLease } from '@/features/leases/api';
import { useLeaseCreateDraft, type LeaseCreateStep } from '../lib/use-lease-create-draft';
import { LeaseCreateHeader } from './LeaseCreateHeader';
import { LeasePriceStep } from './LeasePriceStep';
import { LeaseDatesStep } from './LeaseDatesStep';
import { LeaseSuccessStep } from './LeaseSuccessStep';
import styles from './LeaseCreateWizard.module.css';

export function LeaseCreateWizard(): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();
  const propertyId = searchParams.get('propertyId') ?? '';
  const { draft, setDraft } = useLeaseCreateDraft();
  const [isSubmitting, setIsSubmitting] = useState(false);

  const propertyQuery = useProperty(propertyId);
  const createLease = useCreateLease();

  useEffect(() => {
    if (!propertyId || !/^[0-9a-fA-F-]{36}$/.test(propertyId)) {
      router.replace(ROUTES.properties);
      return;
    }
    if (propertyQuery.isError) {
      router.replace(ROUTES.properties);
      return;
    }
    if (propertyQuery.data && propertyQuery.data.occupancy !== 'free') {
      router.replace(ROUTES.properties);
    }
  }, [propertyId, propertyQuery.isError, propertyQuery.data, router]);

  const handleCancel = () => {
    router.push(ROUTES.properties);
  };

  const handleBack = () => {
    if (draft.step === 1) {
      router.push(ROUTES.properties);
      return;
    }
    setDraft((prev) => ({ ...prev, step: ((prev.step - 1) as LeaseCreateStep) }));
  };

  const handleNext = () => {
    setDraft((prev) => ({ ...prev, step: ((prev.step + 1) as LeaseCreateStep) }));
  };

  const handleSubmit = async () => {
    if (!draft.rentAmount || !draft.paymentDay || !draft.startDate) return;
    if (!propertyId) return;

    setIsSubmitting(true);
    try {
      await createLease.mutateAsync({
        property_id: propertyId,
        rent_amount_kopecks: Math.round(Number(draft.rentAmount) * 100),
        deposit_amount_kopecks: Math.round(Number(draft.depositAmount || '0') * 100),
        payment_day: draft.paymentDay,
        start_date: draft.startDate,
        end_date: draft.endDate || undefined,
      });
      handleNext();
    } catch (error: unknown) {
      console.error('Failed to create lease', error);
    } finally {
      setIsSubmitting(false);
    }
  };

  if (draft.step === 3) {
    return (
      <div className={styles.root}>
        <LeaseSuccessStep
          onAddLater={() => router.push(ROUTES.properties)}
          onAddTenant={() => router.push(ROUTES.tenants)}
        />
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <LeaseCreateHeader step={draft.step} onBack={handleBack} onCancel={handleCancel} />
      <div className={styles.content}>
        {draft.step === 1 && (
          <LeasePriceStep
            rentAmount={draft.rentAmount ?? ''}
            depositAmount={draft.depositAmount ?? ''}
            onRentChange={(rentAmount) => setDraft((prev) => ({ ...prev, rentAmount }))}
            onDepositChange={(depositAmount) => setDraft((prev) => ({ ...prev, depositAmount }))}
            onNext={handleNext}
          />
        )}
        {draft.step === 2 && (
          <LeaseDatesStep
            paymentDay={draft.paymentDay}
            startDate={draft.startDate}
            endDate={draft.endDate}
            onPaymentDayChange={(paymentDay) => setDraft((prev) => ({ ...prev, paymentDay }))}
            onStartDateChange={(startDate) => setDraft((prev) => ({ ...prev, startDate }))}
            onEndDateChange={(endDate) => setDraft((prev) => ({ ...prev, endDate }))}
            onSubmit={handleSubmit}
            isLoading={isSubmitting}
          />
        )}
      </div>
    </div>
  );
}
```

**Step 2: Стили Wizard**

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 48px;
}

.content {
  display: flex;
  flex-direction: column;
  gap: 52px;
}
```

**Step 3: Экспорты leases widgets**

```ts
export { LeaseCreateWizard } from './ui/LeaseCreateWizard';
```

**Step 4: Страница**

```tsx
import type { Metadata } from 'next';
import { Suspense } from 'react';
import { LeaseCreateWizard } from '@/widgets/leases';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Создать аренду — Arenda Platform',
  description: 'Добавление новой аренды',
};

export default function LeasesNewPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <Suspense fallback={null}>
          <LeaseCreateWizard />
        </Suspense>
      </div>
    </div>
  );
}
```

**Step 5: Стили страницы**

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

**Step 6: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run lint && npm run build`
Expected: 0 errors, exit 0.

**Step 7: Commit**

```bash
git add apps/frontend/widgets/leases/ui/LeaseCreateWizard.tsx apps/frontend/widgets/leases/ui/LeaseCreateWizard.module.css apps/frontend/widgets/leases/ui/index.ts apps/frontend/app/(cabinet)/leases/new/page.tsx apps/frontend/app/(cabinet)/leases/new/page.module.css
git commit -m "feat: lease create wizard and page"
```

---

### Task 9: Адаптив и скриншоты

**Files:**
- Modify: `apps/frontend/widgets/leases/ui/LeaseCreateWizard.module.css`
- Modify: `apps/frontend/widgets/leases/ui/LeasePriceStep.module.css`
- Modify: `apps/frontend/widgets/leases/ui/LeaseDatesStep.module.css`

**Step 1: Добавить sticky bottom bar для кнопки на мобильных**

Обновить `LeaseCreateWizard.module.css`:

```css
.root {
  display: flex;
  flex-direction: column;
  gap: 48px;
}

.content {
  display: flex;
  flex-direction: column;
  gap: 52px;
}

@media (max-width: 767px) {
  .content {
    gap: 32px;
  }

  .content > button:last-child {
    position: sticky;
    bottom: 20px;
    margin-top: auto;
  }
}
```

**Step 2: Скриншоты**

Run script `tools/screenshots/capture-leases.js` (mirror `tools/screenshots/capture.js`) for step 1, step 2, success at mobile/desktop.
Expected: screenshots saved to `screenshots/lease-step{1,2,success}-{mobile,desktop}.png`.

**Step 3: Проверить**

Run: `cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend && npm run build`
Expected: exit 0.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/leases/ui/*.module.css tools/screenshots/capture-leases.js screenshots/lease-*.png
git commit -m "feat: lease creation responsive and screenshots"
```

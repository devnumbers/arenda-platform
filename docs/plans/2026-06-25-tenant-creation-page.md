# Tenant Creation Page Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the `/tenants/new` page that lets owners create a tenant contact following the existing property/lease wizard patterns.

**Architecture:** A single-step wizard (`TenantCreateWizard`) with `sessionStorage` draft persistence, reusing shared UI primitives and the existing `useCreateTenantContact` mutation. The page matches the Figma layouts for desktop, tablet and mobile.

**Tech Stack:** Next.js App Router, React, TypeScript, CSS Modules, Hero UI v3-based shared UI kit, TanStack Query.

**Note on testing:** The user decided to skip automated tests for this task. Verification is done via TypeScript type-check and ESLint instead of unit/component tests.

---

## Pre-read before you start

- Design doc: `docs/plans/2026-06-25-tenant-creation-design.md`
- Domain glossary: `CONTEXT.md`
- Existing wizard to mirror:
  - `apps/frontend/widgets/leases/ui/LeaseCreateWizard.tsx`
  - `apps/frontend/widgets/leases/lib/use-lease-create-draft.ts`
  - `apps/frontend/widgets/leases/ui/LeaseCreateHeader.tsx`
  - `apps/frontend/widgets/leases/ui/LeaseSuccessStep.tsx`
  - `apps/frontend/widgets/leases/ui/LeasePriceStep.tsx`
- Existing API hook: `apps/frontend/features/tenant-contacts/api/hooks.ts`
- Routes config: `apps/frontend/shared/config/routes.ts`

---

### Task 1: Create the tenant creation draft hook

**Files:**
- Create: `apps/frontend/widgets/tenants/lib/use-tenant-create-draft.ts`

**Step 1: Create the directory and file**

```bash
mkdir -p apps/frontend/widgets/tenants/lib
```

**Step 2: Write the hook**

```ts
'use client';

import { useState, useEffect } from 'react';
import type { Dispatch, SetStateAction } from 'react';

export type TenantCreateStep = 'form' | 'success';

export type TenantCreateDraft = {
  step: TenantCreateStep;
  name: string;
  surname: string;
  patronymic: string;
  phone: string;
  comment: string;
};

const STORAGE_KEY = 'tenant-create-draft';

const DEFAULT_DRAFT: TenantCreateDraft = {
  step: 'form',
  name: '',
  surname: '',
  patronymic: '',
  phone: '',
  comment: '',
};

export function useTenantCreateDraft(): {
  draft: TenantCreateDraft;
  setDraft: Dispatch<SetStateAction<TenantCreateDraft>>;
} {
  const [draft, setDraft] = useState<TenantCreateDraft>(DEFAULT_DRAFT);

  useEffect(() => {
    // Load persisted draft after hydration to avoid SSR/hydration mismatch.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(loadDraft());
  }, []);

  useEffect(() => {
    try {
      if (draft.step === 'success') {
        sessionStorage.removeItem(STORAGE_KEY);
        return;
      }
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(draft));
    } catch {
      // Ignore storage quota / privacy mode errors.
    }
  }, [draft]);

  return { draft, setDraft };
}

function loadDraft(): TenantCreateDraft {
  if (typeof window === 'undefined') return DEFAULT_DRAFT;

  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_DRAFT;
    const parsed = JSON.parse(raw) as unknown;
    return validateDraft(parsed);
  } catch {
    return DEFAULT_DRAFT;
  }
}

function isString(value: unknown): value is string {
  return typeof value === 'string';
}

function isStep(value: unknown): value is TenantCreateStep {
  return value === 'form' || value === 'success';
}

function validateDraft(parsed: unknown): TenantCreateDraft {
  if (parsed === null || typeof parsed !== 'object') return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  if (!isStep(record.step)) return DEFAULT_DRAFT;
  if (!isString(record.name)) return DEFAULT_DRAFT;
  if (!isString(record.surname)) return DEFAULT_DRAFT;
  if (!isString(record.patronymic)) return DEFAULT_DRAFT;
  if (!isString(record.phone)) return DEFAULT_DRAFT;
  if (!isString(record.comment)) return DEFAULT_DRAFT;

  return {
    step: record.step,
    name: record.name,
    surname: record.surname,
    patronymic: record.patronymic,
    phone: record.phone,
    comment: record.comment,
  };
}
```

**Step 3: Verify types**

Run:
```bash
cd apps/frontend && npx tsc --noEmit
```

Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/tenants/lib/use-tenant-create-draft.ts
git commit -m "feat(frontend): add tenant create draft hook with sessionStorage"
```

---

### Task 2: Create the tenant creation header

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantCreateHeader.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantCreateHeader.module.css`

**Step 1: Create the directory**

```bash
mkdir -p apps/frontend/widgets/tenants/ui
```

**Step 2: Write the component**

```tsx
'use client';

import type { JSX } from 'react';
import { Cancel } from '@/shared/assets/icons';
import { IconButton } from '@/shared/ui/icon-button';
import styles from './TenantCreateHeader.module.css';

export type TenantCreateHeaderProps = {
  readonly onClose: () => void;
};

export function TenantCreateHeader({ onClose }: TenantCreateHeaderProps): JSX.Element {
  return (
    <header className={styles.root}>
      <IconButton
        variant="icon-black"
        size="small"
        icon={<Cancel />}
        aria-label="Закрыть"
        onClick={onClose}
        className={styles.iconButton}
      />
      <h1 className={styles.title}>Добавление арендатора</h1>
      <div className={styles.spacer} />
    </header>
  );
}
```

**Step 3: Write the styles**

```css
.root {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.iconButton {
  flex-shrink: 0;
}

.title {
  flex: 1;
  margin: 0;
  font-size: 20px;
  font-weight: 500;
  line-height: 24px;
  color: #1e1e1e;
  text-align: center;
}

.spacer {
  width: 40px;
}

@media (min-width: 768px) {
  .title {
    font-size: 28px;
    font-weight: 500;
    line-height: 36px;
  }
}
```

**Step 4: Verify types and lint**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 5: Commit**

```bash
git add apps/frontend/widgets/tenants/ui/TenantCreateHeader.tsx apps/frontend/widgets/tenants/ui/TenantCreateHeader.module.css
git commit -m "feat(frontend): add tenant create header"
```

---

### Task 3: Create the tenant form step

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantFormStep.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantFormStep.module.css`

**Step 1: Write the component**

```tsx
'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import styles from './TenantFormStep.module.css';

export type TenantFormStepProps = {
  readonly name: string;
  readonly surname: string;
  readonly patronymic: string;
  readonly phone: string;
  readonly comment: string;
  readonly isLoading: boolean;
  readonly error?: string;
  readonly onNameChange: (value: string) => void;
  readonly onSurnameChange: (value: string) => void;
  readonly onPatronymicChange: (value: string) => void;
  readonly onPhoneChange: (value: string) => void;
  readonly onCommentChange: (value: string) => void;
  readonly onSubmit: () => void;
};

const MAX_COMMENT_LENGTH = 500;

function isPhoneValid(phone: string): boolean {
  if (phone.trim() === '') return true;
  const digits = phone.replace(/\D/g, '');
  return digits.length >= 10;
}

export function TenantFormStep({
  name,
  surname,
  patronymic,
  phone,
  comment,
  isLoading,
  error,
  onNameChange,
  onSurnameChange,
  onPatronymicChange,
  onPhoneChange,
  onCommentChange,
  onSubmit,
}: TenantFormStepProps): JSX.Element {
  const isNameValid = name.trim() !== '';
  const isCommentValid = comment.length <= MAX_COMMENT_LENGTH;
  const isPhoneFormatValid = isPhoneValid(phone);
  const canSubmit = isNameValid && isCommentValid && isPhoneFormatValid && !isLoading;

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Добавление арендатора</h2>
      <div className={styles.fields}>
        <TextField
          label="Имя"
          placeholder=" "
          required
          value={name}
          onChange={(e) => onNameChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Фамилия"
          placeholder=" "
          value={surname}
          onChange={(e) => onSurnameChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Отчество"
          placeholder=" "
          value={patronymic}
          onChange={(e) => onPatronymicChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Комментарий"
          placeholder=" "
          multiline
          maxLength={MAX_COMMENT_LENGTH}
          value={comment}
          onChange={(e) => onCommentChange(e.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Телефон"
          placeholder=" "
          value={phone}
          onChange={(e) => onPhoneChange(e.currentTarget.value)}
          fullWidth
          error={!isPhoneFormatValid ? 'Введите корректный номер телефона' : undefined}
        />
      </div>
      {error && <p className={styles.error}>{error}</p>}
      <div className={styles.footer}>
        <Button
          type="button"
          variant="primary"
          size="large"
          fullWidth
          disabled={!canSubmit}
          loading={isLoading}
          onClick={onSubmit}
        >
          Добавить арендатора
        </Button>
      </div>
    </div>
  );
}
```

**Step 2: Write the styles**

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

.error {
  margin: 0;
  color: #ff4646;
  font-size: 14px;
  line-height: 18px;
}

.footer {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

@media (max-width: 767px) {
  .root {
    min-height: calc(100vh - 84px - 96px);
  }

  .fields {
    flex: 1;
  }

  .footer {
    position: sticky;
    bottom: 20px;
  }

  .heading {
    font-size: 20px;
    line-height: 22px;
  }
}
```

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/tenants/ui/TenantFormStep.tsx apps/frontend/widgets/tenants/ui/TenantFormStep.module.css
git commit -m "feat(frontend): add tenant form step"
```

---

### Task 4: Create the tenant success step

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantSuccessStep.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantSuccessStep.module.css`

**Step 1: Write the component**

```tsx
'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { Icon } from '@/shared/ui/icon';
import { BoldProfile, BoldWallet } from '@/shared/assets/icons';
import styles from './TenantSuccessStep.module.css';

export type TenantSuccessStepProps = {
  readonly onAddLater: () => void;
  readonly onAddPayments: () => void;
};

export function TenantSuccessStep({
  onAddLater,
  onAddPayments,
}: TenantSuccessStepProps): JSX.Element {
  return (
    <div className={styles.root}>
      <div className={styles.card}>
        <div className={styles.iconWrapper}>
          <BoldProfile className={styles.illustration} />
        </div>

        <div className={styles.text}>
          <h2 className={styles.heading}>Арендатор добавлен</h2>
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
            leftIcon={
              <Icon size="m">
                <BoldWallet />
              </Icon>
            }
            onClick={onAddPayments}
          >
            Добавить платежи
          </Button>
        </div>
      </div>
    </div>
  );
}
```

**Step 2: Write the styles**

```css
.root {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  padding: 24px;
}

.card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 32px;
  width: 100%;
  max-width: 480px;
  padding: 40px 24px;
  text-align: center;
  border: 1px solid #e7e9ec;
  border-radius: 32px;
  background: #ffffff;
}

.iconWrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 96px;
  height: 96px;
  color: var(--color-primary);
}

.illustration {
  width: 96px;
  height: 96px;
}

.text {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.heading {
  margin: 0;
  font-size: 20px;
  font-weight: 500;
  line-height: 22px;
  color: var(--color-text);
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
}

.actions > button {
  flex: 1 1 0;
}

@media (min-width: 768px) {
  .actions {
    flex-direction: row;
    justify-content: center;
  }

  .heading {
    font-size: 28px;
    line-height: 36px;
  }
}
```

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/tenants/ui/TenantSuccessStep.tsx apps/frontend/widgets/tenants/ui/TenantSuccessStep.module.css
git commit -m "feat(frontend): add tenant success step"
```

---

### Task 5: Create the tenant create wizard

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantCreateWizard.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantCreateWizard.module.css`

**Step 1: Write the component**

```tsx
'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { useCreateTenantContact } from '@/features/tenant-contacts/api';
import { ApiError } from '@/shared/api/errors';
import { ROUTES } from '@/shared/config/routes';
import { useTenantCreateDraft, type TenantCreateDraft } from '../lib/use-tenant-create-draft';
import { TenantCreateHeader } from './TenantCreateHeader';
import { TenantFormStep } from './TenantFormStep';
import { TenantSuccessStep } from './TenantSuccessStep';
import styles from './TenantCreateWizard.module.css';

function formatErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.detail;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Не удалось добавить арендатора. Попробуйте ещё раз.';
}

export function TenantCreateWizard(): JSX.Element {
  const router = useRouter();
  const { draft, setDraft } = useTenantCreateDraft();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);

  const createTenantContact = useCreateTenantContact();

  const handleClose = () => {
    router.push(ROUTES.tenants);
  };

  const handleFieldChange = <K extends keyof Omit<TenantCreateDraft, 'step'>>(
    field: K,
  ) =>
    (value: TenantCreateDraft[K]) => {
      setDraft((prev) => ({ ...prev, [field]: value }));
    };

  const handleSubmit = async () => {
    if (draft.name.trim() === '') return;

    setIsSubmitting(true);
    setSubmitError(undefined);

    try {
      await createTenantContact.mutateAsync({
        name: draft.name.trim(),
        surname: draft.surname.trim() || undefined,
        patronymic: draft.patronymic.trim() || undefined,
        phone: draft.phone.trim() || undefined,
        comment: draft.comment || undefined,
      });

      setDraft((prev) => ({ ...prev, step: 'success' }));
    } catch (error: unknown) {
      console.error('Failed to create tenant contact', error);
      setSubmitError(formatErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  };

  if (draft.step === 'success') {
    return (
      <div className={styles.root}>
        <TenantSuccessStep
          onAddLater={() => router.push(ROUTES.tenants)}
          onAddPayments={() => router.push(ROUTES.finance)}
        />
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <TenantCreateHeader onClose={handleClose} />
      <div className={styles.content}>
        <TenantFormStep
          name={draft.name}
          surname={draft.surname}
          patronymic={draft.patronymic}
          phone={draft.phone}
          comment={draft.comment}
          isLoading={isSubmitting}
          error={submitError}
          onNameChange={handleFieldChange('name')}
          onSurnameChange={handleFieldChange('surname')}
          onPatronymicChange={handleFieldChange('patronymic')}
          onPhoneChange={handleFieldChange('phone')}
          onCommentChange={handleFieldChange('comment')}
          onSubmit={handleSubmit}
        />
      </div>
    </div>
  );
}
```

**Step 2: Write the styles**

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
  .root {
    gap: 32px;
  }

  .content {
    gap: 32px;
  }
}
```

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/tenants/ui/TenantCreateWizard.tsx apps/frontend/widgets/tenants/ui/TenantCreateWizard.module.css
git commit -m "feat(frontend): wire up tenant create wizard"
```

---

### Task 6: Create the route page

**Files:**
- Create: `apps/frontend/app/(cabinet)/tenants/new/page.tsx`
- Create: `apps/frontend/app/(cabinet)/tenants/new/page.module.css`

**Step 1: Write the page**

```tsx
import type { Metadata } from 'next';
import { TenantCreateWizard } from '@/widgets/tenants';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Добавить арендатора — Arenda Platform',
  description: 'Добавление нового арендатора',
};

export default function TenantsNewPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <TenantCreateWizard />
      </div>
    </div>
  );
}
```

**Step 2: Write the styles**

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

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/app/(cabinet)/tenants/new/page.tsx apps/frontend/app/(cabinet)/tenants/new/page.module.css
git commit -m "feat(frontend): add /tenants/new route page"
```

---

### Task 7: Add widget barrel exports

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/index.ts`
- Create: `apps/frontend/widgets/tenants/index.ts`

**Step 1: Write `widgets/tenants/ui/index.ts`**

```ts
export { TenantCreateWizard } from './TenantCreateWizard';
```

**Step 2: Write `widgets/tenants/index.ts`**

```ts
export { TenantCreateWizard } from './ui';
```

**Step 3: Verify**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/widgets/tenants/ui/index.ts apps/frontend/widgets/tenants/index.ts
git commit -m "feat(frontend): add tenants widget barrel exports"
```

---

### Task 8: Final verification

**Files:**
- None new.

**Step 1: Run type-check and lint**

Run:
```bash
cd apps/frontend && npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 2: Run build (optional but recommended)**

Run:
```bash
cd apps/frontend && npm run build
```

Expected: build succeeds.

**Step 3: Manual check in browser**

Run:
```bash
make backend-run
```

In a separate terminal:
```bash
cd apps/frontend && npm run dev
```

Open `http://localhost:3000/tenants/new`, verify:
- Form renders with all five fields.
- Submit button is disabled when name is empty.
- Comment counter shows `0/500`.
- Filling name enables submit.
- Submit creates tenant and shows success screen.
- «Добавить позже» navigates to `/tenants`.
- «Добавить платежи» navigates to `/finance`.

**Step 4: Commit final adjustments**

```bash
git add .
git commit -m "feat(frontend): finalize tenant creation page"
```

---

## Done

After these tasks the page `/tenants/new` is fully implemented and ready for review.

# Tenant Edit Page Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a full-featured tenant edit page at `/tenants/[id]/edit` that reuses a shared `TenantForm` component and allows editing all tenant contact fields, including email.

**Architecture:** Extract a reusable `TenantForm` from the existing create form, build a `TenantEditForm` widget that loads the contact via TanStack Query, validates locally, and submits a `PATCH`. Follow the established property-edit page pattern and shared UI conventions. Backend already supports the required endpoint.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, TanStack Query, Hero UI 3, CSS Modules, custom `shared/ui` components.

---

## Preparation

Before starting, read these files to confirm context:
- `apps/frontend/widgets/tenants/ui/TenantFormStep.tsx`
- `apps/frontend/widgets/tenants/ui/TenantCreateWizard.tsx`
- `apps/frontend/widgets/tenant-detail/ui/TenantDetailHeader.tsx`
- `apps/frontend/app/(cabinet)/properties/[id]/edit/page.tsx`
- `apps/frontend/widgets/properties/ui/PropertyEditForm.tsx`
- `apps/frontend/shared/config/routes.ts`
- `apps/frontend/features/tenant-contacts/api/hooks.ts`
- `apps/frontend/features/tenant-contacts/api/index.ts`
- `apps/frontend/shared/ui/text-field/TextField.tsx`
- `apps/frontend/shared/ui/button/Button.tsx`

---

### Task 1: Add `tenantEdit` route

**Files:**
- Modify: `apps/frontend/shared/config/routes.ts`

**Step 1: Open `apps/frontend/shared/config/routes.ts`**

**Step 2: Add `tenantEdit` helper next to existing tenant route**

Find the existing tenant route block and add the edit route:

```ts
export const ROUTES = {
  // ...existing routes...
  tenants: '/tenants',
  tenant: (id: string) => `/tenants/${id}`,
  tenantEdit: (id: string) => `/tenants/${id}/edit`,
  // ...existing routes...
};
```

**Step 3: Verify no TypeScript errors**

Run:

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npx tsc --noEmit
```

Expected: no errors in `shared/config/routes.ts`.

**Step 4: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/frontend/shared/config/routes.ts
git commit -m "feat(tenants): add tenantEdit route helper"
```

---

### Task 2: Extract shared `TenantForm` component

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantForm.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantForm.module.css`
- Modify: `apps/frontend/widgets/tenants/ui/TenantFormStep.tsx`

**Goal:** Create a reusable, controlled tenant form that can be used for both create and edit.

**Step 1: Create `apps/frontend/widgets/tenants/ui/TenantForm.tsx`**

```tsx
'use client';

import { useCallback, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';

import { Button } from '@/shared/ui/button';
import { IconLink } from '@/shared/ui/icon-link';
import { TextField } from '@/shared/ui/text-field';
import { ArrowLeft } from '@/shared/assets/icons';
import { formatPhoneInput } from '@/shared/lib/phone';

import styles from './TenantForm.module.css';

export interface TenantContactFormData {
  name: string;
  surname: string;
  patronymic: string;
  phone: string;
  email: string;
  comment: string;
}

export interface TenantFormProps {
  initialData?: Partial<TenantContactFormData>;
  submitLabel: string;
  isLoading: boolean;
  error?: string;
  onSubmit: (data: TenantContactFormData) => void;
  onCancel?: () => void;
  backHref: string;
}

const MAX_COMMENT_LENGTH = 500;

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function getInitialData(
  initialData?: Partial<TenantContactFormData>,
): TenantContactFormData {
  return {
    name: initialData?.name ?? '',
    surname: initialData?.surname ?? '',
    patronymic: initialData?.patronymic ?? '',
    phone: initialData?.phone ?? '',
    email: initialData?.email ?? '',
    comment: initialData?.comment ?? '',
  };
}

export function TenantForm({
  initialData,
  submitLabel,
  isLoading,
  error,
  onSubmit,
  onCancel,
  backHref,
}: TenantFormProps) {
  const router = useRouter();

  const [name, setName] = useState(initialData?.name ?? '');
  const [surname, setSurname] = useState(initialData?.surname ?? '');
  const [patronymic, setPatronymic] = useState(initialData?.patronymic ?? '');
  const [phone, setPhone] = useState(initialData?.phone ?? '');
  const [email, setEmail] = useState(initialData?.email ?? '');
  const [comment, setComment] = useState(initialData?.comment ?? '');

  const [isPhoneTouched, setIsPhoneTouched] = useState(false);
  const [isEmailTouched, setIsEmailTouched] = useState(false);
  const [isSubmittedAttempt, setIsSubmittedAttempt] = useState(false);

  const isNameValid = name.trim() !== '';
  const isPhoneValid = phone === '' || phone.length === 18;
  const isEmailValid = email === '' || EMAIL_REGEX.test(email);
  const isCommentValid = comment.length <= MAX_COMMENT_LENGTH;

  const canSubmit = isNameValid && isPhoneValid && isEmailValid && isCommentValid && !isLoading;

  const nameError = (isSubmittedAttempt || name !== '') && !isNameValid
    ? 'Введите имя'
    : undefined;
  const phoneError = (isSubmittedAttempt || isPhoneTouched) && !isPhoneValid
    ? 'Введите корректный номер телефона'
    : undefined;
  const emailError = (isSubmittedAttempt || isEmailTouched) && !isEmailValid
    ? 'Введите корректный email'
    : undefined;

  const handlePhoneChange = useCallback((value: string) => {
    setIsPhoneTouched(true);
    setPhone(formatPhoneInput(value));
  }, []);

  const handleSubmit = useCallback(
    (event: React.FormEvent) => {
      event.preventDefault();
      setIsSubmittedAttempt(true);

      if (!canSubmit) {
        return;
      }

      onSubmit({
        name: name.trim(),
        surname: surname.trim(),
        patronymic: patronymic.trim(),
        phone: phone.trim(),
        email: email.trim(),
        comment: comment.trim(),
      });
    },
    [
      canSubmit,
      onSubmit,
      name,
      surname,
      patronymic,
      phone,
      email,
      comment,
    ],
  );

  const handleCancel = useCallback(() => {
    if (onCancel) {
      onCancel();
    } else {
      router.push(backHref);
    }
  }, [onCancel, router, backHref]);

  return (
    <form onSubmit={handleSubmit} className={styles.form}>
      <div className={styles.fields}>
        <TextField
          label="Имя"
          value={name}
          onChange={setName}
          required
          error={nameError}
          fullWidth
        />
        <TextField
          label="Фамилия"
          value={surname}
          onChange={setSurname}
          fullWidth
        />
        <TextField
          label="Отчество"
          value={patronymic}
          onChange={setPatronymic}
          fullWidth
        />
        <TextField
          label="Телефон"
          value={phone}
          onChange={handlePhoneChange}
          error={phoneError}
          placeholder="+7 (___) ___-__-__"
          fullWidth
        />
        <TextField
          label="Email"
          value={email}
          onChange={(value) => {
            setIsEmailTouched(true);
            setEmail(value);
          }}
          error={emailError}
          placeholder="example@mail.ru"
          fullWidth
        />
        <TextField
          label="Комментарий"
          value={comment}
          onChange={setComment}
          multiline
          maxLength={MAX_COMMENT_LENGTH}
          showCounter
          fullWidth
        />
      </div>

      {error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}

      <div className={styles.actions}>
        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={isLoading}
          disabled={!canSubmit}
        >
          {submitLabel}
        </Button>
        <IconLink
          href={backHref}
          icon={ArrowLeft}
          variant="clear"
          size="large"
          fullWidth
          onClick={handleCancel}
        >
          Отмена
        </IconLink>
      </div>
    </form>
  );
}
```

**Step 2: Create `apps/frontend/widgets/tenants/ui/TenantForm.module.css`**

Base the styles on `TenantFormStep.module.css`. Copy that file and adjust names:

```css
.form {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.fields {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.error {
  margin: 0;
  color: var(--color-error);
  font-size: var(--font-size-s);
  line-height: var(--line-height-s);
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

@media (max-width: 767px) {
  .actions {
    position: sticky;
    bottom: 20px;
    padding-top: 12px;
    background: var(--color-white);
  }
}
```

**Step 3: Refactor `TenantFormStep.tsx` to use `TenantForm`**

Replace the current implementation with:

```tsx
'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

import { TenantForm } from './TenantForm';

import type { TenantContactFormData } from './TenantForm';

interface TenantFormStepProps {
  onSubmit: (data: TenantContactFormData) => void;
  isLoading: boolean;
  error?: string;
}

export function TenantFormStep({ onSubmit, isLoading, error }: TenantFormStepProps) {
  return (
    <TenantForm
      submitLabel="Добавить арендатора"
      isLoading={isLoading}
      error={error}
      onSubmit={onSubmit}
      backHref="/tenants"
    />
  );
}
```

If `TenantCreateWizard` passes different props, adjust the interface to match. The important thing is that `TenantFormStep` becomes a thin wrapper.

**Step 4: Update `apps/frontend/widgets/tenants/ui/index.ts` exports**

Ensure the index exports both `TenantForm` and `TenantFormStep`:

```ts
export { TenantFormStep } from './TenantFormStep';
export { TenantForm } from './TenantForm';
export type { TenantContactFormData, TenantFormProps } from './TenantForm';
// ...existing exports...
```

**Step 5: Run TypeScript check**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npx tsc --noEmit
```

Expected: no errors.

**Step 6: Run lint**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npm run lint
```

Expected: no lint errors.

**Step 7: Manual smoke test of create flow**

Run the dev server:

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
make backend-run
```

In another terminal:

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npm run dev
```

Open `http://localhost:3000/tenants/new`, fill the form, and verify creation still works.

**Step 8: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/frontend/widgets/tenants/ui/
git commit -m "refactor(tenants): extract reusable TenantForm component"
```

---

### Task 3: Create `TenantEditForm` widget

**Files:**
- Create: `apps/frontend/widgets/tenants/ui/TenantEditForm.tsx`
- Create: `apps/frontend/widgets/tenants/ui/TenantEditForm.module.css`

**Step 1: Create `TenantEditForm.tsx`**

```tsx
'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';

import {
  useTenantContact,
  useUpdateTenantContact,
} from '@/features/tenant-contacts/api';
import { TenantDetailLoading } from '@/widgets/tenant-detail/ui';
import { ROUTES } from '@/shared/config/routes';
import { ApiError } from '@/shared/api/errors';

import { TenantForm } from './TenantForm';

import type { TenantContactFormData } from './TenantForm';

import styles from './TenantEditForm.module.css';

interface TenantEditFormProps {
  tenantId: string;
}

function isValidUUID(value: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);
}

function mapServerError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409) {
      return 'Арендатор с таким телефоном уже существует';
    }
    if (error.status === 404) {
      return 'Арендатор не найден';
    }
    return error.message || 'Не удалось сохранить изменения';
  }
  return 'Не удалось сохранить изменения';
}

export function TenantEditForm({ tenantId }: TenantEditFormProps) {
  const router = useRouter();
  const hasInitializedRef = useRef(false);

  const tenantQuery = useTenantContact(tenantId);
  const updateMutation = useUpdateTenantContact();

  const [initialData, setInitialData] = useState<Partial<TenantContactFormData>>({});
  const [submitError, setSubmitError] = useState<string | undefined>();

  useEffect(() => {
    if (tenantQuery.data && !hasInitializedRef.current) {
      hasInitializedRef.current = true;
      setInitialData({
        name: tenantQuery.data.name ?? '',
        surname: tenantQuery.data.surname ?? '',
        patronymic: tenantQuery.data.patronymic ?? '',
        phone: tenantQuery.data.phone ?? '',
        email: tenantQuery.data.email ?? '',
        comment: tenantQuery.data.comment ?? '',
      });
    }
  }, [tenantQuery.data]);

  const isInvalidId = !isValidUUID(tenantId);

  const handleSubmit = useCallback(
    async (data: TenantContactFormData) => {
      setSubmitError(undefined);

      try {
        const payload: Record<string, string | null> = {};

        if (data.name !== initialData.name) payload.name = data.name;
        if (data.surname !== initialData.surname) {
          payload.surname = data.surname || null;
        }
        if (data.patronymic !== initialData.patronymic) {
          payload.patronymic = data.patronymic || null;
        }
        if (data.phone !== initialData.phone) {
          payload.phone = data.phone || null;
        }
        if (data.email !== initialData.email) {
          payload.email = data.email || null;
        }
        if (data.comment !== initialData.comment) {
          payload.comment = data.comment || null;
        }

        await updateMutation.mutateAsync({ id: tenantId, body: payload });

        toast.success('Арендатор обновлён');
        router.push(ROUTES.tenant(tenantId));
      } catch (error) {
        const message = mapServerError(error);
        setSubmitError(message);
        toast.error(message);
      }
    },
    [initialData, router, tenantId, updateMutation],
  );

  if (isInvalidId) {
    return (
      <div className={styles.errorState}>
        <p>Арендатор не найден</p>
        <a href={ROUTES.tenants}>К списку арендаторов</a>
      </div>
    );
  }

  if (tenantQuery.isPending) {
    return <TenantDetailLoading />;
  }

  if (tenantQuery.isError) {
    return (
      <div className={styles.errorState}>
        <p>Не удалось загрузить арендатора</p>
        <button type="button" onClick={() => tenantQuery.refetch()}>
          Повторить
        </button>
      </div>
    );
  }

  return (
    <TenantForm
      initialData={initialData}
      submitLabel="Сохранить изменения"
      isLoading={updateMutation.isPending}
      error={submitError}
      onSubmit={handleSubmit}
      backHref={ROUTES.tenant(tenantId)}
    />
  );
}
```

**Note:** Verify the exact signature of `useUpdateTenantContact`. If it expects a different payload shape, adjust accordingly. The generated API client likely accepts `TenantContactUpdateRequest` directly.

**Step 2: Create `TenantEditForm.module.css`**

```css
.errorState {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 48px 20px;
  text-align: center;
}

.errorState p {
  margin: 0;
  color: var(--color-text);
  font-size: var(--font-size-m);
  line-height: var(--line-height-m);
}

.errorState a,
.errorState button {
  color: var(--color-primary);
  font-size: var(--font-size-m);
  text-decoration: none;
  background: none;
  border: none;
  cursor: pointer;
}
```

**Step 3: Update `apps/frontend/widgets/tenants/ui/index.ts`**

```ts
export { TenantEditForm } from './TenantEditForm';
// ...existing exports...
```

**Step 4: TypeScript check**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npx tsc --noEmit
```

Expected: no errors.

**Step 5: Lint**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npm run lint
```

Expected: no errors.

**Step 6: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/frontend/widgets/tenants/ui/
git commit -m "feat(tenants): add TenantEditForm widget"
```

---

### Task 4: Create the edit page

**Files:**
- Create: `apps/frontend/app/(cabinet)/tenants/[id]/edit/page.tsx`
- Create: `apps/frontend/app/(cabinet)/tenants/[id]/edit/page.module.css`

**Step 1: Create `page.tsx`**

Model after `apps/frontend/app/(cabinet)/properties/[id]/edit/page.tsx`:

```tsx
import { Metadata } from 'next';

import { TenantEditForm } from '@/widgets/tenants/ui';

import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Редактирование арендатора',
};

interface TenantEditPageProps {
  params: Promise<{ id: string }>;
}

export default async function TenantEditPage({ params }: TenantEditPageProps) {
  const { id } = await params;

  return (
    <div className={styles.page}>
      <div className={styles.container}>
        <h1 className={styles.title}>Редактирование арендатора</h1>
        <TenantEditForm tenantId={id} />
      </div>
    </div>
  );
}
```

**Step 2: Create `page.module.css`**

Base on `apps/frontend/app/(cabinet)/properties/[id]/edit/page.module.css`:

```css
.page {
  padding: 96px 0;
}

.container {
  max-width: 560px;
  margin: 0 auto;
}

.title {
  margin: 0 0 32px;
  font-size: var(--font-size-h1);
  line-height: var(--line-height-h1);
  font-weight: 600;
  color: var(--color-text);
}

@media (max-width: 767px) {
  .page {
    padding: 84px 20px 96px;
  }

  .container {
    max-width: 100%;
  }

  .title {
    margin-bottom: 24px;
  }
}
```

**Step 3: TypeScript check**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npx tsc --noEmit
```

Expected: no errors.

**Step 4: Lint**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npm run lint
```

Expected: no errors.

**Step 5: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/frontend/app/(cabinet)/tenants/[id]/edit/
git commit -m "feat(tenants): add tenant edit page"
```

---

### Task 5: Wire up the edit button in tenant detail header

**Files:**
- Modify: `apps/frontend/widgets/tenant-detail/ui/TenantDetailHeader.tsx`

**Step 1: Open `TenantDetailHeader.tsx`**

**Step 2: Replace the disabled edit button with an enabled `IconLink`**

Current code likely looks like:

```tsx
<Button variant="secondary" size="small" disabled>
  Редактировать
</Button>
```

Replace with:

```tsx
import { IconLink } from '@/shared/ui/icon-link';
import { ROUTES } from '@/shared/config/routes';

// ...
<IconLink
  href={ROUTES.tenantEdit(tenantId)}
  icon={EditIcon}
  variant="secondary"
  size="small"
>
  Редактировать
</IconLink>
```

Use the appropriate edit icon from `@/shared/assets/icons` (e.g. `Edit` or `Pencil`). If no suitable icon exists, use the existing `Button` style but as a Next.js `Link` wrapper.

**Step 3: TypeScript check**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npx tsc --noEmit
```

Expected: no errors.

**Step 4: Lint**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npm run lint
```

Expected: no errors.

**Step 5: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/frontend/widgets/tenant-detail/ui/TenantDetailHeader.tsx
git commit -m "feat(tenants): enable edit button in tenant detail header"
```

---

### Task 6: Update tenant detail info section to show email

**Files:**
- Modify: `apps/frontend/widgets/tenant-detail/ui/TenantInfoSection.tsx`

**Step 1: Open `TenantInfoSection.tsx`**

**Step 2: Add email row if `tenant.email` exists**

After the phone row, add:

```tsx
{tenant.email && (
  <div className={styles.row}>
    <span className={styles.label}>Email</span>
    <a href={`mailto:${tenant.email}`} className={styles.value}>
      {tenant.email}
    </a>
  </div>
)}
```

**Step 3: TypeScript check and lint**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npx tsc --noEmit && npm run lint
```

Expected: no errors.

**Step 4: Commit**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add apps/frontend/widgets/tenant-detail/ui/TenantInfoSection.tsx
git commit -m "feat(tenants): display email on tenant detail page"
```

---

### Task 7: Verify PATCH payload semantics

**Files:**
- Read: `apps/frontend/features/tenant-contacts/api/index.ts`
- Read: `apps/frontend/features/tenant-contacts/api/hooks.ts`
- Read: `apps/frontend/shared/api/generated.ts` (TenantContactUpdateRequest)

**Step 1: Confirm `useUpdateTenantContact` signature**

Open `apps/frontend/features/tenant-contacts/api/hooks.ts` and verify how `useUpdateTenantContact` is defined. The expected shape is:

```ts
useUpdateTenantContact(): UseMutationResult<
  TenantContactResponse,
  ApiError,
  { id: string; body: TenantContactUpdateRequest }
>
```

If the body type requires all fields or uses a different wrapper, adjust `TenantEditForm` payload construction.

**Step 2: Confirm nullable fields**

In `generated.ts`, `TenantContactUpdateRequest` fields should be nullable strings. If a field is not nullable, clearing it in the UI must omit it from the payload instead of sending `null`.

**Step 3: Run backend linter to ensure no accidental backend changes**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
make backend-lint
```

Expected: passes.

**Step 4: Commit any necessary fixes**

If changes are needed, commit them with a clear message.

---

### Task 8: Manual end-to-end verification

**Preconditions:**
- Backend running: `make backend-run`
- Frontend dev server: `cd apps/frontend && npm run dev`
- At least one tenant contact exists

**Step 1: Navigate to tenant list**

Open `http://localhost:3000/tenants`.

**Step 2: Open a tenant detail**

Click any tenant. Confirm the «Редактировать» button is enabled.

**Step 3: Open edit page**

Click «Редактировать». Confirm URL is `/tenants/<id>/edit` and the form is pre-filled.

**Step 4: Edit each field**

Change name, surname, patronymic, phone, email, comment. Save.

**Step 5: Verify redirect and updated detail page**

Confirm redirect to `/tenants/<id>` and all changes are reflected.

**Step 6: Test validation**

- Clear name → submit should be disabled
- Enter invalid email → error shown
- Enter invalid phone → error shown
- Enter comment > 500 chars → counter shows limit

**Step 7: Test duplicate phone**

If another tenant with a different phone exists, try to set the same phone. Confirm 409 error is shown and linked to phone field.

**Step 8: Test loading/error states**

- Refresh edit page; confirm skeleton appears briefly
- Stop backend, open edit page; confirm error state with retry button

**Step 9: Regression test create flow**

Open `/tenants/new`, create a new tenant, confirm success screen and list update.

**Step 10: Final lint and type check**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend
npm run lint && npx tsc --noEmit
```

Expected: no errors.

**Step 11: Update CHANGELOG.md**

Add an entry under today's date:

```markdown
## 2026-06-26

- Добавлена страница редактирования арендатора со всеми полями, включая email.
```

**Step 12: Commit changelog**

```bash
cd /Users/smirnowwwivan/Nambers/arenda-planform
git add CHANGELOG.md
git commit -m "docs: update changelog for tenant edit page"
```

---

## Execution Handoff

**Plan complete and saved to `docs/plans/2026-06-26-tenant-edit-implementation-plan.md`. Two execution options:**

**1. Subagent-Driven (this session)** — Dispatch fresh subagent per task, review between tasks, fast iteration. REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`.

**2. Parallel Session (separate)** — Open new session with `superpowers:executing-plans`, batch execution with checkpoints.

**Which approach would you like?**

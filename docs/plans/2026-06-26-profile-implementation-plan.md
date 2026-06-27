# Profile Pages Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the full profile area in `apps/frontend/app/(cabinet)/profile/` with all backend endpoints required by `docs/plans/2026-06-26-profile-design.md`, using vertical slices.

**Architecture:** Each vertical slice delivers one complete user scenario end-to-end: backend OpenAPI + handlers + service + repository, then frontend page + feature hooks + widget components. Backend uses clean architecture (domain/application/adapters/httpapi). Frontend follows existing FSD and React Query patterns.

**Tech Stack:** Go 1.24, PostgreSQL, sqlc, T-Kassa, OpenAPI; Next.js 16, React 19, TypeScript, Tailwind, HeroUI, React Query.

---

## Slice 1: Profile Overview + Logout

**Deliverable:** `/profile` page shows user card, subscription summary, menu links, and logout button.

### Task 1.1: Create profile feature skeleton

**Files:**
- Create: `apps/frontend/features/profile/api/keys.ts`
- Create: `apps/frontend/features/profile/api/hooks.ts`
- Modify: `apps/frontend/app/(cabinet)/profile/page.tsx`

**Step 1:** Add `profileKeys` to cache profile-specific mutations.

```ts
// apps/frontend/features/profile/api/keys.ts
export const profileKeys = {
  all: ['profile'] as const,
};
```

**Step 2:** Export `useUpdateMe` placeholder and `useLogoutCabinet` hook.

```ts
// apps/frontend/features/profile/api/hooks.ts
'use client';

import { useRouter } from 'next/navigation';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { authKeys } from '@/features/auth/api/keys';
import type { paths } from '@/shared/api/generated';

export function useLogoutCabinet() {
  const router = useRouter();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      return apiClient('/auth/logout', { method: 'POST' });
    },
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: authKeys.all });
      router.push('/login');
    },
  });
}
```

**Step 3:** Build the profile overview page.

```tsx
// apps/frontend/app/(cabinet)/profile/page.tsx
import { Metadata } from 'next';
import { ProfileOverview } from '@/widgets/profile/ui/ProfileOverview';

export const metadata: Metadata = {
  title: 'Профиль — Arenda Platform',
};

export default function ProfilePage() {
  return <ProfileOverview />;
}
```

**Step 4:** Create `ProfileOverview` widget.

```tsx
// apps/frontend/widgets/profile/ui/ProfileOverview.tsx
'use client';

import { useMe } from '@/features/auth/api/hooks';
import { useLogoutCabinet } from '@/features/profile/api/hooks';
import { ProfileMenu } from './ProfileMenu';
import { Button } from '@/shared/ui/button/Button';

export function ProfileOverview() {
  const { data: user, isLoading } = useMe();
  const logout = useLogoutCabinet();

  if (isLoading || !user) {
    return <div>Загрузка...</div>;
  }

  const displayName = [user.surname, user.name, user.patronymic].filter(Boolean).join(' ') || user.phone;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Профиль</h1>
      <div className="rounded-2xl border p-4 shadow-sm">
        <div className="text-lg font-medium">{displayName}</div>
        <div className="text-sm text-gray-500">{user.phone}</div>
        {user.email && <div className="text-sm text-gray-500">{user.email}</div>}
        {user.subscription && (
          <div className="mt-2 text-sm font-medium">
            Тариф: {user.subscription.tariff.name}
          </div>
        )}
      </div>
      <ProfileMenu />
      <Button onClick={() => logout.mutate()} disabled={logout.isPending}>
        Выйти
      </Button>
    </div>
  );
}
```

**Step 5:** Create `ProfileMenu` widget.

```tsx
// apps/frontend/widgets/profile/ui/ProfileMenu.tsx
import NextLink from 'next/link';

const items = [
  { href: '/profile/personal', label: 'Мои данные' },
  { href: '/profile/account', label: 'Аккаунт' },
  { href: '/profile/tariff', label: 'Тариф' },
  { href: '/profile/support', label: 'Поддержка' },
  { href: '/profile/info', label: 'Информация' },
];

export function ProfileMenu() {
  return (
    <nav className="space-y-2">
      {items.map((item) => (
        <NextLink
          key={item.href}
          href={item.href}
          className="block rounded-xl border p-4 hover:bg-gray-50"
        >
          {item.label}
        </NextLink>
      ))}
    </nav>
  );
}
```

**Verification:**
- Run `npm run build` in `apps/frontend`.
- Open `/profile`, see user card, menu, logout button.
- Click logout → redirect to `/login`.

---

## Slice 2: Personal Data Editing (PATCH /me)

**Deliverable:** User can edit name, surname, patronymic, email on `/profile/personal`.

### Task 2.1: Backend — add `UpdateUser` to identity domain

**Files:**
- Modify: `apps/backend/internal/identity/domain/user.go`
- Create: `apps/backend/internal/identity/domain/user_test.go`

**Step 1:** Write failing test.

```go
// apps/backend/internal/identity/domain/user_test.go
package domain

import "testing"

func TestUser_UpdatePersonalData(t *testing.T) {
    u := User{Phone: Phone("+79000000000"), Role: RoleOwner}
    err := u.UpdatePersonalData("Иван", "Иванов", "Иванович", "ivan@example.com")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if *u.Name != "Иван" || *u.Surname != "Иванов" || *u.Patronymic != "Иванович" || *u.Email != "ivan@example.com" {
        t.Fatalf("fields not updated")
    }
}
```

**Step 2:** Run test, expect FAIL (`UpdatePersonalData` undefined).

```bash
cd apps/backend && go test ./internal/identity/domain/... -run TestUser_UpdatePersonalData -v
```

**Step 3:** Implement method.

```go
// apps/backend/internal/identity/domain/user.go
func (u *User) UpdatePersonalData(name, surname, patronymic, email string) error {
    if email != "" {
        // lightweight email validation
        if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
            return errors.New("invalid email")
        }
    }
    u.Name = normalizeOptional(name)
    u.Surname = normalizeOptional(surname)
    u.Patronymic = normalizeOptional(patronymic)
    u.Email = normalizeOptional(email)
    return nil
}

func normalizeOptional(s string) *string {
    s = strings.TrimSpace(s)
    if s == "" {
        return nil
    }
    return &s
}
```

**Step 4:** Run test, expect PASS.

### Task 2.2: Backend — add repository method `UpdateUser`

**Files:**
- Modify: `apps/backend/db/queries/identity.sql`
- Modify: `apps/backend/internal/identity/adapters/postgres/repository.go`
- Modify: `apps/backend/internal/identity/application/ports.go`
- Create: `apps/backend/internal/identity/adapters/postgres/repository_test.go` (or extend existing)

**Step 1:** Add SQL query.

```sql
-- apps/backend/db/queries/identity.sql
-- name: UpdateUser :exec
UPDATE users
SET name = $2,
    surname = $3,
    patronymic = $4,
    email = $5,
    updated_at = now()
WHERE id = $1;
```

**Step 2:** Run sqlc generate.

```bash
cd apps/backend && make generate
```

**Step 3:** Add port method.

```go
// apps/backend/internal/identity/application/ports.go
Update(ctx context.Context, user domain.User) error
```

**Step 4:** Implement repository method.

```go
// apps/backend/internal/identity/adapters/postgres/repository.go
func (r *Repository) Update(ctx context.Context, user domain.User) error {
    return r.q.UpdateUser(ctx, postgres.UpdateUserParams{
        ID:         user.ID,
        Name:       pgutil.StringPtr(user.Name),
        Surname:    pgutil.StringPtr(user.Surname),
        Patronymic: pgutil.StringPtr(user.Patronymic),
        Email:      pgutil.StringPtr(user.Email),
    })
}
```

**Step 5:** Add failing test, make it pass.

### Task 2.3: Backend — add service method `UpdateMe`

**Files:**
- Modify: `apps/backend/internal/identity/application/service.go`
- Create: `apps/backend/internal/identity/application/service_test.go` (extend)

**Step 1:** Add `UpdateMe` method.

```go
// apps/backend/internal/identity/application/service.go
func (s *Service) UpdateMe(ctx context.Context, userID uuid.UUID, name, surname, patronymic, email string) (domain.User, error) {
    user, err := s.users.GetByID(ctx, userID)
    if err != nil {
        return domain.User{}, fmt.Errorf("get user: %w", err)
    }
    if err := user.UpdatePersonalData(name, surname, patronymic, email); err != nil {
        return domain.User{}, err
    }
    if err := s.users.Update(ctx, user); err != nil {
        return domain.User{}, fmt.Errorf("update user: %w", err)
    }
    return user, nil
}
```

**Step 2:** Write and run tests.

### Task 2.4: Backend — add PATCH /me endpoint

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`
- Modify: `apps/backend/internal/platform/httpapi/auth_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/server.go`

**Step 1:** Add schema and path to OpenAPI.

```yaml
# apps/backend/api/openapi/openapi.yaml
UpdateMeRequest:
  type: object
  properties:
    name: { type: string, nullable: true }
    surname: { type: string, nullable: true }
    patronymic: { type: string, nullable: true }
    email: { type: string, nullable: true, format: email }

/me:
  get:
    ...
  patch:
    operationId: updateMe
    tags: [Auth]
    security: [sessionCookie: []]
    requestBody:
      required: true
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/UpdateMeRequest'
    responses:
      '200':
        description: Updated user
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/MeResponse'
```

**Step 2:** Generate types.

```bash
cd apps/backend && make generate-api
```

**Step 3:** Add handler.

```go
// apps/backend/internal/platform/httpapi/auth_handlers.go
func (h *AuthHandlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    userID := UserIDFromContext(ctx)
    var req generated.UpdateMeRequest
    if err := decodeJSON(r, &req); err != nil { ... }
    user, err := h.service.UpdateMe(ctx, userID, ...)
    ...
}
```

**Step 4:** Register route in `server.go`.

```go
r.With(sessionMiddleware).Patch("/me", authHandlers.UpdateMe)
```

**Step 5:** Run backend tests.

```bash
cd apps/backend && go test ./...
```

### Task 2.5: Frontend — implement `useUpdateMe`

**Files:**
- Modify: `apps/frontend/features/profile/api/hooks.ts`

```ts
export function useUpdateMe() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: paths['/me']['patch']['requestBody']['content']['application/json']) => {
      return apiClient('/me', { method: 'PATCH', body: data });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.all });
    },
  });
}
```

### Task 2.6: Frontend — create `/profile/personal` page

**Files:**
- Create: `apps/frontend/app/(cabinet)/profile/personal/page.tsx`
- Create: `apps/frontend/widgets/profile/ui/PersonalDataForm.tsx`

**Step 1:** Implement form with FSD pattern.

```tsx
// apps/frontend/widgets/profile/ui/PersonalDataForm.tsx
'use client';

import { useState, useEffect } from 'react';
import { useMe } from '@/features/auth/api/hooks';
import { useUpdateMe } from '@/features/profile/api/hooks';
import { TextField } from '@/shared/ui/text-field/TextField';
import { Button } from '@/shared/ui/button/Button';

export function PersonalDataForm() {
  const { data: user, isLoading } = useMe();
  const update = useUpdateMe();
  const [surname, setSurname] = useState('');
  const [name, setName] = useState('');
  const [patronymic, setPatronymic] = useState('');
  const [email, setEmail] = useState('');

  useEffect(() => {
    if (user) {
      setSurname(user.surname ?? '');
      setName(user.name ?? '');
      setPatronymic(user.patronymic ?? '');
      setEmail(user.email ?? '');
    }
  }, [user]);

  if (isLoading || !user) return <div>Загрузка...</div>;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    update.mutate({
      name: name || null,
      surname: surname || null,
      patronymic: patronymic || null,
      email: email || null,
    });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <TextField label="Фамилия" value={surname} onChange={setSurname} />
      <TextField label="Имя" value={name} onChange={setName} />
      <TextField label="Отчество" value={patronymic} onChange={setPatronymic} />
      <TextField label="Email" value={email} onChange={setEmail} />
      <Button type="submit" disabled={update.isPending}>
        Сохранить
      </Button>
    </form>
  );
}
```

**Verification:**
- Run backend tests.
- Run frontend build.
- Open `/profile/personal`, edit fields, save, refresh — changes persist.

---

## Slice 3: Phone Change

**Deliverable:** User can change phone number on `/profile/account/phone` via SMS verification.

### Task 3.1: Backend — extend `sms_codes` for purpose

**Files:**
- Modify: `apps/backend/db/migrations/000001_init_schema.up.sql` (only if safe, otherwise new migration)
- Create: `apps/backend/db/migrations/0000XX_add_sms_code_purpose.up.sql`
- Modify: `apps/backend/db/queries/identity.sql`

**Step 1:** Add migration.

```sql
-- apps/backend/db/migrations/0000XX_add_sms_code_purpose.up.sql
ALTER TABLE sms_codes ADD COLUMN purpose VARCHAR(32) NOT NULL DEFAULT 'login';
CREATE INDEX idx_sms_codes_user_purpose ON sms_codes(user_id, purpose);
```

**Step 2:** Update SQL queries to filter by purpose.

```sql
-- name: CreateSMSCode :exec
INSERT INTO sms_codes (id, phone, code_hash, expires_at, user_id, purpose)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetSMSCodeByPhoneAndPurpose :one
SELECT * FROM sms_codes
WHERE phone = $1 AND purpose = $2
ORDER BY created_at DESC LIMIT 1;

-- name: DeleteSMSCodeByPhoneAndPurpose :exec
DELETE FROM sms_codes WHERE phone = $1 AND purpose = $2;
```

**Step 3:** Generate and update repository.

### Task 3.2: Backend — add phone change application logic

**Files:**
- Modify: `apps/backend/internal/identity/application/service.go`
- Create/extend: `apps/backend/internal/identity/application/service_test.go`

**Step 1:** Add `SendPhoneChangeCode` and `ChangePhone` methods.

```go
func (s *Service) SendPhoneChangeCode(ctx context.Context, userID uuid.UUID, newPhone string) error {
    // validate phone
    // check not current phone
    // check not used by other user
    // create sms code with purpose=phone_change
    // send via sms sender
}

func (s *Service) ChangePhone(ctx context.Context, userID uuid.UUID, newPhone, code string) (domain.User, error) {
    // verify code (purpose=phone_change)
    // update user phone
    // delete old sms codes for old and new phone
    // delete other sessions
    // return user
}
```

**Step 2:** Write tests, make them pass.

### Task 3.3: Backend — add handlers and OpenAPI

**Files:**
- Modify: `apps/backend/api/openapi/openapi.yaml`
- Modify: `apps/backend/internal/platform/httpapi/auth_handlers.go`
- Modify: `apps/backend/internal/platform/httpapi/server.go`

Add `POST /me/phone/send-code` and `POST /me/phone/change`.

### Task 3.4: Frontend — implement phone change page

**Files:**
- Create: `apps/frontend/app/(cabinet)/profile/account/page.tsx`
- Create: `apps/frontend/app/(cabinet)/profile/account/phone/page.tsx`
- Create: `apps/frontend/widgets/profile/ui/PhoneChangeForm.tsx`
- Modify: `apps/frontend/features/profile/api/hooks.ts`

Add hooks:

```ts
export function useChangePhoneSendCode() {
  return useMutation({
    mutationFn: (phone: string) =>
      apiClient('/me/phone/send-code', { method: 'POST', body: { phone } }),
  });
}

export function useChangePhone() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ phone, code }: { phone: string; code: string }) =>
      apiClient('/me/phone/change', { method: 'POST', body: { phone, code } }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: authKeys.all }),
  });
}
```

**Verification:**
- Change phone in UI with fake SMS provider.
- Verify `/me` returns new phone.
- Verify other sessions are revoked.

---

## Slice 4: Tariff Overview

**Deliverable:** `/profile/tariff` shows current tariff and controls.

### Task 4.1: Frontend — create tariff overview page

**Files:**
- Create: `apps/frontend/app/(cabinet)/profile/tariff/page.tsx`
- Create: `apps/frontend/widgets/profile/ui/TariffOverview.tsx`

Use existing billing hooks: `useSubscription`, `useToggleAutoRenew`, `useCancelSubscription`.

**Verification:**
- Open `/profile/tariff`, see current tariff.
- Toggle auto-renew works.
- Cancel subscription works.

---

## Slice 5: Tariff Change

**Deliverable:** `/profile/tariff/change` allows selecting a new tariff and paying/upgrading.

### Task 5.1: Frontend — create tariff change page

**Files:**
- Create: `apps/frontend/app/(cabinet)/profile/tariff/change/page.tsx`
- Create: `apps/frontend/app/(cabinet)/profile/tariff/change/success/page.tsx`
- Create: `apps/frontend/widgets/profile/ui/TariffChangeForm.tsx`

Use `useTariffs` and `useChangeTariff` from billing hooks.

**Verification:**
- Select upgrade tariff → redirect to T-Kassa payment URL.
- Select downgrade → success page shows effective date.

---

## Slice 6: Payment Methods

**Deliverable:** `/profile/tariff/payment-methods` lists, adds, activates, deletes cards.

### Task 6.1: Frontend — create payment methods pages

**Files:**
- Create: `apps/frontend/app/(cabinet)/profile/tariff/payment-methods/page.tsx`
- Create: `apps/frontend/app/(cabinet)/profile/tariff/payment-methods/add/page.tsx`
- Create: `apps/frontend/widgets/profile/ui/PaymentMethodList.tsx`

Use existing billing hooks.

**Verification:**
- List cards.
- Add card redirects to T-Kassa and returns.
- Activate/delete card works.

---

## Slice 7: Payments History

**Deliverable:** `/profile/tariff/payments` and `/profile/tariff/payments/[id]`.

### Task 7.1: Frontend — create payments pages

**Files:**
- Create: `apps/frontend/app/(cabinet)/profile/tariff/payments/page.tsx`
- Create: `apps/frontend/app/(cabinet)/profile/tariff/payments/[id]/page.tsx`
- Create: `apps/frontend/widgets/profile/ui/PaymentList.tsx`

Use `useSubscriptionPayments`.

**Verification:**
- List shows payments.
- Click opens detail.

---

## Slice 8: Support and Info Pages

**Deliverable:** `/profile/support`, `/profile/info`, `/profile/info/privacy`, `/profile/info/terms`.

### Task 8.1: Create support page

**Files:**
- Create: `apps/frontend/app/(cabinet)/profile/support/page.tsx`
- Create: `apps/frontend/content/support.md`

Use placeholder content; replace with user-provided text.

### Task 8.2: Create info pages

**Files:**
- Create: `apps/frontend/app/(cabinet)/profile/info/page.tsx`
- Create: `apps/frontend/app/(cabinet)/profile/info/privacy/page.tsx`
- Create: `apps/frontend/app/(cabinet)/profile/info/terms/page.tsx`
- Create: `apps/frontend/content/privacy.md`
- Create: `apps/frontend/content/terms.md`

Use `next-mdx-remote` or `remark`/`react-markdown` to render markdown.

**Verification:**
- All pages render without errors.
- Mobile navigation works.

---

## Final Verification

1. Backend: `cd apps/backend && go test ./...`
2. Frontend lint: `cd apps/frontend && npm run lint`
3. Frontend build: `cd apps/frontend && npm run build`
4. Manual smoke test of all profile routes.
5. Update `CHANGELOG.md` with profile feature entry.

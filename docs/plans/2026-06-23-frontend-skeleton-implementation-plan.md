# Frontend Skeleton Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Создать каркас фронтенда Arenda Platform на Next.js: сгенерировать TypeScript-типы из OpenAPI, настроить прокси к бэкенду через cookie, подготовить TanStack Query hooks для всех бэкенд-ресурсов. Базовые UI-компоненты и feature placeholder-компоненты реализуются следующим шагом.

**Architecture:** Feature-based структура в `apps/frontend/features/`. Shared слой содержит сгенерированные типы, API-клиент и провайдеры. Next.js API Route Handler в `app/api/[...path]/route.ts` проксирует запросы на бэкенд, сохраняя session cookie. Каждая feature экспортирует свои TanStack Query hooks.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, `@tanstack/react-query`, `openapi-typescript`, `clsx`.

---

### Task 1: Add dependencies

**Files:**
- Modify: `apps/frontend/package.json`
- Test: `cd apps/frontend && npm install` succeeds

**Step 1: Add dependencies**

```json
{
  "dependencies": {
    "next": "16.2.9",
    "react": "19.2.4",
    "react-dom": "19.2.4",
    "@tanstack/react-query": "^5.51.0",
    "clsx": "^2.1.1"
  },
  "devDependencies": {
    "@types/node": "^20",
    "@types/react": "^19",
    "@types/react-dom": "^19",
    "babel-plugin-react-compiler": "1.0.0",
    "eslint": "^9",
    "eslint-config-next": "16.2.9",
    "openapi-typescript": "^7.4.0",
    "typescript": "^5"
  }
}
```

**Step 2: Install**

Run: `cd apps/frontend && npm install`

Expected: `package-lock.json` обновлён, `node_modules` установлены.

**Step 3: Commit**

```bash
git add apps/frontend/package.json apps/frontend/package-lock.json
git commit -m "deps(frontend): add react-query, openapi-typescript, clsx"
```

---

### Task 2: Configure OpenAPI type generation

**Files:**
- Modify: `apps/frontend/package.json`
- Create: `apps/frontend/shared/api/generated.ts`

**Step 1: Add generate script**

```json
{
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start",
    "lint": "eslint",
    "generate:api": "openapi-typescript ../backend/api/openapi/openapi.yaml -o shared/api/generated.ts"
  }
}
```

**Step 2: Run generation**

Run: `cd apps/frontend && npm run generate:api`

Expected: файл `shared/api/generated.ts` создан и содержит типы `components`/`operations`/`paths`.

**Step 3: Commit**

```bash
git add apps/frontend/package.json apps/frontend/shared/api/generated.ts
git commit -m "feat(frontend): add openapi-typescript generation script and generated types"
```

---

### Task 3: Create shared API client

**Files:**
- Create: `apps/frontend/shared/api/errors.ts`
- Create: `apps/frontend/shared/api/client.ts`

**Step 1: Write error class**

```ts
// apps/frontend/shared/api/errors.ts
export class ApiError extends Error {
  constructor(
    public code: string,
    public detail: string,
    public requestId?: string,
    public status?: number,
  ) {
    super(detail);
    this.name = 'ApiError';
  }
}
```

**Step 2: Write fetch wrapper**

```ts
// apps/frontend/shared/api/client.ts
import { ApiError } from './errors';

export async function apiClient<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(`/api${path}`, {
    ...options,
    headers,
  });

  const requestId = response.headers.get('X-Request-ID') ?? undefined;

  if (!response.ok) {
    let code = 'unknown';
    let detail = `Request failed with status ${response.status}`;

    const contentType = response.headers.get('Content-Type');
    if (contentType?.includes('application/problem+json')) {
      const problem = (await response.json()) as Record<string, unknown>;
      code = String(problem.code ?? problem.type ?? code);
      detail = String(problem.detail ?? problem.title ?? detail);
    }

    throw new ApiError(code, detail, requestId, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}
```

**Step 3: Type-check**

Run: `cd apps/frontend && npx tsc --noEmit`

Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/shared/api/
git commit -m "feat(frontend): add shared api client with problem details error handling"
```

---

### Task 4: Setup TanStack Query provider

**Files:**
- Create: `apps/frontend/shared/providers/query-provider.tsx`
- Modify: `apps/frontend/app/layout.tsx`

**Step 1: Create provider**

```tsx
// apps/frontend/shared/providers/query-provider.tsx
'use client';

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ReactNode, useState } from 'react';

export function QueryProvider({ children }: { children: ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30 * 1000,
            refetchOnWindowFocus: false,
          },
        },
      }),
  );

  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}
```

**Step 2: Update root layout**

```tsx
// apps/frontend/app/layout.tsx
import type { Metadata } from 'next';
import { Geist, Geist_Mono } from 'next/font/google';
import { QueryProvider } from '@/shared/providers/query-provider';
import './globals.css';

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
});

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
});

export const metadata: Metadata = {
  title: 'Arenda Platform',
  description: 'Управление арендной недвижимостью',
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru" className={`${geistSans.variable} ${geistMono.variable}`}>
      <body>
        <QueryProvider>{children}</QueryProvider>
      </body>
    </html>
  );
}
```

**Step 3: Type-check**

Run: `cd apps/frontend && npx tsc --noEmit`

Expected: no errors.

**Step 4: Commit**

```bash
git add apps/frontend/shared/providers/query-provider.tsx apps/frontend/app/layout.tsx
git commit -m "feat(frontend): add tanstack query provider and wire into layout"
```

---

### Task 5: Create catch-all backend proxy

**Files:**
- Create: `apps/frontend/app/api/[...path]/route.ts`

**Step 1: Implement proxy**

```ts
// apps/frontend/app/api/[...path]/route.ts
import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';

const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';

async function handler(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const targetPath = `/${path.join('/')}`;
  const search = request.nextUrl.searchParams.toString();
  const targetUrl = `${BACKEND_URL}${targetPath}${search ? `?${search}` : ''}`;

  const cookieStore = await cookies();
  const cookieHeader = cookieStore.toString();

  const headers = new Headers(request.headers);
  headers.delete('host');
  if (cookieHeader) {
    headers.set('cookie', cookieHeader);
  }

  const response = await fetch(targetUrl, {
    method: request.method,
    headers,
    body: request.body,
    // @ts-expect-error Next.js streaming requirement
    duplex: 'half',
  });

  const responseHeaders = new Headers(response.headers);
  responseHeaders.delete('transfer-encoding');

  return new NextResponse(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders,
  });
}

export const GET = handler;
export const POST = handler;
export const PATCH = handler;
export const DELETE = handler;
```

**Step 2: Type-check**

Run: `cd apps/frontend && npx tsc --noEmit`

Expected: no errors.

**Step 3: Commit**

```bash
git add apps/frontend/app/api/\[...path\]/route.ts
git commit -m "feat(frontend): add catch-all proxy route to backend"
```

---

### Task 6: Create auth feature hooks

**Files:**
- Create: `apps/frontend/features/auth/api/keys.ts`
- Create: `apps/frontend/features/auth/api/hooks.ts`

**Step 1: Query keys**

```ts
// apps/frontend/features/auth/api/keys.ts
export const authKeys = {
  me: ['auth', 'me'] as const,
};
```

**Step 2: Hooks**

```ts
// apps/frontend/features/auth/api/hooks.ts
'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { authKeys } from './keys';
import type { components } from '@/shared/api/generated';

type MeResponse = components['schemas']['MeResponse'];
type SendPhoneCodeRequest = components['schemas']['SendPhoneCodeRequest'];
type VerifyPhoneCodeRequest = components['schemas']['VerifyPhoneCodeRequest'];

export function useMe() {
  return useQuery({
    queryKey: authKeys.me,
    queryFn: () => apiClient<MeResponse>('/me'),
    retry: false,
  });
}

export function useSendPhoneCode() {
  return useMutation({
    mutationFn: (data: SendPhoneCodeRequest) =>
      apiClient<void>('/auth/phone/send', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
  });
}

export function useVerifyPhoneCode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: VerifyPhoneCodeRequest) =>
      apiClient<MeResponse>('/auth/phone/verify', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.me });
    },
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiClient<void>('/auth/logout', { method: 'POST' }),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: authKeys.me });
    },
  });
}
```

**Step 3: Commit**

```bash
git add apps/frontend/features/auth/
git commit -m "feat(frontend): add auth feature hooks"
```

---

### Task 7: Create properties feature hooks

**Files:**
- Create: `apps/frontend/features/properties/api/keys.ts`
- Create: `apps/frontend/features/properties/api/hooks.ts`

**Step 1: Query keys**

```ts
// apps/frontend/features/properties/api/keys.ts
export const propertyKeys = {
  all: ['properties'] as const,
  detail: (id: string) => [...propertyKeys.all, id] as const,
};
```

**Step 2: Hooks**

```ts
// apps/frontend/features/properties/api/hooks.ts
'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { propertyKeys } from './keys';
import type { components } from '@/shared/api/generated';

type PropertyResponse = components['schemas']['PropertyResponse'];
type PropertyCreateRequest = components['schemas']['PropertyCreateRequest'];
type PropertyUpdateRequest = components['schemas']['PropertyUpdateRequest'];

export function useProperties() {
  return useQuery({
    queryKey: propertyKeys.all,
    queryFn: () => apiClient<PropertyResponse[]>('/properties'),
  });
}

export function useProperty(id: string) {
  return useQuery({
    queryKey: propertyKeys.detail(id),
    queryFn: () => apiClient<PropertyResponse>(`/properties/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateProperty() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: PropertyCreateRequest) =>
      apiClient<PropertyResponse>('/properties', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.all });
    },
  });
}

export function useUpdateProperty() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: PropertyUpdateRequest }) =>
      apiClient<PropertyResponse>(`/properties/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.all });
      queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

export function useArchiveProperty() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<PropertyResponse>(`/properties/${id}/archive`, {
        method: 'POST',
      }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.all });
      queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

export function useUnarchiveProperty() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<PropertyResponse>(`/properties/${id}/unarchive`, {
        method: 'POST',
      }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.all });
      queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}
```

**Step 3: Commit**

```bash
git add apps/frontend/features/properties/
git commit -m "feat(frontend): add properties feature hooks"
```

---

### Task 8: Create leases feature hooks

**Files:**
- Create: `apps/frontend/features/leases/api/keys.ts`
- Create: `apps/frontend/features/leases/api/hooks.ts`

**Step 1: Query keys**

```ts
// apps/frontend/features/leases/api/keys.ts
export const leaseKeys = {
  all: ['leases'] as const,
  detail: (id: string) => [...leaseKeys.all, id] as const,
  reminders: (id: string) => [...leaseKeys.all, id, 'reminders'] as const,
};
```

**Step 2: Hooks**

```ts
// apps/frontend/features/leases/api/hooks.ts
'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { leaseKeys } from './keys';
import type { components } from '@/shared/api/generated';

type LeaseResponse = components['schemas']['LeaseResponse'];
type LeaseCreateRequest = components['schemas']['LeaseCreateRequest'];
type LeaseUpdateRequest = components['schemas']['LeaseUpdateRequest'];
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useLeases() {
  return useQuery({
    queryKey: leaseKeys.all,
    queryFn: () => apiClient<LeaseResponse[]>('/leases'),
  });
}

export function useLease(id: string) {
  return useQuery({
    queryKey: leaseKeys.detail(id),
    queryFn: () => apiClient<LeaseResponse>(`/leases/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateLease() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: LeaseCreateRequest) =>
      apiClient<LeaseResponse>('/leases', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
    },
  });
}

export function useUpdateLease() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: LeaseUpdateRequest }) =>
      apiClient<LeaseResponse>(`/leases/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      queryClient.invalidateQueries({ queryKey: leaseKeys.detail(id) });
    },
  });
}

export function useCompleteLease() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<LeaseResponse>(`/leases/${id}/complete`, {
        method: 'POST',
      }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      queryClient.invalidateQueries({ queryKey: leaseKeys.detail(id) });
    },
  });
}

export function useLeaseReminders(id: string) {
  return useQuery({
    queryKey: leaseKeys.reminders(id),
    queryFn: () =>
      apiClient<RemindersResponse>(`/leases/${id}/reminders`),
    enabled: Boolean(id),
  });
}

export function useCreateLeaseReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: ReminderCreateRequest }) =>
      apiClient<ReminderResponse>(`/leases/${id}/reminders`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.reminders(id) });
    },
  });
}
```

**Step 3: Commit**

```bash
git add apps/frontend/features/leases/
git commit -m "feat(frontend): add leases feature hooks"
```

---

### Task 9: Create operations feature hooks

**Files:**
- Create: `apps/frontend/features/operations/api/keys.ts`
- Create: `apps/frontend/features/operations/api/hooks.ts`

**Step 1: Query keys**

```ts
// apps/frontend/features/operations/api/keys.ts
export const operationKeys = {
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'operations'] as const,
  detail: (id: string) => ['operations', id] as const,
  reminders: (propertyId: string, operationId: string) =>
    [...operationKeys.detail(operationId), 'reminders', propertyId] as const,
};
```

**Step 2: Hooks**

```ts
// apps/frontend/features/operations/api/hooks.ts
'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { operationKeys } from './keys';
import type { components } from '@/shared/api/generated';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationCreateRequest = components['schemas']['OperationCreateRequest'];
type OperationUpdateRequest = components['schemas']['OperationUpdateRequest'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useOperationsByProperty(propertyId: string) {
  return useQuery({
    queryKey: operationKeys.byProperty(propertyId),
    queryFn: () =>
      apiClient<OperationsResponse>(`/properties/${propertyId}/operations`),
    enabled: Boolean(propertyId),
  });
}

export function useOperation(id: string) {
  return useQuery({
    queryKey: operationKeys.detail(id),
    queryFn: () => apiClient<OperationResponse>(`/operations/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      propertyId,
      data,
    }: {
      propertyId: string;
      data: OperationCreateRequest;
    }) =>
      apiClient<OperationResponse>(`/properties/${propertyId}/operations`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: operationKeys.byProperty(propertyId),
      });
    },
  });
}

export function useUpdateOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: OperationUpdateRequest }) =>
      apiClient<OperationResponse>(`/operations/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
    },
  });
}

export function useDeleteOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, propertyId }: { id: string; propertyId: string }) =>
      apiClient<void>(`/operations/${id}`, { method: 'DELETE' }),
    onSuccess: (_, { propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: operationKeys.byProperty(propertyId),
      });
    },
  });
}

export function useOperationReminders(
  propertyId: string,
  operationId: string,
) {
  return useQuery({
    queryKey: operationKeys.reminders(propertyId, operationId),
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/properties/${propertyId}/operations/${operationId}/reminders`,
      ),
    enabled: Boolean(propertyId) && Boolean(operationId),
  });
}

export function useCreateOperationReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      propertyId,
      operationId,
      data,
    }: {
      propertyId: string;
      operationId: string;
      data: ReminderCreateRequest;
    }) =>
      apiClient<ReminderResponse>(
        `/properties/${propertyId}/operations/${operationId}/reminders`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      ),
    onSuccess: (_, { propertyId, operationId }) => {
      queryClient.invalidateQueries({
        queryKey: operationKeys.reminders(propertyId, operationId),
      });
    },
  });
}
```

**Step 3: Commit**

```bash
git add apps/frontend/features/operations/
git commit -m "feat(frontend): add operations feature hooks"
```

---

### Task 10: Create recurring-operations feature hooks

**Files:**
- Create: `apps/frontend/features/recurring-operations/api/keys.ts`
- Create: `apps/frontend/features/recurring-operations/api/hooks.ts`

**Step 1: Query keys**

```ts
// apps/frontend/features/recurring-operations/api/keys.ts
export const recurringOperationKeys = {
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
  detail: (id: string) => ['recurring-operations', id] as const,
  reminders: (propertyId: string, recurringOperationId: string) =>
    [
      ...recurringOperationKeys.detail(recurringOperationId),
      'reminders',
      propertyId,
    ] as const,
};
```

**Step 2: Hooks**

```ts
// apps/frontend/features/recurring-operations/api/hooks.ts
'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { recurringOperationKeys } from './keys';
import type { components } from '@/shared/api/generated';

type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];
type RecurringOperationCreateRequest =
  components['schemas']['RecurringOperationCreateRequest'];
type RecurringOperationUpdateRequest =
  components['schemas']['RecurringOperationUpdateRequest'];
type RecurringOperationsResponse =
  components['schemas']['RecurringOperationsResponse'];
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useRecurringOperationsByProperty(propertyId: string) {
  return useQuery({
    queryKey: recurringOperationKeys.byProperty(propertyId),
    queryFn: () =>
      apiClient<RecurringOperationsResponse>(
        `/properties/${propertyId}/recurring-operations`,
      ),
    enabled: Boolean(propertyId),
  });
}

export function useRecurringOperation(id: string) {
  return useQuery({
    queryKey: recurringOperationKeys.detail(id),
    queryFn: () =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateRecurringOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      propertyId,
      data,
    }: {
      propertyId: string;
      data: RecurringOperationCreateRequest;
    }) =>
      apiClient<RecurringOperationResponse>(
        `/properties/${propertyId}/recurring-operations`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      ),
    onSuccess: (_, { propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.byProperty(propertyId),
      });
    },
  });
}

export function useUpdateRecurringOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: RecurringOperationUpdateRequest;
    }) =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function usePauseRecurringOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}/pause`, {
        method: 'POST',
      }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function useResumeRecurringOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<RecurringOperationResponse>(
        `/recurring-operations/${id}/resume`,
        {
          method: 'POST',
        },
      ),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function useRecurringOperationReminders(
  propertyId: string,
  recurringOperationId: string,
) {
  return useQuery({
    queryKey: recurringOperationKeys.reminders(propertyId, recurringOperationId),
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/properties/${propertyId}/recurring-operations/${recurringOperationId}/reminders`,
      ),
    enabled: Boolean(propertyId) && Boolean(recurringOperationId),
  });
}

export function useCreateRecurringOperationReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      propertyId,
      recurringOperationId,
      data,
    }: {
      propertyId: string;
      recurringOperationId: string;
      data: ReminderCreateRequest;
    }) =>
      apiClient<ReminderResponse>(
        `/properties/${propertyId}/recurring-operations/${recurringOperationId}/reminders`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      ),
    onSuccess: (_, { propertyId, recurringOperationId }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.reminders(
          propertyId,
          recurringOperationId,
        ),
      });
    },
  });
}
```

**Step 3: Commit**

```bash
git add apps/frontend/features/recurring-operations/
git commit -m "feat(frontend): add recurring-operations feature hooks"
```

---

### Task 11: Create reminders feature hooks

**Files:**
- Create: `apps/frontend/features/reminders/api/keys.ts`
- Create: `apps/frontend/features/reminders/api/hooks.ts`

**Step 1: Query keys**

```ts
// apps/frontend/features/reminders/api/keys.ts
export const reminderKeys = {
  all: ['reminders'] as const,
  detail: (id: string) => [...reminderKeys.all, id] as const,
};
```

**Step 2: Hooks**

```ts
// apps/frontend/features/reminders/api/hooks.ts
'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { reminderKeys } from './keys';
import type { components } from '@/shared/api/generated';

type ReminderResponse = components['schemas']['ReminderResponse'];
type ReminderUpdateRequest = components['schemas']['ReminderUpdateRequest'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useReminders(limit = 100, offset = 0) {
  return useQuery({
    queryKey: reminderKeys.all,
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/reminders?limit=${limit}&offset=${offset}`,
      ),
  });
}

export function useReminder(id: string) {
  return useQuery({
    queryKey: reminderKeys.detail(id),
    queryFn: () => apiClient<ReminderResponse>(`/reminders/${id}`),
    enabled: Boolean(id),
  });
}

export function useUpdateReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: ReminderUpdateRequest;
    }) =>
      apiClient<ReminderResponse>(`/reminders/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: reminderKeys.all });
      queryClient.invalidateQueries({ queryKey: reminderKeys.detail(id) });
    },
  });
}

export function useDeleteReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<void>(`/reminders/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: reminderKeys.all });
    },
  });
}
```

**Step 3: Commit**

```bash
git add apps/frontend/features/reminders/
git commit -m "feat(frontend): add reminders feature hooks"
```

---

### Task 12: Create tenant-contacts feature hooks

**Files:**
- Create: `apps/frontend/features/tenant-contacts/api/keys.ts`
- Create: `apps/frontend/features/tenant-contacts/api/hooks.ts`

**Step 1: Query keys**

```ts
// apps/frontend/features/tenant-contacts/api/keys.ts
export const tenantContactKeys = {
  all: ['tenant-contacts'] as const,
  detail: (id: string) => [...tenantContactKeys.all, id] as const,
};
```

**Step 2: Hooks**

```ts
// apps/frontend/features/tenant-contacts/api/hooks.ts
'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { tenantContactKeys } from './keys';
import type { components } from '@/shared/api/generated';

type TenantContactResponse = components['schemas']['TenantContactResponse'];
type TenantContactCreateRequest =
  components['schemas']['TenantContactCreateRequest'];
type TenantContactUpdateRequest =
  components['schemas']['TenantContactUpdateRequest'];
type TenantContactsResponse = components['schemas']['TenantContactsResponse'];

export function useTenantContacts() {
  return useQuery({
    queryKey: tenantContactKeys.all,
    queryFn: () => apiClient<TenantContactsResponse>('/tenant-contacts'),
  });
}

export function useTenantContact(id: string) {
  return useQuery({
    queryKey: tenantContactKeys.detail(id),
    queryFn: () => apiClient<TenantContactResponse>(`/tenant-contacts/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateTenantContact() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: TenantContactCreateRequest) =>
      apiClient<TenantContactResponse>('/tenant-contacts', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: tenantContactKeys.all });
    },
  });
}

export function useUpdateTenantContact() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: TenantContactUpdateRequest;
    }) =>
      apiClient<TenantContactResponse>(`/tenant-contacts/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: tenantContactKeys.all });
      queryClient.invalidateQueries({ queryKey: tenantContactKeys.detail(id) });
    },
  });
}
```

**Step 3: Commit**

```bash
git add apps/frontend/features/tenant-contacts/
git commit -m "feat(frontend): add tenant-contacts feature hooks"
```

---

### Task 13: Create billing feature hooks

**Files:**
- Create: `apps/frontend/features/billing/api/keys.ts`
- Create: `apps/frontend/features/billing/api/hooks.ts`

**Step 1: Query keys**

```ts
// apps/frontend/features/billing/api/keys.ts
export const billingKeys = {
  tariffs: ['billing', 'tariffs'] as const,
  subscription: ['billing', 'subscription'] as const,
};
```

**Step 2: Hooks**

```ts
// apps/frontend/features/billing/api/hooks.ts
'use client';

import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { billingKeys } from './keys';
import type { components } from '@/shared/api/generated';

type TariffsResponse = components['schemas']['TariffsResponse'];
type Subscription = components['schemas']['Subscription'];

export function useTariffs() {
  return useQuery({
    queryKey: billingKeys.tariffs,
    queryFn: () => apiClient<TariffsResponse>('/tariffs'),
  });
}

export function useSubscription() {
  return useQuery({
    queryKey: billingKeys.subscription,
    queryFn: () => apiClient<Subscription>('/subscription'),
  });
}
```

> Примечание: мутации подписки, способов оплаты и платежей добавляются при реализации страниц billing; для каркаса достаточно чтения.

**Step 3: Commit**

```bash
git add apps/frontend/features/billing/
git commit -m "feat(frontend): add billing feature read hooks"
```

---

### Task 14: Create shared UI components

> **Status:** skipped for this iteration; will be implemented as the next step after the core skeleton is verified.

**Files:**
- Create: `apps/frontend/shared/ui/button/button.tsx`
- Create: `apps/frontend/shared/ui/button/button.module.css`
- Create: `apps/frontend/shared/ui/input/input.tsx`
- Create: `apps/frontend/shared/ui/input/input.module.css`
- Create: `apps/frontend/shared/ui/label/label.tsx`
- Create: `apps/frontend/shared/ui/label/label.module.css`
- Create: `apps/frontend/shared/ui/card/card.tsx`
- Create: `apps/frontend/shared/ui/card/card.module.css`
- Create: `apps/frontend/shared/ui/spinner/spinner.tsx`
- Create: `apps/frontend/shared/ui/spinner/spinner.module.css`
- Create: `apps/frontend/shared/ui/error-message/error-message.tsx`
- Create: `apps/frontend/shared/ui/error-message/error-message.module.css`

**Step 1: Button component**

```tsx
// apps/frontend/shared/ui/button/button.tsx
import { ButtonHTMLAttributes, ReactNode } from 'react';
import styles from './button.module.css';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary';
}

export function Button({
  children,
  variant = 'primary',
  className = '',
  ...props
}: ButtonProps) {
  return (
    <button
      className={`${styles.button} ${styles[variant]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}
```

```css
/* apps/frontend/shared/ui/button/button.module.css */
.button {
  padding: 0.5rem 1rem;
  border: 1px solid transparent;
  border-radius: 0.375rem;
  cursor: pointer;
  font-size: 1rem;
}

.primary {
  background: #111;
  color: #fff;
}

.secondary {
  background: #fff;
  color: #111;
  border-color: #111;
}

.button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
```

**Step 2: Input component**

```tsx
// apps/frontend/shared/ui/input/input.tsx
import { InputHTMLAttributes, forwardRef } from 'react';
import styles from './input.module.css';

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className = '', ...props }, ref) {
    return <input ref={ref} className={`${styles.input} ${className}`} {...props} />;
  },
);
```

```css
/* apps/frontend/shared/ui/input/input.module.css */
.input {
  padding: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 0.375rem;
  font-size: 1rem;
  width: 100%;
}
```

**Step 3: Label component**

```tsx
// apps/frontend/shared/ui/label/label.tsx
import { LabelHTMLAttributes, ReactNode } from 'react';
import styles from './label.module.css';

export function Label({
  children,
  className = '',
  ...props
}: LabelHTMLAttributes<HTMLLabelElement> & { children: ReactNode }) {
  return (
    <label className={`${styles.label} ${className}`} {...props}>
      {children}
    </label>
  );
}
```

```css
/* apps/frontend/shared/ui/label/label.module.css */
.label {
  display: block;
  margin-bottom: 0.25rem;
  font-size: 0.875rem;
  font-weight: 500;
}
```

**Step 4: Card component**

```tsx
// apps/frontend/shared/ui/card/card.tsx
import { ReactNode } from 'react';
import styles from './card.module.css';

export function Card({
  children,
  className = '',
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={`${styles.card} ${className}`}>{children}</div>;
}
```

```css
/* apps/frontend/shared/ui/card/card.module.css */
.card {
  padding: 1rem;
  border: 1px solid #e5e5e5;
  border-radius: 0.5rem;
}
```

**Step 5: Spinner component**

```tsx
// apps/frontend/shared/ui/spinner/spinner.tsx
import styles from './spinner.module.css';

export function Spinner({ className = '' }: { className?: string }) {
  return <span className={`${styles.spinner} ${className}`} aria-label="Loading" />;
}
```

```css
/* apps/frontend/shared/ui/spinner/spinner.module.css */
.spinner {
  display: inline-block;
  width: 1rem;
  height: 1rem;
  border: 2px solid #ccc;
  border-top-color: #111;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
```

**Step 6: ErrorMessage component**

```tsx
// apps/frontend/shared/ui/error-message/error-message.tsx
import styles from './error-message.module.css';

export function ErrorMessage({ message }: { message: string }) {
  return <p className={styles.error}>{message}</p>;
}
```

```css
/* apps/frontend/shared/ui/error-message/error-message.module.css */
.error {
  color: #c00;
  font-size: 0.875rem;
}
```

**Step 7: Type-check**

Run: `cd apps/frontend && npx tsc --noEmit`

Expected: no errors.

**Step 8: Commit**

```bash
git add apps/frontend/shared/ui/
git commit -m "feat(frontend): add shared base ui components"
```

---

### Task 15: Create feature placeholder components

> **Status:** skipped for this iteration; will be implemented alongside shared UI components as the next step.

**Files:**
- Create: `apps/frontend/features/properties/components/property-card/property-card.tsx`
- Create: `apps/frontend/features/properties/components/property-card/property-card.module.css`
- Create: `apps/frontend/features/leases/components/lease-list/lease-list.tsx`
- Create: `apps/frontend/features/leases/components/lease-list/lease-list.module.css`
- Create: `apps/frontend/features/operations/components/operation-row/operation-row.tsx`
- Create: `apps/frontend/features/operations/components/operation-row/operation-row.module.css`
- Create: `apps/frontend/features/recurring-operations/components/recurring-operation-item/recurring-operation-item.tsx`
- Create: `apps/frontend/features/recurring-operations/components/recurring-operation-item/recurring-operation-item.module.css`
- Create: `apps/frontend/features/reminders/components/reminder-row/reminder-row.tsx`
- Create: `apps/frontend/features/reminders/components/reminder-row/reminder-row.module.css`
- Create: `apps/frontend/features/tenant-contacts/components/tenant-contact-card/tenant-contact-card.tsx`
- Create: `apps/frontend/features/tenant-contacts/components/tenant-contact-card/tenant-contact-card.module.css`
- Create: `apps/frontend/features/billing/components/subscription-info/subscription-info.tsx`
- Create: `apps/frontend/features/billing/components/subscription-info/subscription-info.module.css`

**Step 1: PropertyCard**

```tsx
// apps/frontend/features/properties/components/property-card/property-card.tsx
import { Card } from '@/shared/ui/card/card';
import styles from './property-card.module.css';
import type { components } from '@/shared/api/generated';

type Property = components['schemas']['PropertyResponse'];

export function PropertyCard({ property }: { property: Property }) {
  return (
    <Card className={styles.card}>
      <h3>{property.name}</h3>
      <p>{property.address}</p>
    </Card>
  );
}
```

```css
/* apps/frontend/features/properties/components/property-card/property-card.module.css */
.card h3 {
  margin: 0 0 0.25rem;
}

.card p {
  margin: 0;
  color: #666;
}
```

**Step 2: LeaseList**

```tsx
// apps/frontend/features/leases/components/lease-list/lease-list.tsx
import styles from './lease-list.module.css';
import type { components } from '@/shared/api/generated';

type Lease = components['schemas']['LeaseResponse'];

export function LeaseList({ leases }: { leases: Lease[] }) {
  return (
    <ul className={styles.list}>
      {leases.map((lease) => (
        <li key={lease.id} className={styles.item}>
          {lease.startDate} — {lease.endDate ?? '...'}
        </li>
      ))}
    </ul>
  );
}
```

```css
/* apps/frontend/features/leases/components/lease-list/lease-list.module.css */
.list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.item {
  padding: 0.5rem 0;
  border-bottom: 1px solid #eee;
}
```

**Step 3: OperationRow**

```tsx
// apps/frontend/features/operations/components/operation-row/operation-row.tsx
import styles from './operation-row.module.css';
import type { components } from '@/shared/api/generated';

type Operation = components['schemas']['OperationResponse'];

export function OperationRow({ operation }: { operation: Operation }) {
  return (
    <div className={styles.row}>
      <span>{operation.date}</span>
      <span>{operation.amount}</span>
      <span>{operation.category}</span>
    </div>
  );
}
```

```css
/* apps/frontend/features/operations/components/operation-row/operation-row.module.css */
.row {
  display: flex;
  gap: 1rem;
  padding: 0.5rem 0;
}
```

**Step 4: RecurringOperationItem**

```tsx
// apps/frontend/features/recurring-operations/components/recurring-operation-item/recurring-operation-item.tsx
import styles from './recurring-operation-item.module.css';
import type { components } from '@/shared/api/generated';

type RecurringOperation = components['schemas']['RecurringOperationResponse'];

export function RecurringOperationItem({
  operation,
}: {
  operation: RecurringOperation;
}) {
  return (
    <div className={styles.item}>
      <span>{operation.dayOfMonth} числа</span>
      <span>{operation.amount}</span>
    </div>
  );
}
```

```css
/* apps/frontend/features/recurring-operations/components/recurring-operation-item/recurring-operation-item.module.css */
.item {
  display: flex;
  gap: 1rem;
  padding: 0.5rem 0;
}
```

**Step 5: ReminderRow**

```tsx
// apps/frontend/features/reminders/components/reminder-row/reminder-row.tsx
import styles from './reminder-row.module.css';
import type { components } from '@/shared/api/generated';

type Reminder = components['schemas']['ReminderResponse'];

export function ReminderRow({ reminder }: { reminder: Reminder }) {
  return (
    <div className={styles.row}>
      <span>{reminder.scheduledAt}</span>
      <span>{reminder.status}</span>
    </div>
  );
}
```

```css
/* apps/frontend/features/reminders/components/reminder-row/reminder-row.module.css */
.row {
  display: flex;
  gap: 1rem;
  padding: 0.5rem 0;
}
```

**Step 6: TenantContactCard**

```tsx
// apps/frontend/features/tenant-contacts/components/tenant-contact-card/tenant-contact-card.tsx
import { Card } from '@/shared/ui/card/card';
import styles from './tenant-contact-card.module.css';
import type { components } from '@/shared/api/generated';

type TenantContact = components['schemas']['TenantContactResponse'];

export function TenantContactCard({ contact }: { contact: TenantContact }) {
  return (
    <Card className={styles.card}>
      <h3>{contact.name}</h3>
      <p>{contact.phone}</p>
    </Card>
  );
}
```

```css
/* apps/frontend/features/tenant-contacts/components/tenant-contact-card/tenant-contact-card.module.css */
.card h3 {
  margin: 0 0 0.25rem;
}

.card p {
  margin: 0;
  color: #666;
}
```

**Step 7: SubscriptionInfo**

```tsx
// apps/frontend/features/billing/components/subscription-info/subscription-info.tsx
import styles from './subscription-info.module.css';
import type { components } from '@/shared/api/generated';

type Subscription = components['schemas']['Subscription'];

export function SubscriptionInfo({
  subscription,
}: {
  subscription: Subscription;
}) {
  return (
    <div className={styles.info}>
      <p>Тариф: {subscription.tariffName}</p>
      <p>Статус: {subscription.status}</p>
    </div>
  );
}
```

```css
/* apps/frontend/features/billing/components/subscription-info/subscription-info.module.css */
.info p {
  margin: 0 0 0.25rem;
}
```

**Step 8: Type-check**

Run: `cd apps/frontend && npx tsc --noEmit`

Expected: no errors.

**Step 9: Commit**

```bash
git add apps/frontend/features/*/components/
git commit -m "feat(frontend): add feature placeholder components"
```

---

### Task 16: Verify build and lint

**Files:**
- All files above

**Step 1: Run type check**

Run: `cd apps/frontend && npx tsc --noEmit`

Expected: no errors.

**Step 2: Run lint**

Run: `cd apps/frontend && npm run lint`

Expected: no errors.

**Step 3: Run build**

Run: `cd apps/frontend && npm run build`

Expected: build succeeds.

**Step 4: Final commit**

```bash
git add CHANGELOG.md docs/plans/2026-06-23-frontend-skeleton-design.md docs/plans/2026-06-23-frontend-skeleton-implementation-plan.md
git commit -m "docs: update frontend skeleton docs to reflect delivered scope"
```

---

## Done Criteria

- `npm run generate:api` создаёт актуальные типы из `openapi.yaml`.
- `npx tsc --noEmit`, `npm run lint` и `npm run build` проходят без ошибок.
- Все feature-папки содержат `api/keys.ts` и `api/hooks.ts`.
- Прокси-роут пересылает запросы на бэкенд.
- QueryProvider оборачивает приложение.
- Shared UI-компоненты и feature placeholder-компоненты реализуются следующим шагом.
- QueryProvider оборачивает приложение.

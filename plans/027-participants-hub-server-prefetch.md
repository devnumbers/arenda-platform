# 027 — Хаб «Участники»: серверный префетч с тарифным гейтом (§15)

- **Status**: TODO
- **Commit**: 2a37f5bc5
- **Severity**: MEDIUM
- **Category**: Loading policy (DESIGN.md §15; гейты экрана повторяются серверно)
- **Estimated scope**: 5 файлов (queries.ts, hooks.ts, новый архетип, page.tsx, новый loading.tsx)

## Problem

`/participants` — хаб-лента раздела (D1–D4 #1313: «/participants — сам
список»), раздел закрыт auth-гейтом (§15, матчер `proxy.ts`). Серверного
префетча нет: `app/(screens)/participants/page.tsx:12-19` рендерит
`<ParticipantsListScreen initialOrder={…}/>` без Suspense и границы;
`loading.tsx` на уровне хаба нет.

В отличие от хаба «Объектов» (план 026) SSR-кадр здесь честный: чтение
списка гейтится подпиской (`participants-list-screen.tsx:120-133`),

```tsx
const subscriptionQuery = useSubscription();
const isPaid = subscriptionQuery.data
  ? isPaidTariff(subscriptionQuery.data.tariff.name)
  : true;
const subscriptionResolved =
  subscriptionQuery.isSuccess || subscriptionQuery.isError;

const participantsQuery = useParticipantsList({
  enabled: subscriptionResolved && isPaid,
});
```

и до её разрешения экран рисует нейтральный скелетон (`:245-252`,
«без флэша пустоты»). Но планка §15 не выполняется: холодный вход даёт
скелетон в первом кадре, данные — только после гидратации и двух
последовательных фетчей (подписка → список). Эндпоинт списка за тарифным
гейтом (ADR 0064, 402 на basic) — серверная раскладка обязана повторить
гейт: на basic список не запрашивать вовсе (сейчас клиент и не запрашивает —
«обречённого 402 нет», `:115-119`).

## Target

Гейт-паттерн канона — `app/(screens)/properties/[id]/page.tsx:39-45`
(«деталь решает читаемость, зависимые запросы — только после её успеха»):

```tsx
async function prefetchPropertyScreen(queryClient: QueryClient, id: string): Promise<void> {
  await queryClient.prefetchQuery(propertyDetailQueryOptions({ id, transport: serverApiClient }));
  if (queryClient.getQueryState(propertyKeys.detail(id))?.status !== 'success') {
    return;
  }
  /* зависимые префетчи — после успеха гейта */
}
```

Тот же состав для участников (гейт — подписка, зависимый — список):

```tsx
/* app/(screens)/participants/page.tsx — target */
import { Suspense, type JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantsListLoading, ParticipantsListScreen, parseParticipantsListOrderParams } from '@/widgets/participants';
import { participantsListQueryOptions } from '@/features/participants';
import { subscriptionQueryOptions } from '@/features/subscription';
import { isPaidTariff } from '@/entities/user';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/** Раздел «Участники» — единая страница списка (D1–D4 #1313): хаб-карточки
 * и «Объекты пользователей» снесены, /participants — сам список. Состояние
 * сортировки живёт в адресе (?order=, #785) — стартовое значение парсится
 * здесь, на сервере. Серверный префетч (§15, тикет #1422): гейт подписки
 * повторяется серверно (ADR 0064) — список раскладывается только на
 * платном тарифе; сбой подписки — платный фолбэк экрана (деградация
 * видима на списке), basic остаётся на апселле без запроса списка. */
export const metadata: Metadata = {
  title: 'Участники — Рентли',
};

async function prefetchParticipantsScreen(queryClient: QueryClient): Promise<void> {
  await queryClient.prefetchQuery(subscriptionQueryOptions(serverApiClient));
  if (queryClient.getQueryState(subscriptionKeys.subscription)?.status !== 'success') {
    return;
  }
  const subscription = queryClient.getQueryData(subscriptionKeys.subscription);
  if (subscription ? isPaidTariff(subscription.tariff.name) : true) {
    void queryClient.prefetchQuery(participantsListQueryOptions(serverApiClient));
  }
}

export default async function ParticipantsRoutePage({
  searchParams,
}: PageProps<'/participants'>) {
  const resolved = await searchParams;
  const initialOrder = parseParticipantsListOrderParams(resolved.order);

  return (
    <Suspense fallback={<ParticipantsListLoading />}>
      <ServerPrefetchBoundary prefetch={prefetchParticipantsScreen}>
        <ParticipantsListScreen initialOrder={initialOrder} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
```

`subscriptionKeys` импортируется из `@/shared/api/query-keys` (проверить
экспорт `subscriptionKeys.subscription` — используется в
`features/subscription/api/queries.ts:33`).

`loading.tsx` — тот же кадр:

```tsx
/* app/(screens)/participants/loading.tsx — новый файл */
import type { JSX } from 'react';
import { ParticipantsListLoading } from '@/widgets/participants';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же
 * кадр, что и fallback Suspense-границы страницы.
 */
export default function Loading(): JSX.Element {
  return <ParticipantsListLoading />;
}
```

## Repo conventions to follow

- Фабрика списка: сейчас ключ+fetch живут в хуке
  (`features/participants/api/hooks.ts:42-49`) — §15 требует фабрику в
  двойном модуле. Образец — `participantQueryOptions`
  (`features/participants/api/queries.ts:28-41`).
- `subscriptionQueryOptions(transport)` — `features/subscription/api/queries.ts:26-33`.
- `isPaidTariff` — чистая функция без 'use client':
  `entities/user/lib/is-paid-tariff.ts:1` (реэкспорт `@/entities/user`).
- Архетип: `participants-list-skeletons.tsx:16` (`ParticipantsListSkeleton`).
  Чип в скелетоне — реальный с дефолтным лейблом, прецедент —
  `payment-overdue-global-screen`/`operations-loading.tsx:38-41` («Чипы —
  дефолтные лейблы… без сдвига геометрии»).

## Steps

1. `features/participants/api/queries.ts` — добавить фабрику (перенести
   fetch из хука; баррель `features/participants/index.ts` экспортирует
   хуки и фабрику detail поимённо — добавить туда
   `export { participantsListQueryOptions } from './api/queries';` если
   barrel не перевы экспортирует `./api` целиком):
   ```ts
   /** Чистый fetch списка участников — общее горло хука и серверного
    * префетча #887 (список целиком, без пагинации; ADR 0064: за тарифным
    * гейтом — раскладывается только после успеха подписки). */
   export async function fetchParticipantsList(
     transport: ApiTransport = apiClient,
   ): Promise<Participant[]> {
     const response = await transport<ParticipantsResponse>('/participants');
     return response.items.map(mapParticipant);
   }

   /** Опции списка участников (канон #887): один источник ключ+fetch для
    * хука и серверного префетча. */
   export function participantsListQueryOptions({
     transport = apiClient,
   }: {
     readonly transport?: ApiTransport;
   } = {}): UseQueryOptions<Participant[], ApiError, Participant[], ReturnType<typeof participantsKeys.list>> {
     return queryOptions({
       queryKey: participantsKeys.list(),
       queryFn: () => fetchParticipantsList(transport),
     });
   }
   ```
   (`ParticipantsResponse` — существующий тип импорта хука, `hooks.ts:44`.)
2. `features/participants/api/hooks.ts` — `useParticipantsList` читает
   фабрику (enabled остаётся опцией):
   ```ts
   return useQuery({
     ...participantsListQueryOptions(),
     enabled: options.enabled,
   });
   ```
3. Новый архетип `widgets/participants/ui/participants-list-loading.tsx`:
   ```tsx
   import type { JSX } from 'react';
   import { ArrowDown } from '@/shared/assets/icons';
   import { ChipButton } from '@/shared/ui/design';
   import { ParticipantsListSkeleton } from './participants-list-skeletons';

   /**
    * Route-loading архетип списка участников (#609, тикет #1422): чип
    * сортировки — реальный с дефолтным лейблом «Имя» (без onClick — фаза
    * загрузки не действует; канон чипов в скелетонах, operations-loading
    * #609), тело — скелетон списка (паритет #604). Тот же кадр рисует
    * fallback Suspense-границы страницы и loading.tsx.
    */
   export function ParticipantsListLoading(): JSX.Element {
     return (
       <>
         <div className="mt-4 px-6">
           <ChipButton trailingIcon={<ArrowDown />}>Имя</ChipButton>
         </div>
         <ParticipantsListSkeleton />
       </>
     );
   }
   ```
   Добавить в `widgets/participants/index.ts`:
   `export { ParticipantsListLoading } from './ui/participants-list-loading';`
4. `app/(screens)/participants/page.tsx` — заменить на Target-код.
5. `app/(screens)/participants/loading.tsx` — создать.
6. Юнит-тест фабрики — по образцу `features/properties/api/queries.test.ts`
   (ключ и fetch-функция).

## Boundaries

- Do NOT трогать клиентские гейты экрана (`!subscriptionResolved` →
  нейтральный скелетон, `:245-259`) — они остаются тёплым поведением.
- Do NOT раскладывать список при неуспешной подписке или basic-тарифе —
  402-волна на basic запрещена (ADR 0064).
- Do NOT префетчить детализацию/приглашения — только первый кадр списка.
- Do NOT менять `ParticipantsListSkeleton`.
- Если код не совпал с цитатами — STOP и доложить.

## Verification

- **Mechanical**: `make frontend-test` — зелёный (включая новый тест
  фабрики); `make frontend-e2e TESTS='-g Участники'` (Docker) — зелёный.
- **Feel check**: F5 на `/participants` на платном тарифе — первый кадр
  со списком; на basic — апселл без сетевого запроса списка (Network);
  при медленном бэке — кадр `ParticipantsListLoading` из стрима, идентичный
  `loading.tsx`; hover-переход из шита «Еще» — серверных запросов до
  настоящего перехода нет.
- **Done when**: холодный вход на платном тарифе рисует данные в первом
  кадре; basic не порождает запроса списка; кадры Suspense-fallback и
  loading.tsx неразличимы.

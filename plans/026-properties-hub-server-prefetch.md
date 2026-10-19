# 026 — Хаб «Объекты»: серверный префетч холодного входа (§15)

- **Status**: TODO
- **Commit**: 2a37f5bc5
- **Severity**: HIGH
- **Category**: Loading policy (DESIGN.md §15, §14 «Первый кадр с данными»)
- **Estimated scope**: 3 файла (page.tsx, новый loading.tsx, index.ts)

## Problem

Хаб «Объекты» — единственная хаб-лента без серверного префетча холодного
входа. Канон §15: «холодный вход (прямая ссылка, F5, запуск PWA) на
контентных страницах рисует первый кадр с данными»; шесть sibling-хабов
(контакты, уведомления, операции, платежи, задачи, история) обёрнуты в
`ServerPrefetchBoundary`, хаб объектов — нет. Тёплые переходы живы:
`HubPrefetchProvider` греет /properties (`hub-prefetch-provider.tsx:44-46`),
пробел — именно холодный вход.

Текущий код — `app/(screens)/properties/page.tsx:9-16` (целиком):

```tsx
export default async function PropertiesRoutePage({
    searchParams,
}: PageProps<'/properties'>) {
    const resolved = await searchParams;
    const initialSort = parseSortFromParams(resolved);

    return <PropertiesPage initialSort={initialSort}/>;
}
```

Ни Suspense, ни ServerPrefetchBoundary; `loading.tsx` на уровне хаба нет
(в `app/(screens)/properties/` он есть только у `search/`).

Виджет `widgets/properties/ui/PropertiesPage.tsx` — `'use client'`, но
сервер-рендерится, и фазы гейтятся на `isLoading` (`:211`):

```tsx
{isLoading && <PropertiesLoading/>}
```

В react-query v5 на сервере фетч не стартует (подписка обсервера —
клиентский эффект), `fetchStatus` остаётся `idle`, поэтому
`isLoading = isPending && isFetching` даёт **false** (исходник:
`node_modules/@tanstack/query-core/build/modern/queryObserver.js:302`).
В SSR-кадре `data === undefined`, `isLoading === false`, `isError === false`
→ `isEmpty = true` (`PropertiesPage.tsx:119`) → серверный HTML рисует
**пустое состояние** (`:215-217`) и прячет пилюлю поиска (`:193`).
Пользователь с объектами на холодном входе видит три кадра: «пустота» (SSR)
→ скелетон (гидратация) → список. Прямое нарушение и «нуля blank frame»
(§14), и планки §15.

## Target

Кадр платежного хаба — эталон (`app/(screens)/payments/page.tsx`, приведён
целиком в «Repo conventions»). Тот же состав для объектов:

```tsx
/* app/(screens)/properties/page.tsx — target */
import { Suspense, type JSX } from 'react';
import type { Metadata } from 'next';
import { PropertiesLoading, PropertiesPage, parseSortFromParams } from '@/widgets/properties';
import { propertiesListQueryOptions } from '@/features/properties';
import { subscriptionQueryOptions } from '@/features/subscription';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

export const metadata: Metadata = {
    title: 'Объекты — Рентли',
    description: 'Список объектов',
};

/**
 * Хаб «Объекты» — серверный префетч холодного входа (§15, тикет #1422):
 * граница внутри Suspense — hover-префетч динамики рендерит RSC только до
 * неё. Подписка — второй запрос первого кадра (кнопки создания: canAdd,
 * isActionLoading); список и meta-проекции suspended_shared/today — один
 * ключ /properties (fetchProperties кладёт полный ответ, select экрана
 * проецирует строки).
 */
export default async function PropertiesRoutePage({
    searchParams,
}: PageProps<'/properties'>) {
    const resolved = await searchParams;
    const initialSort = parseSortFromParams(resolved);

    return (
        <Suspense fallback={<PropertiesLoading />}>
            <ServerPrefetchBoundary
                prefetch={(queryClient) => {
                    void queryClient.prefetchQuery(propertiesListQueryOptions(serverApiClient));
                    void queryClient.prefetchQuery(subscriptionQueryOptions(serverApiClient));
                }}
            >
                <PropertiesPage initialSort={initialSort} />
            </ServerPrefetchBoundary>
        </Suspense>
    );
}
```

`loading.tsx` — тот же кадр, что fallback (#609):

```tsx
/* app/(screens)/properties/loading.tsx — новый файл */
import type { JSX } from 'react';
import { PropertiesLoading } from '@/widgets/properties';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же
 * кадр, что и fallback Suspense-границы страницы.
 */
export default function Loading(): JSX.Element {
    return <PropertiesLoading />;
}
```

## Repo conventions to follow

Эталон — `app/(screens)/payments/page.tsx` (целиком, копировать стиль
докблока и раскладки):

```tsx
export default function PaymentsRoutePage(): JSX.Element {
    return (
        <Suspense fallback={<PaymentsLoading />}>
            <ServerPrefetchBoundary
                prefetch={(queryClient) => {
                    void queryClient.prefetchQuery(globalPaymentsFeedQueryOptions(serverApiClient));
                    void queryClient.prefetchQuery(globalPaymentObjectsQueryOptions({ transport: serverApiClient }));
                }}
            >
                <PaymentsGlobalScreen />
            </ServerPrefetchBoundary>
        </Suspense>
    );
}
```

- Фабрики (§15 «Один источник ключ+fetch»): `propertiesListQueryOptions(transport)` —
  `features/properties/api/queries.ts:86-93`; `subscriptionQueryOptions(transport)` —
  `features/subscription/api/queries.ts:26-33` (404 → null через `nullOn404`, #768).
- Срез — дефолтный (§15): первый ключ списков без фильтров; сортировка
  экрана живёт в URL и перечитывается клиентом — префетчится дефолт.
- Архетип `PropertiesLoading` уже есть: `widgets/properties/ui/PropertiesLoading.tsx:29-38`
  (карточки-плитки, детерминированные `skeletonRowWidths`), хром в нём не
  рисуется — как у всех хабовых архетипов (#609).
- `usePropertiesWithMeta` (`PropertiesPage.tsx:72`) читает **тот же ключ**
  /properties — отдельного префетча не нужно.

## Steps

1. `widgets/properties/index.ts` — добавить экспорт архетипа (сейчас
   `PropertiesLoading` из index не выходит, виджет импортирует его
   относительно):
   ```ts
   export { PropertiesLoading } from './ui/PropertiesLoading';
   ```
2. `app/(screens)/properties/page.tsx` — заменить содержимое на Target-код
   выше.
3. `app/(screens)/properties/loading.tsx` — создать (Target-код выше).
4. Прогнать `make frontend-test`; e2e по хабу объектов
   (`make frontend-e2e TESTS='-g Объекты'` — Docker) — регресс-гейт.

## Boundaries

- Do NOT трогать внутренние гейты `PropertiesPage` (`isLoading`/`isError`/
  `isEmpty` на `:119-132`, `:211-217`) — они остаются поведением
  перечитывания/ошибки на тёплом клиенте; план чинит только холодный вход.
- Do NOT переносить `loading.tsx` в `search/` или `archive/` — у них свои.
- Do NOT добавлять префетч третьих запросов (meta/properties — один ключ;
  subscription — второй и последний).
- Do NOT менять `PropertiesLoading` (геометрия канонична).
- Если код на месте исполнения не совпал с цитатами (дрейф от коммита) —
  STOP и доложить.

## Verification

- **Mechanical**: `make frontend-test` — зелёный; `make frontend-e2e
  TESTS='-g Объекты'` (Docker) — зелёный; типография теста «данные в
  первом кадре» — `curl` холодного `/properties` не содержит
  `properties-empty`/CTA пустоты до гидратации (сравнить с do-кадром).
- **Feel check**: F5 на `/properties` при наличии объектов — первый кадр
  рисует список без пустоты и без скелетона; замедлить бэк в DevTools
  (Network throttling) — короткий кадр `PropertiesLoading` из стрима, затем
  данные без сдвига; hover по разделу в навигации — RSC рендерится до
  границы, серверных запросов впустую нет (Network: запросы объектов идут
  только при настоящем переходе/холодном входе).
- **Done when**: холодный вход с данными не показывает ни пустого
  состояния, ни скелетона в типичных условиях; при медленном бэке кадр
  скелетона идентичен `loading.tsx`.

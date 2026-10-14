# 028 — Хаб «Профиль»: серверный префетч useMe (§15)

- **Status**: TODO
- **Commit**: 2a37f5bc5
- **Severity**: LOW
- **Category**: Loading policy (DESIGN.md §15; хаб-меню уточняет скоуп)
- **Estimated scope**: 2 файла (page.tsx, ProfileHub.tsx экспорт скелетона)

## Problem

`/profile` читает `useMe` (`widgets/profile/ui/ProfileHub.tsx:134`) —
запрос первого кадра без серверного префетча. SSR-кадр честный (скелетон):
гейт на данных, не на `isLoading` (`:164-166`),

```tsx
if (me === undefined) {
  return <ProfileHubSkeleton />;
}
```

— на сервере `me === undefined`, скелетон рисуется с первого кадра, флэша
нет. Пробел один: планка §15 «холодный вход без скелетона в типичных
условиях» не выполняется (данные — после гидратации и фетча /me). Хаб
редко входит холодным (навигация «Еще»/сайдбар тёплая), поэтому LOW.

Точка маршрута — `app/(screens)/profile/page.tsx:16-27`: TopNav рендерится
на маршруте, `ProfileHub` — внутри `PageContent`. Скелетон хаба
(`ProfileHubSkeleton`, `ProfileHub.tsx:99-119`) — внутренний, не
экспортируется.

Побочный выигрыш: `SyncHydrationBoundary` кладёт /me в кэш синхронно в
рендере — UserButton сайдбара (читает `useMe` над страницей) и хаб
перестают расходиться на холодном входе; это тот самый кейс, ради которого
#887 отказался от библиотечного `HydrationBoundary` (DESIGN.md §15,
React #418).

## Target

```tsx
/* app/(screens)/profile/page.tsx — target */
import { Suspense } from 'react';
import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { meQueryOptions } from '@/features/auth';
import { ProfileHub, ProfileHubSkeleton } from '@/widgets/profile';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

export const metadata: Metadata = {
  title: 'Профиль — Рентли',
};

/** Хаб профиля в новом дизайне (карта #591, тикет #592; Figma 1903-38340 /
 * 1786-31288): шапка — анатомия подэкрана без «Назад» (решение владельца
 * 02.10, образец — /profile/devices): крылья только на ПК, заголовок
 * «Профиль» 16/18 в баре по центру на всех ярусах. Ниже ПК вход в профиль —
 * TabBar («Еще») и крылья других хабов, на ПК — крыло UserButton.
 * Подэкраны /profile* ссылаются на него фолбэком «Назад».
 * Серверный префетч useMe (§15, тикет #1422): граница внутри Suspense
 * поверх контента, хром маршрута вне фазы загрузки; fallback — тот же
 * скелетон, что держит хаб в pending (тот же кадр, #609). */
export default function ProfilePage() {
  return (
    <>
      <TopNav hideWingsBelowDesktop>
        <TopNavTitle title="Профиль" />
      </TopNav>
      <PageContent>
        <Suspense fallback={<ProfileHubSkeleton />}>
          <ServerPrefetchBoundary
            prefetch={(queryClient) => {
              void queryClient.prefetchQuery(meQueryOptions(serverApiClient));
            }}
          >
            <ProfileHub />
          </ServerPrefetchBoundary>
        </Suspense>
      </PageContent>
    </>
  );
}
```

## Repo conventions to follow

- `meQueryOptions(transport)` — `features/auth/api/queries.ts:23-33`
  (фабрика §15 уже есть, сервером не использовалась).
- Граница поверх контента при хроме на маршруте — профиль повторяет
  раскладку `profile/notifications` (SubScreenShell на маршруте
  `page.tsx:18`, скелетон внутри — единственный гибрид канона, аудит #877).
- Скелетон хаба уже с паритетом (#604, ходка #729 — «Выйти» в скелетоне) —
  не менять.

## Steps

1. `widgets/profile/ui/ProfileHub.tsx` — экспортировать скелетон:
   `function ProfileHubSkeleton` → `export function ProfileHubSkeleton`
   (`:99`); добавить в `widgets/profile/index.ts`:
   `export { ProfileHub, ProfileHubSkeleton } from './ui/ProfileHub';`
   (строка `:16` правится).
2. `app/(screens)/profile/page.tsx` — заменить на Target-код.
3. Прогнать `make frontend-test`; e2e профиля, если покрывают хаб
   (`make frontend-e2e TESTS='-g Профиль'` — Docker).

## Boundaries

- Do NOT трогать внутренние гейты `ProfileHub` (`isError` → карточка
  повтора `:153-162`, `me === undefined` → скелетон) — тёплое поведение.
- Do NOT добавлять `loading.tsx` — серверной async-работы на маршруте нет,
  внешнего loading-окна у хаба не существует (канон §7: экран, собирающий
  шапку сам, внешний loading.tsx не имеет).
- Do NOT префетчить photoBuster/прочие производные — только /me.
- Если код не совпал с цитатами — STOP и доложить.

## Verification

- **Mechanical**: `make frontend-test` — зелёный.
- **Feel check**: F5 на `/profile` — первый кадр с именем/телефоном без
  скелетона; при медленном бэке — скелетон хаба из стрима (тот же кадр,
  что текущий SSR); на ПК UserButton сайдбара и хаб согласованы с первого
  кадра.
- **Done when**: холодный вход /profile в типичных условиях без
  скелетона; деградация при медленном бэке — канонный скелетон.

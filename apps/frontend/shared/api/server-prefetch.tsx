import 'server-only';
import type { JSX, ReactNode } from 'react';
import type { QueryClient } from '@tanstack/react-query';
import { SyncHydrationBoundary } from '@/shared/providers/sync-hydration-boundary';
import {
  createServerQueryClient,
  dehydrateServerPrefetch,
  dropUnhydratableQueries,
  settleServerPrefetch,
} from './server-prefetch-core';

/**
 * RSC-граница серверного префетча #887 — третья часть канона (транспорт —
 * server-client.ts, ядро — server-prefetch-core.ts). Живёт в page.tsx
 * ВНУТРИ <Suspense>: hover-префетч динамического маршрута рендерит RSC
 * только до границы, поэтому серверные запросы не гоняются впустую на
 * наводке (решение research #863).
 *
 * Контракт `prefetch` — раскладка запросов страницы через
 * queryOptions-фабрики с серверным транспортом. Очередь задаёт сам вызов:
 * гейт-запрос (деталь объекта на странице объекта) ждётся await'ом, и
 * зависимые секции раскладываются только после его успеха — серверный
 * повтор гарда #769, на нечитаемом объекте секции не простреливают 404.
 * Остальные запросы раскладываются без await — параллельно.
 *
 * После раскладки запросы оседают в бюджете; неуспевшие и упавшие
 * снимаются с hydration. Успешные hydrate'ятся в клиентский кэш до
 * рендера виджета: холодный вход рисует первый кадр с данными, живой
 * SSE-кадр, успевший между ответом сервера и гидратацией, не затирается
 * (гарды dataUpdatedAt/dehydratedAt в hydrate — фиксируется тестом).
 * Граница — SyncHydrationBoundary: HydrationBoundary откладывает запросы,
 * уже существующие в кэше, в useEffect, а на сервере эффекты не идут —
 * Shell-запросы (useMe пилюли профиля) оставались пустыми в SSR-HTML.
 */
export async function ServerPrefetchBoundary({
  prefetch,
  children,
}: Readonly<{
  prefetch: (queryClient: QueryClient) => Promise<void> | void;
  children: ReactNode;
}>): Promise<JSX.Element> {
  const queryClient = createServerQueryClient();
  await prefetch(queryClient);
  await settleServerPrefetch(queryClient);
  dropUnhydratableQueries(queryClient);

  return (
    <SyncHydrationBoundary state={dehydrateServerPrefetch(queryClient)}>
      {children}
    </SyncHydrationBoundary>
  );
}

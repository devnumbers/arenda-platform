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
 * БЕЗ Suspense-обёртки (аменда #1605): граница ждёт данные до рендера, и
 * на маршрутах без route-level loading.tsx HTML приходит однофлушным —
 * второй флуш ($RC/$RB) не возникает, окно гонки гидратации React 19.2
 * (react/react#37014: batched Suspense reveal + синхронный ре-рендер в
 * окне дают ~300 мс дубль контента) закрыто по построению. Стриминг
 * остаётся только на маршрутах с loading.tsx — там сегмент стримит
 * route-граница (скелетон §7), окно теоретически живо, но эмпирически
 * не материализуется; сама граница о способе рендера не знает.
 * Hover-префетч динамического маршрута рендерит RSC до границы, и
 * серверные запросы на наводке не запускаются впустую (решение research
 * #863): запросы стартуют только на реальном рендере маршрута.
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

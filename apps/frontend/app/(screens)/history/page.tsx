import type { Metadata } from 'next';
import { HistoryFeedScreen } from '@/widgets/history';
import { historyFeedQueryOptions, historyFiltersQueryOptions } from '@/features/history';
import { meQueryOptions } from '@/features/auth';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

/** Лента «История действий» (карта #704, тикеты #709–#711): общая лента
 * по всем доступным объектам на едином хроме группы (screens) — оболочка
 * ScreenLayout, вход — кебаб шапки «Ваших участников» (#843, решение
 * владельца 24.09). Новые снизу (мессенджер), догрузка старых — скроллом
 * вверх; поиск (#710), фильтры (#711), действия участника (#712) и
 * переходы строк (#713) — свои тикеты карты. Фильтры живут в
 * query-параметрах, поэтому клиентский экран со useSearchParams стоит за
 * Suspense-границей — требование App Router (прецедент /operations).
 * Фолбэк границы — с хромом экрана (#1281, карта #1324): скелетон без
 * шапки «прыгал» в хром при приходе контента. */
export const metadata: Metadata = {
  title: 'История действий — Рентли',
  description: 'Лента действий по объектам',
};

export default function HistoryRoutePage() {
  return (
<ServerPrefetchBoundary
  prefetch={(queryClient) => {
    // Дефолтный срез ленты — пустой скоуп (#709); опции шита фильтров
    // и свой актор («(Вы)» у своей шапки) — в первом кадре всегда.
    void queryClient.prefetchInfiniteQuery(historyFeedQueryOptions({
      transport: serverApiClient,
    }));
    void queryClient.prefetchQuery(historyFiltersQueryOptions({ transport: serverApiClient }));
    void queryClient.prefetchQuery(meQueryOptions(serverApiClient));
  }}
>
  <HistoryFeedScreen />
</ServerPrefetchBoundary>
  );
}

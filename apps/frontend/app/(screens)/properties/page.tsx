import type { Metadata } from 'next';
import { PropertiesPage, parseSortFromParams } from '@/widgets/properties';
import { propertiesListQueryOptions } from '@/features/properties';
import { subscriptionQueryOptions } from '@/features/subscription';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

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

    // Полный кадр хаба (план 044): хром вне фазы загрузки (§7),
    // сегментный loading.tsx снесён — его архетип стримился первым
    // кадром и детям (/properties/[id] получал скелетон списка).
    return (
  <ServerPrefetchBoundary
      prefetch={(queryClient) => {
          void queryClient.prefetchQuery(propertiesListQueryOptions(serverApiClient));
          void queryClient.prefetchQuery(subscriptionQueryOptions(serverApiClient));
      }}
  >
      <PropertiesPage initialSort={initialSort} />
  </ServerPrefetchBoundary>
    );
}

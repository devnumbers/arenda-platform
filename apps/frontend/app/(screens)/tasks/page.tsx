import type { Metadata } from 'next';
import { globalTasksQueryOptions, parseTasksSortParams } from '@/features/tasks';
import { propertiesListQueryOptions } from '@/features/properties';
import { TasksFeedScreen } from '@/widgets/tasks';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

/**
 * Глобальная лента «Задачи» (#523, Figma 1733-27411/1726-86913/1733-92349):
 * топ-уровень группы (screens) рядом с объектами; вход — пункт «Задачи» в
 * меню профиля (решение 1 #522). Фильтр по объекту (#524) живёт в
 * query-параметрах, поэтому клиентский экран со useSearchParams стоит за
 * Suspense-границей — требование App Router. Сортировка тоже живёт в
 * адресе (?sort=&order=, #785) — стартовое значение парсится здесь, на
 * сервере.
 */
export const metadata: Metadata = {
  title: 'Задачи — Рентли',
};

export default async function TasksRoutePage({ searchParams }: PageProps<'/tasks'>) {
  const resolved = await searchParams;
  const initialSort = parseTasksSortParams(resolved.sort, resolved.order);

  return (
<ServerPrefetchBoundary
  prefetch={(queryClient) => {
    // Дефолтный срез ленты — бакеты без фильтра, EMPTY_TASKS_FEED_FILTER
    // экрана; справочник объектов — для фильтра и групп.
    void queryClient.prefetchQuery(globalTasksQueryOptions({
      completed: false,
      propertyIds: [],
      withoutProperty: false,
      transport: serverApiClient,
    }));
    void queryClient.prefetchQuery(globalTasksQueryOptions({
      completed: true,
      propertyIds: [],
      withoutProperty: false,
      transport: serverApiClient,
    }));
    void queryClient.prefetchQuery(propertiesListQueryOptions(serverApiClient));
  }}
>
  <TasksFeedScreen initialSort={initialSort} />
</ServerPrefetchBoundary>
  );
}

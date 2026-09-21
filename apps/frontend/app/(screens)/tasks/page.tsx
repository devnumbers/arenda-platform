import { Suspense } from 'react';
import type { Metadata } from 'next';
import { parseTasksSortParams } from '@/features/tasks';
import { TasksFeedScreen, TasksLoading } from '@/widgets/tasks';

/**
 * Глобальная лента «Задачи» (#523, Figma 1733-27411/1726-86913/1733-92349):
 * топ-уровень группы (screens) рядом с объектами; вход — пункт «Задачи» в
 * меню профиля (решение 1 #522). Фильтр по объекту (#524) живёт в
 * query-параметрах, поэтому клиентский экран со useSearchParams стоит за
 * Suspense-границей — требование App Router. Сортировка тоже живёт в адресе
 * (?sort=&order=, #785) — стартовое значение парсится здесь, на сервере.
 */
export const metadata: Metadata = {
  title: 'Задачи — Рентли',
};

type TasksRoutePageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function TasksRoutePage({ searchParams }: TasksRoutePageProps) {
  const resolved = searchParams ? await searchParams : {};
  const initialSort = parseTasksSortParams(resolved.sort, resolved.order);

  return (
    <Suspense fallback={<TasksLoading />}>
      <TasksFeedScreen initialSort={initialSort} />
    </Suspense>
  );
}

import type { Metadata } from 'next';
import { parseTasksSortParams } from '@/features/tasks';
import { TasksOfPropertyScreen } from '@/widgets/tasks';

/** Экран «Задачи объекта» (#499): секции «Просроченные / Сегодня / Завтра /
 * даты / Без даты / Выполненные», сортировка, кебаб «Удалить все
 * выполненные». Выбор сортировки живёт в адресе (?sort=&order=, #785) —
 * стартовое значение парсится здесь, на сервере. Оболочка новых экранов —
 * из layout группы (screens). */
export const metadata: Metadata = {
  title: 'Задачи объекта — Рентли',
};

type TasksRoutePageProps = {
  params: Promise<{ id: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function TasksRoutePage({ params, searchParams }: TasksRoutePageProps) {
  const { id } = await params;
  const resolved = searchParams ? await searchParams : {};
  const initialSort = parseTasksSortParams(resolved.sort, resolved.order);

  return <TasksOfPropertyScreen propertyId={id} initialSort={initialSort} />;
}

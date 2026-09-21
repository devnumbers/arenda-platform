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

export default async function TasksRoutePage({
  params,
  searchParams,
}: PageProps<'/properties/[id]/tasks'>) {
  const { id } = await params;
  const resolved = await searchParams;
  const initialSort = parseTasksSortParams(resolved.sort, resolved.order);

  return <TasksOfPropertyScreen propertyId={id} initialSort={initialSort} />;
}

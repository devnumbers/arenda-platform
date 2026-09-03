import type { Metadata } from 'next';
import { TasksOfPropertyScreen } from '@/widgets/tasks';

/** Экран «Задачи объекта» (#499): секции «Просроченные / Сегодня / Завтра /
 * даты / Без даты / Выполненные», сортировка, кебаб «Удалить все
 * выполненные». Оболочка новых экранов — из layout группы (screens). */
export const metadata: Metadata = {
  title: 'Задачи объекта — Рентли',
};

type TasksRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function TasksRoutePage({ params }: TasksRoutePageProps) {
  const { id } = await params;

  return <TasksOfPropertyScreen propertyId={id} />;
}

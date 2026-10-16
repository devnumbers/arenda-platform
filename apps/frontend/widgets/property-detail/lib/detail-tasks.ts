import type { Task, TasksPage } from '@/entities/task';
import type { IsoDate } from '@/shared/lib/calendar';

/**
 * Отбор задач для секции «Задачи» детали объекта (тикет #589, правка
 * владельца 11.09): максимум 3 строки — сначала просрочки от старейшей,
 * затем по ближайшей дате; недатированные не выводятся вовсе. Тон строк —
 * канон features/tasks (taskRowTone).
 */
export function propertyDetailTasks(page: TasksPage): ReadonlyArray<Task> {
  const dated = page.items.flatMap((task) =>
    task.dueDate === null ? [] : [{ task, dueDate: task.dueDate }],
  );
  const byDueDate = (
    a: { readonly dueDate: IsoDate },
    b: { readonly dueDate: IsoDate },
  ): number => (a.dueDate < b.dueDate ? -1 : a.dueDate > b.dueDate ? 1 : 0);
  const overdue = dated
    .filter((entry) => entry.task.status === 'overdue')
    .sort(byDueDate);
  const upcoming = dated
    .filter((entry) => entry.task.status !== 'overdue')
    .sort(byDueDate);
  return [...overdue, ...upcoming]
    .slice(0, 3)
    .map((entry) => entry.task);
}

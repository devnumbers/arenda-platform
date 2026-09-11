import type { TasksPage } from '@/entities/task';
import { pluralize } from '@/shared/lib/pluralize';

/**
 * Сводка секции «Задачи» на детали объекта (тикет #589). Макетом блока
 * не покрыт (в кадрах карты #583 секции «Задачи» нет) — сверить на
 * приёмке: строка «N активных задач» + число просроченных, тап ведёт в
 * задачи объекта. Заголовок — по серверному total (страница ограничена
 * лимитом листинга), просроченные — по статусам загруженных задач
 * (серверный статус, ADR 0048).
 */
export type PropertyTasksSummary = {
  /** «1 активная задача» / «2 активные задачи» / «5 активных задач». */
  readonly title: string;
  /** Задачи со статусом overdue. */
  readonly overdueCount: number;
};

export function propertyTasksSummary(page: TasksPage): PropertyTasksSummary | null {
  if (page.total === 0) {
    return null;
  }
  const overdueCount = page.items.filter((task) => task.status === 'overdue').length;
  return {
    title: `${page.total} ${pluralize(page.total, 'активная задача', 'активные задачи', 'активных задач')}`,
    overdueCount,
  };
}

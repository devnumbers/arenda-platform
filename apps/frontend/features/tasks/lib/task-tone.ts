import type { Task } from '@/entities/task';
import { addDays, type IsoDate } from '@/shared/lib/calendar';
import type { TaskSectionKind } from './tasks-list';

/** Цвет акцента строки времени: просрочка красным, сегодня/завтра синим,
 * остальные серым (Figma 1531:12784 — варианты Task Button Miss/Today/
 * Default); выполненные строки — серые. */
export type TaskRowTone = 'danger' | 'primary' | 'muted';

/** Тон строки задачи — канон строк задач, единственное место правила:
 * деталь объекта (#589) считает тон по задаче и «сегодня владельца»,
 * экраны задач (#499/#523) — по типу секции. */
export function taskRowTone(task: Task, today: IsoDate): TaskRowTone {
  if (task.status === 'overdue') {
    return 'danger';
  }
  if (task.dueDate === today || task.dueDate === addDays(today, 1)) {
    return 'primary';
  }
  return 'muted';
}

/** Тот же канон по типу секции сгруппированной ленты. */
export function taskSectionTone(kind: TaskSectionKind): TaskRowTone {
  if (kind === 'overdue') {
    return 'danger';
  }
  if (kind === 'today' || kind === 'tomorrow') {
    return 'primary';
  }
  return 'muted';
}

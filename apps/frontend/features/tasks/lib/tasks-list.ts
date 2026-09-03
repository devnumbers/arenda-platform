/**
 * Модель списка задач (#499, резолюция #497): активные задачи группируются
 * в фиксированные секции «Просроченные» → «Сегодня, D» → «Завтра, D» →
 * даты → «Без даты», выполненные — одной секцией в конце. Сортировка
 * «Дата/Название» с направлением (Figma 1535-76225) меняет только порядок
 * строк ВНУТРИ групп; группы всегда по возрастанию даты, просрочка всегда
 * сверху (решение владельца 2026-09-02).
 */

import {
  addDays,
  formatSectionDate,
  type IsoDate,
  type Task,
} from '@/entities/task';

export type TasksSortField = 'date' | 'title';
export type TasksSortDirection = 'asc' | 'desc';

/** Сортировка строк внутри групп: поле (дата срока / название) и направление. */
export type TasksSort = {
  readonly field: TasksSortField;
  readonly direction: TasksSortDirection;
};

export const DEFAULT_TASKS_SORT: TasksSort = { field: 'date', direction: 'asc' };

export type TaskSectionKind =
  | 'overdue'
  | 'today'
  | 'tomorrow'
  | 'dated'
  | 'undated'
  | 'completed';

export type TaskSection = {
  readonly kind: TaskSectionKind;
  /** Заголовок секции-карточки (счётчик «Выполненных» добавляет экран). */
  readonly title: string;
  /** ISO-дата секции (today/tomorrow/dated); у служебных — null. */
  readonly date: IsoDate | null;
  readonly tasks: readonly Task[];
};

/** Сравнение строк внутри одной группы. Направление меняет только первичный
 * ключ; тай-брейк по времени создания всегда восходящий — порядок стабилен
 * в обе стороны. «Дата» внутри группы — время дня: задача на весь день
 * (без времени) идёт первой при возрастании; в недатированной группе
 * первичный ключ вырождается во время создания. Даты и HH:MM сравниваются
 * лексикографически (обе строки фиксированного формата), не через
 * localeCompare — коллация игнорирует пробелы и цифры сравнивает численно. */
export function sortTasks(
  tasks: readonly Task[],
  sort: TasksSort,
): readonly Task[] {
  const sign = sort.direction === 'asc' ? 1 : -1;
  return [...tasks].sort((a, b) => {
    const byPrimary =
      sort.field === 'title'
        ? a.title.localeCompare(b.title, 'ru')
        : compareDue(a, b);
    if (byPrimary !== 0) {
      return sign * byPrimary;
    }
    return plainCompare(a.createdAt, b.createdAt);
  });
}

/** Первичный ключ «даты»: срок, затем время дня. */
function compareDue(a: Task, b: Task): number {
  const byDate = plainCompare(a.dueDate ?? '', b.dueDate ?? '');
  if (byDate !== 0) {
    return byDate;
  }
  return plainCompare(a.dueTime ?? '', b.dueTime ?? '');
}

function plainCompare(a: string, b: string): -1 | 0 | 1 {
  if (a < b) {
    return -1;
  }
  return a > b ? 1 : 0;
}

/** Группировка активных и выполненных задач в секции экрана. Пустые секции
 * не создаются; выполненные идут одной секцией «Выполненные» в конце
 * (счётчик — серверный total, его приносит хук). */
export function groupTasks(
  active: readonly Task[],
  completed: readonly Task[],
  today: IsoDate,
  sort: TasksSort,
): readonly TaskSection[] {
  const tomorrow = addDays(today, 1);

  const overdue = active.filter((task) => task.status === 'overdue');
  const todays = active.filter(
    (task) => task.status !== 'overdue' && task.dueDate === today,
  );
  const tomorrows = active.filter(
    (task) => task.status !== 'overdue' && task.dueDate === tomorrow,
  );
  const undated = active.filter(
    (task) => task.status !== 'overdue' && task.dueDate === null,
  );
  const rest = active.filter(
    (task) =>
      task.status !== 'overdue' &&
      task.dueDate !== null &&
      task.dueDate !== today &&
      task.dueDate !== tomorrow,
  );

  // Датированные группы — по возрастанию даты (группы фиксированы,
  // направление сортировки на их порядок не влияет).
  const datedGroups = new Map<IsoDate, Task[]>();
  for (const task of rest) {
    const date = task.dueDate;
    if (date === null) {
      continue;
    }
    const bucket = datedGroups.get(date);
    if (bucket === undefined) {
      datedGroups.set(date, [task]);
    } else {
      bucket.push(task);
    }
  }
  const dated = [...datedGroups.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, tasks]) => ({
      kind: 'dated' as const,
      title: formatSectionDate(date, today),
      date,
      tasks: sortTasks(tasks, sort),
    }));

  const sections: TaskSection[] = [];
  if (overdue.length > 0) {
    sections.push({
      kind: 'overdue',
      title: 'Просроченные',
      date: null,
      tasks: sortTasks(overdue, sort),
    });
  }
  if (todays.length > 0) {
    sections.push({
      kind: 'today',
      title: `Сегодня, ${formatSectionDate(today, today)}`,
      date: today,
      tasks: sortTasks(todays, sort),
    });
  }
  if (tomorrows.length > 0) {
    sections.push({
      kind: 'tomorrow',
      title: `Завтра, ${formatSectionDate(tomorrow, today)}`,
      date: tomorrow,
      tasks: sortTasks(tomorrows, sort),
    });
  }
  sections.push(...dated);
  if (undated.length > 0) {
    sections.push({
      kind: 'undated',
      title: 'Без даты',
      date: null,
      tasks: sortTasks(undated, sort),
    });
  }
  if (completed.length > 0) {
    sections.push({
      kind: 'completed',
      title: 'Выполненные',
      date: null,
      tasks: sortTasks(completed, sort),
    });
  }
  return sections;
}

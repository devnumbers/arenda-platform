// ПРОТОТИП (throwaway): проекции задач для отображения (списки, календари).

import type { PrototypeTask } from '../model/types';
import { addDaysISO, expandOccurrences } from '../lib/recurrence';

export type Occurrence = {
    readonly task: PrototypeTask;
    readonly date: string;
};

function sortOccurrences(a: Occurrence, b: Occurrence): number {
    if (a.date !== b.date) return a.date < b.date ? -1 : 1;
    const timeA = a.task.dueTime ?? '99';
    const timeB = b.task.dueTime ?? '99';
    if (timeA !== timeB) return timeA < timeB ? -1 : 1;
    return a.task.title.localeCompare(b.task.title, 'ru');
}

/** Развёртка вхождений в полуоткрытом окне [fromISO, toISO). */
export function occurrencesInWindow(
    tasks: readonly PrototypeTask[],
    fromISO: string,
    toISO: string,
): Occurrence[] {
    const result: Occurrence[] = [];
    for (const task of tasks) {
        if (task.recurrence) {
            for (const date of expandOccurrences(task.recurrence, task.dueDate, fromISO, toISO)) {
                result.push({ task, date });
            }
        } else if (task.dueDate >= fromISO && task.dueDate < toISO) {
            result.push({ task, date: task.dueDate });
        }
    }
    return result.sort(sortOccurrences);
}

export function occurrenceCounts(
    tasks: readonly PrototypeTask[],
    fromISO: string,
    toISO: string,
): Map<string, number> {
    const counts = new Map<string, number>();
    for (const occ of occurrencesInWindow(tasks, fromISO, toISO)) {
        counts.set(occ.date, (counts.get(occ.date) ?? 0) + 1);
    }
    return counts;
}

export function isDone(task: PrototypeTask, date: string): boolean {
    return task.completedDates.includes(date);
}

export type ActiveRow = {
    readonly task: PrototypeTask;
    /** Отображаемая дата: ближайшее невыполненное вхождение. */
    readonly date: string;
    readonly overdue: boolean;
};

/** Активный список: одинокие невыполненные + серии с ближайшим невыполненным вхождением. */
export function activeRows(tasks: readonly PrototypeTask[], todayISO: string): ActiveRow[] {
    const rows: ActiveRow[] = [];
    for (const task of tasks) {
        if (!task.recurrence) {
            if (task.completedDates.length === 0) {
                rows.push({ task, date: task.dueDate, overdue: task.dueDate < todayISO });
            }
            continue;
        }
        const windowEnd = task.recurrence.until
            ? addDaysISO(task.recurrence.until, 1)
            : addDaysISO(todayISO, 800);
        const future = expandOccurrences(task.recurrence, task.dueDate, todayISO, windowEnd);
        const next = future.find((d) => !isDone(task, d));
        if (next) {
            rows.push({ task, date: next, overdue: false });
            continue;
        }
        const past = expandOccurrences(task.recurrence, task.dueDate, task.dueDate, todayISO);
        const lastMissed = [...past].reverse().find((d) => !isDone(task, d));
        if (lastMissed) {
            rows.push({ task, date: lastMissed, overdue: true });
        }
    }
    return rows.sort((a, b) => {
        if (a.overdue !== b.overdue) return a.overdue ? -1 : 1;
        if (a.date !== b.date) return a.date < b.date ? -1 : 1;
        return a.task.title.localeCompare(b.task.title, 'ru');
    });
}

export type CompletedRow = {
    readonly task: PrototypeTask;
    readonly date: string;
};

/** Выполненные: журнальные записи, свежие сверху. */
export function completedRows(tasks: readonly PrototypeTask[]): CompletedRow[] {
    const rows: CompletedRow[] = [];
    for (const task of tasks) {
        for (const date of task.completedDates) {
            rows.push({ task, date });
        }
    }
    return rows.sort((a, b) => (a.date > b.date ? -1 : a.date < b.date ? 1 : 0));
}

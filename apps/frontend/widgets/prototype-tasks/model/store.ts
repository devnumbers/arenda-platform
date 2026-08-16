// ПРОТОТИП (throwaway): in-memory стор задач с семантикой серий из #272/#275:
// правка/удаление одиночного вхождения = разрыв (обрезка UNTIL + одинокая задача + продолжение);
// «вся серия» — только контентные поля; завершения ≥ точки разрыва переходят к продолжению.

import type { EditScope, PrototypeTask, TaskDraft } from './types';
import {
    addDaysISO,
    expandOccurrences,
    firstOccurrenceAfter,
    hasOccurrenceOn,
} from '../lib/recurrence';

export type TasksAction =
    | { readonly type: 'add'; readonly draft: TaskDraft }
    | {
          readonly type: 'update';
          readonly id: string;
          readonly draft: TaskDraft;
          readonly scope: EditScope;
          readonly occurrenceDate?: string;
      }
    | {
          readonly type: 'remove';
          readonly id: string;
          readonly scope: EditScope;
          readonly occurrenceDate?: string;
      }
    | { readonly type: 'toggle'; readonly id: string; readonly occurrenceDate: string }
    | { readonly type: 'reset' };

let idCounter = 1000;
function nextId(): string {
    idCounter += 1;
    return `t${idCounter}`;
}

function draftToTask(draft: TaskDraft, completedDates: readonly string[] = []): PrototypeTask {
    return {
        id: nextId(),
        title: draft.title,
        description: draft.description,
        dueDate: draft.dueDate,
        dueTime: draft.dueTime,
        recurrence: draft.recurrence,
        completedDates,
    };
}

/** Обрезать серию «до» даты: until = день перед cutISO. Возвращает undefined, если вхождений не осталось и журнал пуст. */
function truncateBefore(task: PrototypeTask, cutISO: string): PrototypeTask | undefined {
    if (!task.recurrence) return undefined;
    const until = addDaysISO(cutISO, -1);
    const kept = task.completedDates.filter((d) => d < cutISO);
    const remaining = expandOccurrences(
        { ...task.recurrence, until, count: undefined },
        task.dueDate,
        task.dueDate,
        cutISO,
    );
    if (remaining.length === 0 && kept.length === 0) return undefined;
    return {
        ...task,
        recurrence: { ...task.recurrence, until, count: undefined },
        completedDates: kept,
    };
}

/** Продолжение серии с новым DTSTART (первая дата развёртки ≥ fromISO по новому правилу). */
function continuation(
    task: PrototypeTask,
    draft: TaskDraft,
    fromISO: string,
    includeFrom: boolean,
): PrototypeTask | undefined {
    if (!draft.recurrence) return undefined;
    const start = includeFrom
        ? (hasOccurrenceOn(draft.recurrence, draft.dueDate, fromISO)
              ? fromISO
              : firstOccurrenceAfter(draft.recurrence, draft.dueDate, addDaysISO(fromISO, -1)))
        : firstOccurrenceAfter(draft.recurrence, draft.dueDate, fromISO);
    if (!start) return undefined;
    const moved = task.completedDates.filter((d) => (includeFrom ? d >= fromISO : d > fromISO));
    return {
        ...draftToTask(draft),
        dueDate: start,
        completedDates: moved,
    };
}

function updateScoped(
    tasks: readonly PrototypeTask[],
    action: Extract<TasksAction, { readonly type: 'update' }>,
): PrototypeTask[] {
    const task = tasks.find((t) => t.id === action.id);
    if (!task) return [...tasks];

    // Одинокая задача — правится на месте, scope не имеет смысла.
    if (!task.recurrence) {
        return tasks.map((t) =>
            t.id === task.id
                ? { ...t, ...action.draft, completedDates: t.completedDates }
                : t,
        );
    }

    const occ = action.occurrenceDate ?? task.dueDate;

    if (action.scope === 'all') {
        // Только контентные поля; расписание не трогаем.
        return tasks.map((t) =>
            t.id === task.id
                ? { ...t, title: action.draft.title, description: action.draft.description }
                : t,
        );
    }

    if (action.scope === 'this_and_future') {
        const before = truncateBefore(task, occ);
        const after = continuation(task, action.draft, occ, true);
        const result = tasks.filter((t) => t.id !== task.id);
        if (before) result.push(before);
        if (after) result.push(after);
        return result;
    }

    // scope === 'this': разрыв — до + одинокая + продолжение (по СТАРОМУ правилу и контенту).
    const before = truncateBefore(task, occ);
    const after = continuationFromOldRule(task, occ);
    const single = draftToTask(
        { ...action.draft, recurrence: undefined },
        task.completedDates.includes(occ) ? [occ] : [],
    );
    const result = tasks.filter((t) => t.id !== task.id);
    if (before) result.push(before);
    result.push(single);
    if (after) result.push(after);
    return result;
}

/** Продолжение по СТАРОМУ правилу (для хвоста серии после разрыва «только это»). */
function continuationFromOldRule(task: PrototypeTask, occISO: string): PrototypeTask | undefined {
    if (!task.recurrence) return undefined;
    const start = firstOccurrenceAfter(task.recurrence, task.dueDate, occISO);
    if (!start) return undefined;
    return {
        ...task,
        id: nextId(),
        dueDate: start,
        completedDates: task.completedDates.filter((d) => d > occISO),
    };
}

function removeScoped(
    tasks: readonly PrototypeTask[],
    action: Extract<TasksAction, { readonly type: 'remove' }>,
): PrototypeTask[] {
    const task = tasks.find((t) => t.id === action.id);
    if (!task) return [...tasks];
    if (!task.recurrence || action.scope === 'all') {
        return tasks.filter((t) => t.id !== task.id);
    }
    const occ = action.occurrenceDate ?? task.dueDate;
    if (action.scope === 'this_and_future') {
        const before = truncateBefore(task, occ);
        const result = tasks.filter((t) => t.id !== task.id);
        if (before) result.push(before);
        return result;
    }
    // 'this': вырезать одно вхождение.
    const before = truncateBefore(task, occ);
    const after = continuationFromOldRule(task, occ);
    const result = tasks.filter((t) => t.id !== task.id);
    if (before) result.push(before);
    if (after) result.push(after);
    return result;
}

export function tasksReducer(
    tasks: readonly PrototypeTask[],
    action: TasksAction,
): PrototypeTask[] {
    switch (action.type) {
        case 'add':
            return [...tasks, draftToTask(action.draft)];
        case 'update':
            return updateScoped(tasks, action);
        case 'remove':
            return removeScoped(tasks, action);
        case 'toggle':
            return tasks.map((t) =>
                t.id === action.id
                    ? {
                          ...t,
                          completedDates: t.completedDates.includes(action.occurrenceDate)
                              ? t.completedDates.filter((d) => d !== action.occurrenceDate)
                              : [...t.completedDates, action.occurrenceDate],
                      }
                    : t,
            );
        case 'reset':
            return [];
    }
}

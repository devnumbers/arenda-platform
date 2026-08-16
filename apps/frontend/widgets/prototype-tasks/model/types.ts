// ПРОТОТИП (throwaway): модель данных для демо UI задач. Не для продакшена.
// Отражает решения карты #266: журнал завершений (task_id, occurrence_date),
// повтор — типизированное подмножество RFC 5545 (FREQ/INTERVAL/BYDAY/BYMONTHDAY/UNTIL/COUNT).

export type RecurrenceFreq = 'daily' | 'weekly' | 'monthly' | 'yearly';

/** День недели: 0 = пн … 6 = вс. */
export type Weekday = 0 | 1 | 2 | 3 | 4 | 5 | 6;

export type RecurrenceRule = {
    readonly freq: RecurrenceFreq;
    readonly interval: number;
    /** Только weekly. Пусто/undefined → день недели DTSTART. */
    readonly byWeekdays?: readonly Weekday[];
    /** Только monthly/yearly. Пусто/undefined → число DTSTART. Невалидные даты пропускаются. */
    readonly byMonthDays?: readonly number[];
    /** Включительно, 'YYYY-MM-DD'. UNTIL+COUNT вместе запрещены. */
    readonly until?: string;
    /** Всего вхождений, включая DTSTART. */
    readonly count?: number;
};

export type PrototypeTask = {
    readonly id: string;
    readonly title: string;
    readonly description?: string;
    /** Плавающая дата 'YYYY-MM-DD' (= DTSTART для повторяющейся). */
    readonly dueDate: string;
    /** 'HH:MM', необязательно. */
    readonly dueTime?: string;
    /** undefined = одинокая задача. */
    readonly recurrence?: RecurrenceRule;
    /** Журнал завершений: даты развёртки вхождений. */
    readonly completedDates: readonly string[];
};

/** Рамки правки/удаления вхождения серии (см. #275/#278). */
export type EditScope = 'this' | 'this_and_future' | 'all';

export type TaskDraft = {
    readonly title: string;
    readonly description?: string;
    readonly dueDate: string;
    readonly dueTime?: string;
    readonly recurrence?: RecurrenceRule;
};

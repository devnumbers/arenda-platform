// ПРОТОТИП (throwaway): развёртка повторов на чистых датах.
// Семантика краёв — по research #274: невалидные даты skip'аются (не расходуют COUNT),
// UNTIL включителен, UNTIL+COUNT не сочетаются, COUNT включает DTSTART.
// Даты — локальные 'YYYY-MM-DD', без времени и таймзон.

import type { RecurrenceRule, Weekday } from '../model/types';

export function parseISODate(iso: string): Date {
    const [y, m, d] = iso.split('-').map(Number);
    return new Date(y, m - 1, d);
}

export function toISODate(date: Date): string {
    const y = date.getFullYear();
    const m = String(date.getMonth() + 1).padStart(2, '0');
    const d = String(date.getDate()).padStart(2, '0');
    return `${y}-${m}-${d}`;
}

export function todayISO(): string {
    return toISODate(new Date());
}

export function addDaysISO(iso: string, days: number): string {
    const date = parseISODate(iso);
    date.setDate(date.getDate() + days);
    return toISODate(date);
}

/** Пн=0 … Вс=6. */
export function weekdayOf(iso: string): Weekday {
    return ((parseISODate(iso).getDay() + 6) % 7) as Weekday;
}

function diffDays(fromISO: string, toISO_: string): number {
    return Math.round((parseISODate(toISO_).getTime() - parseISODate(fromISO).getTime()) / 86_400_000);
}

/** Совпадает ли дата с правилом (без учёта DTSTART/UNTIL/COUNT). Невалидные числа месяца просто не совпадают никогда — skip. */
function matchesRule(rule: RecurrenceRule, dtstartISO: string, dateISO: string): boolean {
    if (dateISO < dtstartISO) return false;
    const dt = parseISODate(dateISO);
    const start = parseISODate(dtstartISO);
    const interval = Math.max(1, rule.interval);

    switch (rule.freq) {
        case 'daily':
            return diffDays(dtstartISO, dateISO) % interval === 0;
        case 'weekly': {
            const weekStart = (iso: string): string => addDaysISO(iso, -weekdayOf(iso));
            const weeks = diffDays(weekStart(dtstartISO), weekStart(dateISO)) / 7;
            if (weeks % interval !== 0) return false;
            const days = rule.byWeekdays ?? [weekdayOf(dtstartISO)];
            return days.includes(weekdayOf(dateISO));
        }
        case 'monthly':
        case 'yearly': {
            const monthDiff =
                (dt.getFullYear() - start.getFullYear()) * 12 + (dt.getMonth() - start.getMonth());
            const step = rule.freq === 'monthly' ? interval : interval * 12;
            if (monthDiff % step !== 0) return false;
            if (rule.freq === 'yearly' && dt.getMonth() !== start.getMonth()) return false;
            const days = rule.byMonthDays ?? [start.getDate()];
            return days.includes(dt.getDate());
        }
    }
}

const MAX_SCAN_DAYS = 2200; // ~6 лет — потолок перебора для демо.

/** Все даты вхождений в полуоткрытом окне [windowStartISO, windowEndISO). */
export function expandOccurrences(
    rule: RecurrenceRule,
    dtstartISO: string,
    windowStartISO: string,
    windowEndISO: string,
): string[] {
    const result: string[] = [];
    let matched = 0; // для COUNT: считаются все вхождения от DTSTART, и вне окна.
    let cursor = dtstartISO;
    for (let i = 0; i < MAX_SCAN_DAYS; i += 1) {
        if (cursor >= windowEndISO) break;
        if (rule.until && cursor > rule.until) break;
        if (matchesRule(rule, dtstartISO, cursor)) {
            matched += 1;
            if (rule.count !== undefined && matched > rule.count) break;
            if (cursor >= windowStartISO) result.push(cursor);
        }
        cursor = addDaysISO(cursor, 1);
    }
    return result;
}

/** Первое вхождение строго после даты (для продолжения серии после разрыва). */
export function firstOccurrenceAfter(rule: RecurrenceRule, dtstartISO: string, afterISO: string): string | undefined {
    const found = expandOccurrences(rule, dtstartISO, addDaysISO(afterISO, 1), addDaysISO(afterISO, 800));
    return found[0];
}

/** Есть ли вхождение ровно в дату (с учётом UNTIL/COUNT). */
export function hasOccurrenceOn(rule: RecurrenceRule, dtstartISO: string, dateISO: string): boolean {
    return expandOccurrences(rule, dtstartISO, dateISO, addDaysISO(dateISO, 1)).length > 0;
}

const WEEKDAY_SHORT: readonly string[] = ['пн', 'вт', 'ср', 'чт', 'пт', 'сб', 'вс'];

export function formatDateHuman(iso: string): string {
    const [y, m, d] = iso.split('-');
    return `${Number(d)}.${m}.${y}`;
}

/** Короткая подпись повтора: «Каждый месяц: 11, 14, 27 числа». */
export function formatRecurrence(rule: RecurrenceRule, dtstartISO: string): string {
    const interval = Math.max(1, rule.interval);
    let base: string;
    switch (rule.freq) {
        case 'daily':
            base = interval === 1 ? 'Каждый день' : `Каждые ${interval} дн.`;
            break;
        case 'weekly': {
            const days = (rule.byWeekdays ?? [weekdayOf(dtstartISO)])
                .map((d) => WEEKDAY_SHORT[d])
                .join(', ');
            base = interval === 1 ? `Каждую неделю: ${days}` : `Каждые ${interval} нед.: ${days}`;
            break;
        }
        case 'monthly': {
            const days = [...(rule.byMonthDays ?? [parseISODate(dtstartISO).getDate()])]
                .sort((a, b) => a - b)
                .join(', ');
            base = interval === 1 ? `Каждый месяц: ${days}` : `Каждые ${interval} мес.: ${days}`;
            break;
        }
        case 'yearly':
            base = interval === 1 ? 'Каждый год' : `Каждые ${interval} г.`;
            break;
    }
    if (rule.until) return `${base}, до ${formatDateHuman(rule.until)}`;
    if (rule.count !== undefined) return `${base}, ${rule.count} раз`;
    return base;
}

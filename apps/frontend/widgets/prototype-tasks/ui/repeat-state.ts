// ПРОТОТИП (throwaway): состояние редактора повтора ↔ RecurrenceRule.

import type { RecurrenceFreq, RecurrenceRule, Weekday } from '../model/types';
import { formatRecurrence, weekdayOf, parseISODate } from '../lib/recurrence';

export type RepeatEnd = 'never' | 'until' | 'count';

export type RepeatEditorState = {
    readonly enabled: boolean;
    readonly freq: RecurrenceFreq;
    readonly interval: number;
    readonly weekdays: readonly Weekday[];
    readonly monthDays: readonly number[];
    readonly end: RepeatEnd;
    readonly untilDate: string;
    readonly countNum: number;
};

export function repeatStateFromRule(
    rule: RecurrenceRule | undefined,
    dtstartISO: string,
): RepeatEditorState {
    return {
        enabled: Boolean(rule),
        freq: rule?.freq ?? 'weekly',
        interval: rule?.interval ?? 1,
        weekdays: rule?.byWeekdays ?? [weekdayOf(dtstartISO)],
        monthDays: rule?.byMonthDays ?? [parseISODate(dtstartISO).getDate()],
        end: rule?.until ? 'until' : rule?.count !== undefined ? 'count' : 'never',
        untilDate: rule?.until ?? dtstartISO,
        countNum: rule?.count ?? 10,
    };
}

export function repeatStateToRule(state: RepeatEditorState): RecurrenceRule | undefined {
    if (!state.enabled) return undefined;
    const base = {
        freq: state.freq,
        interval: Math.max(1, state.interval),
        ...(state.freq === 'weekly' ? { byWeekdays: state.weekdays } : {}),
        ...(state.freq === 'monthly' ? { byMonthDays: state.monthDays } : {}),
    };
    if (state.end === 'until' && state.untilDate) return { ...base, until: state.untilDate };
    if (state.end === 'count' && state.countNum > 0) return { ...base, count: state.countNum };
    return base;
}

export function repeatSummary(state: RepeatEditorState, dtstartISO: string): string {
    const rule = repeatStateToRule(state);
    return rule ? formatRecurrence(rule, dtstartISO) : 'Не повторяется';
}

// ПРОТОТИП (throwaway): месячный календарь с точками вхождений задач.

'use client';

import { useMemo, useState, type JSX } from 'react';
import clsx from 'clsx';
import { addDaysISO, parseISODate, toISODate, todayISO } from '../lib/recurrence';
import styles from './PrototypeMonthCalendar.module.css';

const WEEKDAY_HEADERS: readonly string[] = ['пн', 'вт', 'ср', 'чт', 'пт', 'сб', 'вс'];
const MONTH_NAMES: readonly string[] = [
    'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
    'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

export type PrototypeMonthCalendarProps = {
    /** Дата → число вхождений (точки в ячейке). */
    readonly counts: ReadonlyMap<string, number>;
    readonly selectedDate?: string;
    readonly onSelectDate: (iso: string) => void;
};

export function PrototypeMonthCalendar({
    counts,
    selectedDate,
    onSelectDate,
}: PrototypeMonthCalendarProps): JSX.Element {
    const today = todayISO();
    const [cursor, setCursor] = useState<string>(() => (selectedDate ?? today).slice(0, 8) + '01');

    const cells = useMemo(() => {
        const first = parseISODate(cursor);
        const startOffset = (first.getDay() + 6) % 7; // пн=0
        const gridStart = addDaysISO(cursor, -startOffset);
        return Array.from({ length: 42 }, (_, i) => addDaysISO(gridStart, i));
    }, [cursor]);

    const shiftMonth = (delta: number): void => {
        const d = parseISODate(cursor);
        setCursor(toISODate(new Date(d.getFullYear(), d.getMonth() + delta, 1)));
    };

    const monthLabel = `${MONTH_NAMES[parseISODate(cursor).getMonth()]} ${cursor.slice(0, 4)}`;

    return (
        <div className={styles.root}>
            <div className={styles.header}>
                <button type="button" className={styles.nav} aria-label="Предыдущий месяц" onClick={() => shiftMonth(-1)}>‹</button>
                <span className={styles.month}>{monthLabel}</span>
                <button type="button" className={styles.nav} aria-label="Следующий месяц" onClick={() => shiftMonth(1)}>›</button>
            </div>
            <div className={styles.grid}>
                {WEEKDAY_HEADERS.map((d) => (
                    <span key={d} className={styles.weekday}>{d}</span>
                ))}
                {cells.map((iso) => {
                    const inMonth = iso.slice(0, 7) === cursor.slice(0, 7);
                    const count = counts.get(iso) ?? 0;
                    return (
                        <button
                            key={iso}
                            type="button"
                            className={clsx(
                                styles.cell,
                                !inMonth && styles.cellOutside,
                                iso === today && styles.cellToday,
                                iso === selectedDate && styles.cellSelected,
                            )}
                            onClick={() => onSelectDate(iso)}
                        >
                            <span>{Number(iso.slice(8, 10))}</span>
                            {count > 0 && (
                                <span className={styles.dots} aria-hidden="true">
                                    {Array.from({ length: Math.min(3, count) }, (_, i) => (
                                        <span key={i} className={styles.dot} />
                                    ))}
                                </span>
                            )}
                        </button>
                    );
                })}
            </div>
        </div>
    );
}

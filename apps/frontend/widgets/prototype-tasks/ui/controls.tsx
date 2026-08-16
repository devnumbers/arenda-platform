// ПРОТОТИП (throwaway): нейтральные контролы редактора повтора, общие для вариантов.

import type { JSX } from 'react';
import clsx from 'clsx';
import type { Weekday } from '../model/types';
import styles from './controls.module.css';

const WEEKDAY_LABELS: readonly string[] = ['пн', 'вт', 'ср', 'чт', 'пт', 'сб', 'вс'];

export function WeekdayChips({
    value,
    onChange,
}: {
    readonly value: readonly Weekday[];
    readonly onChange: (days: Weekday[]) => void;
}): JSX.Element {
    const toggle = (day: Weekday): void => {
        const next = value.includes(day)
            ? value.filter((d) => d !== day)
            : [...value, day].sort((a, b) => a - b);
        if (next.length > 0) onChange(next as Weekday[]);
    };
    return (
        <div className={styles.chipRow} role="group" aria-label="Дни недели">
            {WEEKDAY_LABELS.map((label, index) => {
                const day = index as Weekday;
                const selected = value.includes(day);
                return (
                    <button
                        key={label}
                        type="button"
                        aria-pressed={selected}
                        className={clsx(styles.chip, selected && styles.chipSelected)}
                        onClick={() => toggle(day)}
                    >
                        {label}
                    </button>
                );
            })}
        </div>
    );
}

/** Сетка чисел 1–31 с мультивыбором — «каждый месяц 11, 14, 27 числа». */
export function MonthDayGrid({
    value,
    onChange,
}: {
    readonly value: readonly number[];
    readonly onChange: (days: number[]) => void;
}): JSX.Element {
    const toggle = (day: number): void => {
        const next = value.includes(day)
            ? value.filter((d) => d !== day)
            : [...value, day].sort((a, b) => a - b);
        if (next.length > 0) onChange(next);
    };
    return (
        <div className={styles.dayGrid} role="group" aria-label="Числа месяца">
            {Array.from({ length: 31 }, (_, i) => i + 1).map((day) => {
                const selected = value.includes(day);
                return (
                    <button
                        key={day}
                        type="button"
                        aria-pressed={selected}
                        className={clsx(styles.dayCell, selected && styles.dayCellSelected)}
                        onClick={() => toggle(day)}
                    >
                        {day}
                    </button>
                );
            })}
        </div>
    );
}

'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { CalendarButton } from './calendar-button';

/** Мини-грид дней месяца (тикет #459, Figma 823:11422) — ветка
 * периодичности «Каждый месяц»: числа 1..N подряд с первого столбца, без
 * привязки к дням недели и без заголовка недели; выбор нескольких дней.
 * Опция «Последний день месяца» — строка с чекбоксом под гридом на экране
 * (ListRow + Checkbox), сюда не входит. */
export type MonthDaysGridProps = {
  /** Число дней: 28..31. */
  readonly days: number;
  readonly selectedDays?: ReadonlySet<number>;
  readonly onDayToggle?: (day: number) => void;
  readonly className?: string;
};

export function MonthDaysGrid({
  days,
  selectedDays,
  onDayToggle,
  className,
}: MonthDaysGridProps): JSX.Element {
  return (
    <div className={cn('grid grid-cols-7 gap-0.5 px-4', className)}>
      {Array.from({ length: days }, (_, index) => {
        const day = index + 1;
        const selected = selectedDays?.has(day) === true;
        return (
          <CalendarButton
            key={day}
            className="w-full"
            aria-pressed={selected}
            state={selected ? 'selected' : 'default'}
            onClick={onDayToggle !== undefined ? () => onDayToggle(day) : undefined}
          >
            {day}
          </CalendarButton>
        );
      })}
    </div>
  );
}

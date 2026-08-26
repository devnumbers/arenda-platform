'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { CalendarButton } from './calendar-button';

/** Мини-грид дней месяца (тикет #459, Figma 823:11422) — ветка
 * периодичности «Каждый месяц»: числа 1..N подряд с первого столбца, без
 * привязки к дням недели и без заголовка недели; выбор нескольких дней.
 * Опция «Последний день месяца» — строка с чекбоксом под гридом на экране
 * (ListRow + Checkbox), сюда не входит. Ячейки квадратные, как в макете:
 * ширина колонки, высота следует за ней; блок тянется во всю доступную
 * ширину, максимум — ширина контента страницы 560 − 2×24 = 512px
 * (решение владельца 2026-08-26), на узких экранах ужимается
 * пропорционально и не выходит за экран. */
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
    <div className={cn('mx-auto grid w-full max-w-[512px] grid-cols-7 gap-0.5 px-4', className)}>
      {Array.from({ length: days }, (_, index) => {
        const day = index + 1;
        const selected = selectedDays?.has(day) === true;
        return (
          <CalendarButton
            key={day}
            className="aspect-square h-auto w-full"
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

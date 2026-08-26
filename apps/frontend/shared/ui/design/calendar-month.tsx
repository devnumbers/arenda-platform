'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { CalendarButton } from './calendar-button';
import {
  WEEKDAY_LABELS,
  daysInMonth,
  firstWeekdayOfMonth,
  isCalendarDay,
  monthTitle,
} from './month-grid';

/** Блок одного месяца календаря (тикет #459, Figma 835:20007): строка дней
 * недели, заголовок «Август, 2026» и сетка дней выбранного месяца —
 * без бесконечного скролла и дней соседних месяцев (решение владельца
 * 2026-08-26; месяц и год выбираются крутилкой MonthYearPicker). Порядок
 * блоков и отступы — по макету: дни недели (отступ 20), заголовок
 * (отступ 24, Пн-строка вплотную сверху), сетка (отступ 16, gap 2). */
export type CalendarMonthProps = {
  readonly year: number;
  /** 0..11, как у Date. */
  readonly month: number;
  /** Выбранная дата; сегодня — маркер today у ячейки. */
  readonly value?: Date;
  readonly today?: Date;
  readonly onDateSelect?: (date: Date) => void;
  readonly isDateDisabled?: (date: Date) => boolean;
  readonly className?: string;
};

export function CalendarMonth({
  year,
  month,
  value,
  today,
  onDateSelect,
  isDateDisabled,
  className,
}: CalendarMonthProps): JSX.Element {
  const firstWeekday = firstWeekdayOfMonth(year, month);
  const days = daysInMonth(year, month);

  return (
    <div className={cn('flex flex-col', className)}>
      <div className="grid grid-cols-7 px-5">
        {WEEKDAY_LABELS.map((weekday) => (
          <div
            key={weekday}
            className="flex h-12 items-center justify-center font-sans text-base font-medium leading-[18px] text-content-tertiary"
          >
            {weekday}
          </div>
        ))}
      </div>
      <h3 className="px-6 font-sans text-xl font-semibold leading-6 text-content">
        {monthTitle(year, month)}
      </h3>
      <div className="mt-4 grid grid-cols-7 gap-0.5 px-4">
        {Array.from({ length: firstWeekday }, (_, index) => (
          <div key={`blank-${index}`} aria-hidden className="h-12" />
        ))}
        {Array.from({ length: days }, (_, index) => {
          const day = index + 1;
          const date = new Date(year, month, day);
          const selected = value !== undefined && isCalendarDay(value, year, month, day);
          const isToday = today !== undefined && isCalendarDay(today, year, month, day);
          return (
            <CalendarButton
              key={day}
              className="w-full"
              state={selected ? 'selected' : isToday ? 'today' : 'default'}
              aria-current={isToday ? 'date' : undefined}
              disabled={isDateDisabled !== undefined && isDateDisabled(date)}
              onClick={onDateSelect !== undefined ? () => onDateSelect(date) : undefined}
            >
              {day}
            </CalendarButton>
          );
        })}
      </div>
    </div>
  );
}

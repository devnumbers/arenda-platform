'use client';

import type { JSX } from 'react';
import type { IsoDate } from '@/entities/payment';
import type {
  OperationsMonth,
  OperationsPeriod,
  OperationsPeriodDraft,
} from '@/features/payments';
import {
  booleanRunSegments,
  operationsMonthIndex,
  operationsMonthIso,
  operationsMonthOf,
} from '@/features/payments';
import { cn } from '@/shared/lib/cn';
import {
  daysInMonth,
  firstWeekdayOfMonth,
  monthTitle,
} from '@/shared/ui/design/month-grid';

/**
 * Месячный блок календаря выбора периода (#477, Figma 1495-64015):
 * заголовок «Ноябрь, 2026» и сетка дней, в которой диапазон подчёркнут
 * серыми «пилюлями»-подложками — недели целиком внутри диапазона и
 * краевые отрезки до/после синих границ; сами границы — синие ячейки
 * radius 12 с белым текстом. Будущие дни «сегодня» приглушены и
 * недоступны (экраны операций — только paid, резолюция #474), целиком
 * будущий месяц гасится весь (в макете — Декабрь 2026, Январь 2027).
 * Строка недели — grid 7 колонок: подложки и ячейки — элементы одного
 * ряда, подложка кладётся gridColumn-отрезком под ячейки.
 */

export type OperationsRangeCalendarMonthProps = {
  readonly month: OperationsMonth;
  readonly draft: OperationsPeriodDraft;
  /** ISO-дата «сегодня» — граница доступных дней. */
  readonly today: IsoDate;
  readonly onPick: (day: IsoDate) => void;
};

export function OperationsRangeCalendarMonth({
  month,
  draft,
  today,
  onPick,
}: OperationsRangeCalendarMonthProps): JSX.Element {
  const complete: OperationsPeriod | null =
    draft.end !== null ? { from: draft.start, to: draft.end } : null;
  const firstWeekday = firstWeekdayOfMonth(month.year, month.month);
  const days = daysInMonth(month.year, month.month);
  const todayMonth = operationsMonthOf(today);
  const monthFuture =
    operationsMonthIndex(month) > operationsMonthIndex(todayMonth);

  // Недели месяца с пустыми колонками до 1-го числа и после последнего.
  const weeks: Array<Array<IsoDate | null>> = [];
  let week: Array<IsoDate | null> = Array.from({ length: firstWeekday }, () => null);
  for (let day = 1; day <= days; day += 1) {
    week.push(operationsMonthIso(month, day));
    if (week.length === 7) {
      weeks.push(week);
      week = [];
    }
  }
  if (week.length > 0) {
    weeks.push([...week, ...Array.from({ length: 7 - week.length }, () => null)]);
  }

  return (
    <section
      aria-label={monthTitle(month.year, month.month)}
      className={cn('flex w-full flex-col gap-2', monthFuture && 'opacity-50')}
    >
      <h3 className="px-6 text-xl font-semibold leading-6 text-content">
        {monthTitle(month.year, month.month)}
      </h3>
      <div className="flex flex-col gap-0.5 px-4">
        {weeks.map((daysOfWeek, weekIndex) => (
          <WeekRow
            key={weekIndex}
            days={daysOfWeek}
            draft={draft}
            complete={complete}
            today={today}
            onPick={onPick}
          />
        ))}
      </div>
    </section>
  );
}

/** Недельный ряд: серые отрезки диапазона + ячейки дней одного грида. */
function WeekRow({
  days,
  draft,
  complete,
  today,
  onPick,
}: {
  readonly days: ReadonlyArray<IsoDate | null>;
  readonly draft: OperationsPeriodDraft;
  readonly complete: OperationsPeriod | null;
  readonly today: IsoDate;
  readonly onPick: (day: IsoDate) => void;
}): JSX.Element {
  const inRange = days.map(
    (day) => day !== null && complete !== null && day >= complete.from && day <= complete.to,
  );
  const segments = booleanRunSegments(inRange);

  return (
    <div className="grid grid-cols-7 gap-0.5">
      {segments.map(([from, to]) => (
        <div
          key={`segment-${from}-${to}`}
          aria-hidden
          className="row-start-1 h-full rounded-xl bg-surface-muted"
          style={{ gridColumn: `${from + 1} / ${to + 2}`, gridRow: 1 }}
        />
      ))}
      {days.map((day, column) =>
        day === null ? (
          <div
            key={`blank-${column}`}
            className="row-start-1 aspect-square"
            style={{ gridColumn: column + 1, gridRow: 1 }}
          />
        ) : (
          <DayCell
            key={day}
            day={day}
            column={column}
            boundary={day === draft.start || day === draft.end}
            inRange={inRange[column] === true}
            disabled={day > today}
            onPick={onPick}
          />
        ),
      )}
    </div>
  );
}

/** Ячейка дня: граница — синяя (radius 12, белый текст), день диапазона —
 * прозрачная над серой подложкой, вне диапазона — обычная с hover. */
function DayCell({
  day,
  column,
  boundary,
  inRange,
  disabled,
  onPick,
}: {
  readonly day: IsoDate;
  readonly column: number;
  readonly boundary: boolean;
  readonly inRange: boolean;
  readonly disabled: boolean;
  readonly onPick: (day: IsoDate) => void;
}): JSX.Element {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={() => onPick(day)}
      aria-pressed={boundary}
      className={cn(
        'aspect-square w-full cursor-pointer rounded-xl text-center font-sans text-base font-medium leading-[18px] text-content outline-none transition-colors',
        'focus-visible:ring-2 focus-visible:ring-primary',
        'disabled:pointer-events-none disabled:opacity-50',
        boundary
          ? 'bg-primary text-white hover:bg-primary-hover active:bg-primary-active'
          : inRange
            ? 'bg-transparent hover:bg-black/5 active:bg-black/10'
            : 'bg-transparent hover:bg-surface-muted active:bg-surface-muted-hover',
      )}
      style={{ gridColumn: column + 1, gridRow: 1 }}
    >
      {Number(day.slice(8, 10))}
    </button>
  );
}

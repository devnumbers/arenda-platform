'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ArrowLeft, ArrowRight, ChevronDown, ChevronUp } from '@/shared/assets/icons';
import type { IsoDate } from '@/entities/payment';
import { formatDayMonthWithYear } from '@/entities/payment';
import {
  CalendarMonth,
  IconButton,
  ListRow,
  Button,
  monthTitle,
} from '@/shared/ui/design';
import { isoToUtcDate, utcDateToIso } from '../../lib/calendar-date';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 4 визарда — «Окончание платежа» (Figma 843:8345/851:15788): только
 * дата; напоминания и email-уведомления — вне среза. Пусто — бессрочный.
 * Строка «Выбрать дату» разворачивает календарь прямо в шаге (ветки дат
 * шага 3 — тот же паттерн инлайн-выбора), прошлое отключено: правило
 * действует с даты заведения без вхождений задним числом.
 */

export type EndDateStepProps = {
  readonly endDate: IsoDate | undefined;
  readonly onEndDateChange: (endDate: IsoDate | undefined) => void;
  readonly today: IsoDate;
  /** Заголовок шага рисует хост (шит правки #467 несёт его в ModalContent). */
  readonly withHeading?: boolean;
};

export function EndDateStep({
  endDate,
  onEndDateChange,
  today,
  withHeading = true,
}: EndDateStepProps): JSX.Element {
  const [calendarOpen, setCalendarOpen] = useState(false);
  const [view, setView] = useState<{ year: number; month: number }>(() => {
    const anchor = endDate ?? today;
    return { year: Number(anchor.slice(0, 4)), month: Number(anchor.slice(5, 7)) - 1 };
  });

  const shiftMonth = (delta: number): void => {
    setView((prev) => {
      const next = new Date(Date.UTC(prev.year, prev.month + delta, 1));
      return { year: next.getUTCFullYear(), month: next.getUTCMonth() };
    });
  };

  const pick = (date: Date): void => {
    if (utcDateToIso(date) < today) {
      return;
    }
    onEndDateChange(utcDateToIso(date));
    setCalendarOpen(false);
  };

  return (
    <>
      {withHeading && (
        <WizardHeading
          title="Окончание платежа"
          subtitle="После выбранной даты, платеж перестанет оплачиваться и удалится. Необязательно"
        />
      )}
      <div className="flex flex-col pt-2">
        <ListRow
          title="Выбрать дату"
          value={endDate !== undefined ? formatDayMonthWithYear(endDate, today) : undefined}
          trailing={
            calendarOpen ? (
              <ChevronUp className="h-5 w-5 text-content-secondary" aria-hidden />
            ) : (
              <ChevronDown className="h-5 w-5 text-content-secondary" aria-hidden />
            )
          }
          onSelect={() => setCalendarOpen((open) => !open)}
        />
        {calendarOpen && (
          <div className="px-6 pt-3">
            <div className="flex items-center justify-between pb-2">
              <IconButton
                icon={<ArrowLeft />}
                label="Предыдущий месяц"
                variant="secondary"
                onClick={() => shiftMonth(-1)}
              />
              <span className="text-base font-medium text-content" aria-live="polite">
                {monthTitle(view.year, view.month)}
              </span>
              <IconButton
                icon={<ArrowRight />}
                label="Следующий месяц"
                variant="secondary"
                onClick={() => shiftMonth(1)}
              />
            </div>
            <CalendarMonth
              year={view.year}
              month={view.month}
              value={endDate !== undefined ? isoToUtcDate(endDate) : undefined}
              isDateDisabled={(date) => utcDateToIso(date) < today}
              onDateSelect={pick}
            />
          </div>
        )}
        {endDate !== undefined && (
          <div className="pt-2 pr-6 pl-6">
            <Button variant="clear" size="small" onClick={() => onEndDateChange(undefined)}>
              Убрать дату
            </Button>
          </div>
        )}
      </div>
    </>
  );
}

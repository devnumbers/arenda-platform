'use client';

import type { JSX } from 'react';
import { Check } from '@/shared/assets/icons';
import { recurrenceLabel, type IsoDate, type Recurrence } from '@/entities/payment';
import {
  CalendarButton,
  ListRow,
  MonthDaysGrid,
  WheelPicker,
} from '@/shared/ui/design';
// Именованные константы грида — прямой импорт модуля shared (общие слои —
// точки входа сами по себе).
import { MONTH_LABELS } from '@/shared/ui/design/month-grid';
import {
  PERIODICITY_OPTIONS,
  WEEKDAY_BUTTONS,
  branchKind,
  toggleWeekday,
  type PeriodicityBranch,
  type PeriodicityKind,
} from '@/features/payments';
import { isoDayOfMonth, isoMonthNumber } from '../../lib/calendar-date';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 3 визарда — периодичность без «Один раз» (решение #449, ошибка
 * дизайна Figma): меню день/неделя/месяц/год (Figma 823:11219) и ветки
 * дат — дни недели (830:13354), мини-грид месяца с «последним днем месяца»
 * (823:11422/830:12311), месяц+день для года (829:11606). У ежедневного
 * правила ветки нет — выбор сразу завершает шаг.
 */

export type PeriodicityStepProps = {
  readonly recurrence: Recurrence | undefined;
  readonly openBranch: PeriodicityBranch | null;
  readonly onOpenBranch: (branch: PeriodicityBranch | null) => void;
  /** Замена регулярности целиком. */
  readonly onRecurrenceChange: (recurrence: Recurrence) => void;
  /** Ежедневное правило готово сразу — выбор ведёт на следующий шаг. */
  readonly onDailyPick: () => void;
  /** «Сегодня» клиентской проекции — дефолты якорей вида месяц/год. */
  readonly today: IsoDate;
  /** Заголовки шага рисует хост (шит правки #467 несёт их в ModalContent). */
  readonly withHeading?: boolean;
};

const YEARLY_MONTH_ITEMS = MONTH_LABELS.map((label, index) => ({
  value: String(index + 1),
  label,
}));

function defaultForKind(
  kind: Exclude<PeriodicityKind, 'daily'>,
  today: IsoDate,
): Recurrence {
  const todayDay = isoDayOfMonth(today);
  const todayMonth = isoMonthNumber(today);
  switch (kind) {
    case 'weekly':
      return { kind: 'weekly', weekdays: [] };
    case 'monthly':
      return { kind: 'monthly', dayOfMonth: todayDay };
    case 'yearly':
      return { kind: 'yearly', month: todayMonth, day: todayDay };
  }
}

export function PeriodicityStep({
  recurrence,
  openBranch,
  onOpenBranch,
  onRecurrenceChange,
  onDailyPick,
  today,
  withHeading = true,
}: PeriodicityStepProps): JSX.Element {
  const heading = (title: string, subtitle?: string): JSX.Element | null =>
    withHeading ? <WizardHeading title={title} subtitle={subtitle} /> : null;

  const pickKind = (kind: PeriodicityKind): void => {
    if (kind === 'daily') {
      onRecurrenceChange({ kind: 'daily' });
      onDailyPick();
      return;
    }
    const kept =
      recurrence?.kind === kind ? recurrence : defaultForKind(kind, today);
    onRecurrenceChange(kept);
    // kind ≠ daily здесь гарантирован ранним возвратом выше.
    onOpenBranch(branchKind(kept));
  };

  if (openBranch === null) {
    return (
      <>
        {heading('Периодичность платежа')}
        <div className="flex flex-col pt-2">
          {PERIODICITY_OPTIONS.map((option) => (
            <ListRow
              key={option.kind}
              title={option.label}
              value={
                recurrence !== undefined && recurrence.kind === option.kind
                  ? recurrenceLabel(recurrence)
                  : undefined
              }
              onSelect={() => pickKind(option.kind)}
            />
          ))}
        </div>
      </>
    );
  }

  switch (openBranch) {
    case 'weekdays':
      return (
        <>
          {heading('Выберите день', 'Можно выбрать несколько дней')}
          {/* Грид недели в ширину контента — как мини-грид месяца (823:11422):
              7 квадратов всегда помещаются даже на узких экранах. */}
          <div className="mx-auto grid w-full max-w-[512px] grid-cols-7 gap-2 px-4 pt-4">
            {WEEKDAY_BUTTONS.map((day) => {
              const selectedDays = new Set(
                recurrence?.kind === 'weekly' ? recurrence.weekdays : [],
              );
              return (
                <CalendarButton
                  key={day.value}
                  className="aspect-square h-auto w-full"
                  state={selectedDays.has(day.value) ? 'selected' : 'default'}
                  onClick={() =>
                    onRecurrenceChange({
                      kind: 'weekly',
                      weekdays: toggleWeekday([...selectedDays], day.value),
                    })
                  }
                >
                  {day.label}
                </CalendarButton>
              );
            })}
          </div>
        </>
      );
    case 'monthDays': {
      const dayOfMonth =
        recurrence?.kind === 'monthly' ? recurrence.dayOfMonth : isoDayOfMonth(today);
      return (
        <>
          {heading('Выберите день')}
          <div className="px-2 pt-4">
            <MonthDaysGrid
              days={31}
              selectedDays={new Set([dayOfMonth])}
              onDayToggle={(picked) => onRecurrenceChange({ kind: 'monthly', dayOfMonth: picked })}
            />
          </div>
          <div className="pt-4">
            <ListRow
              title="Последний день месяца"
              onSelect={() => onRecurrenceChange({ kind: 'monthly', dayOfMonth: 31 })}
              trailing={
                dayOfMonth === 31 ? (
                  <Check className="h-5 w-5 text-primary" aria-hidden />
                ) : undefined
              }
            />
          </div>
        </>
      );
    }
    case 'yearly': {
      const month =
        recurrence?.kind === 'yearly' ? recurrence.month : isoMonthNumber(today);
      const day = recurrence?.kind === 'yearly' ? recurrence.day : isoDayOfMonth(today);
      return (
        <>
          {heading('Выберите месяц и день')}
          <div className="flex gap-4 px-6 pt-4">
            <WheelPicker
              items={YEARLY_MONTH_ITEMS}
              value={String(month)}
              onValueChange={(value) =>
                onRecurrenceChange({ kind: 'yearly', month: Number(value), day })
              }
              label="Месяц"
              className="flex-1"
            />
            <MonthDaysGrid
              days={31}
              selectedDays={new Set([day])}
              onDayToggle={(picked) =>
                onRecurrenceChange({ kind: 'yearly', month, day: picked })
              }
              className="flex-1"
            />
          </div>
        </>
      );
    }
  }
};

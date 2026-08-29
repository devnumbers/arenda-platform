'use client';

import type { JSX } from 'react';
import { ArrowRight, CheckBoxFalse, CheckBoxTrue } from '@/shared/assets/icons';
import { type IsoDate, type Recurrence } from '@/entities/payment';
import {
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
 * дизайна Figma): меню день/неделя/месяц/год (Figma 1049:48174 — строки
 * с круглой стрелкой вправо) и ветки дат — дни недели списком с
 * чекбоксами (Figma 1056:52140/52895), мини-грид месяца с «последним
 * днем месяца» (823:11422/830:12311), месяц+день для года (829:11606).
 * У ежедневного правила ветки нет — выбор сразу завершает шаг.
 */

/** Полные названия дней недели для списка ветки недели (значения как в
 * WEEKDAY_BUTTONS: 1..6 — Пн..Сб, 0 — Вс). */
const WEEKDAY_FULL_LABELS: Record<number, string> = {
  1: 'Понедельник',
  2: 'Вторник',
  3: 'Среда',
  4: 'Четверг',
  5: 'Пятница',
  6: 'Суббота',
  0: 'Воскресенье',
};

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
      // По фрейму 1056:53076 ничего не предвыбрано: правило готово после
      // первого дня или отметки последнего дня.
      return { kind: 'monthly', daysOfMonth: [], lastDay: false };
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
        <div className="flex flex-col pt-6">
          {PERIODICITY_OPTIONS.map((option) => (
            <ListRow
              key={option.kind}
              className="py-3.5"
              title={option.label}
              onSelect={() => pickKind(option.kind)}
              trailing={<ArrowRight className="h-6 w-6" aria-hidden />}
            />
          ))}
        </div>
      </>
    );
  }

  switch (openBranch) {
    case 'weekdays': {
      const selectedDays = new Set(
        recurrence?.kind === 'weekly' ? recurrence.weekdays : [],
      );
      return (
        <>
          {heading('Выберите день', 'Можно выбрать несколько дней')}
          <div className="flex flex-col pt-6">
            {WEEKDAY_BUTTONS.map((day) => (
              <ListRow
                key={day.value}
                className="py-3.5"
                title={WEEKDAY_FULL_LABELS[day.value]}
                onSelect={() =>
                  onRecurrenceChange({
                    kind: 'weekly',
                    weekdays: toggleWeekday([...selectedDays], day.value),
                  })
                }
                trailing={
                  selectedDays.has(day.value) ? (
                    <CheckBoxTrue className="h-6 w-6" aria-hidden />
                  ) : (
                    <CheckBoxFalse className="h-6 w-6" aria-hidden />
                  )
                }
              />
            ))}
          </div>
        </>
      );
    }
    case 'monthDays': {
      // Несколько дней месяца (Figma 1056:53076/53382): плоский грид 1..30 на
      // всю ширину, выбор мультивыбором; последний день месяца — отдельный
      // маркер-чекбокс, независимый от дней.
      const current: Recurrence =
        recurrence?.kind === 'monthly'
          ? recurrence
          : { kind: 'monthly', daysOfMonth: [], lastDay: false };
      const days = current.daysOfMonth;
      const toggleDay = (day: number): Recurrence => ({
        kind: 'monthly',
        daysOfMonth: days.includes(day)
          ? days.filter((picked) => picked !== day)
          : [...days, day].sort((a, b) => a - b),
        lastDay: current.lastDay,
      });
      return (
        <>
          {heading('Выберите день', 'Можно выбрать несколько дней')}
          <div className="grid grid-cols-7 gap-2 px-6 pt-6">
            {Array.from({ length: 30 }, (_, index) => index + 1).map((day) => {
              const selected = days.includes(day);
              return (
                <button
                  key={day}
                  type="button"
                  aria-pressed={selected}
                  onClick={() => onRecurrenceChange(toggleDay(day))}
                  className={
                    selected
                      ? 'aspect-square w-full cursor-pointer rounded-xl bg-primary text-base font-medium leading-[18px] text-white outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary'
                      : 'aspect-square w-full cursor-pointer rounded-xl text-base font-medium leading-[18px] text-content outline-none transition-colors hover:bg-surface-muted active:bg-surface-muted-hover focus-visible:ring-2 focus-visible:ring-primary'
                  }
                >
                  {day}
                </button>
              );
            })}
          </div>
          <div className="pt-4">
            <ListRow
              title="Последний день месяца"
              className="py-3.5"
              onSelect={() =>
                onRecurrenceChange({ ...current, lastDay: !current.lastDay })
              }
              trailing={
                current.lastDay ? (
                  <CheckBoxTrue className="h-6 w-6" aria-hidden />
                ) : (
                  <CheckBoxFalse className="h-6 w-6" aria-hidden />
                )
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

'use client';

import type { JSX } from 'react';
import { SmallArrowRight, CheckBoxFalse, CheckBoxTrue } from '@/shared/assets/icons';
import { type IsoDate, type Recurrence } from '@/entities/payment';
import { CalendarDatePicker, ListRow, MonthDaysGrid } from '@/shared/ui/design';
import { isoDayOfMonth, isoMonthNumber } from '@/shared/lib/calendar';
import {
  PERIODICITY_OPTIONS,
  WEEKDAY_BUTTONS,
  periodicityMenuValue,
  pickPeriodicityKind,
  toggleWeekday,
  yearlyAnchorDate,
  type PeriodicityBranch,
  type PeriodicityKind,
} from '@/features/payments';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 3 визарда — периодичность без «Один раз» (решение #449, ошибка
 * дизайна Figma): меню день/неделя/месяц/год (Figma 1049:48174 — строки
 * с круглой стрелкой вправо) и ветки дат — дни недели списком с
 * чекбоксами (Figma 1056:52140/52895), мини-грид месяца — канонный
 * MonthDaysGrid с «последним днем месяца» (1056:53076/53077, тикет
 * #809), год — бесконечный календарь как в
 * задачах (решение владельца 2026-09-04, раньше был грид месяца
 * 829:11606/1056:53547). У ежедневного правила ветки нет — выбор сразу
 * завершает шаг.
 */

/** Подпись типа периода в хедере открытой ветки (Figma 1056:52895):
 * названия совпадают с пунктами меню. Общая для визарда и страницы
 * периодичности экрана правки. */
export const BRANCH_PERIOD_LABELS: Record<PeriodicityBranch, string> = {
  weekdays: 'Каждую неделю',
  monthDays: 'Каждый месяц',
  yearly: 'Каждый год',
};

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
  /** Замена регулярности целиком; undefined — прежняя готовая ветка
   * сброшена (годовая подтверждается календарём, дефект А #948). */
  readonly onRecurrenceChange: (recurrence: Recurrence | undefined) => void;
  /** Ежедневное правило готово сразу — выбор ведёт на следующий шаг. */
  readonly onDailyPick: () => void;
  /** «Сегодня» клиентской проекции — якорь предвыбора годового правила. */
  readonly today: IsoDate;
  /** Заголовки шага рисует хост (шит правки #467 несёт их в ModalContent). */
  readonly withHeading?: boolean;
};

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
    const pick = pickPeriodicityKind(kind, recurrence);
    onRecurrenceChange(pick.recurrence);
    if (pick.branch === null) {
      onDailyPick();
      return;
    }
    onOpenBranch(pick.branch);
  };

  if (openBranch === null) {
    // Видимый выбор (дефект Б #948, канон пикера аренды): готовая
    // периодичность читается в строке своего пункта — «Каждый год ·
    // 15 октября»; неготовая ветка значения не имеет.
    const selectedValue = periodicityMenuValue(recurrence);
    return (
      <>
        {heading('Периодичность платежа')}
        <div className="flex flex-col pt-6">
          {PERIODICITY_OPTIONS.map((option) => (
            <ListRow
              key={option.kind}
              className="py-3.5"
              title={
                selectedValue !== undefined && recurrence?.kind === option.kind
                  ? `${option.label} · ${selectedValue}`
                  : option.label
              }
              onSelect={() => pickKind(option.kind)}
              trailing={<SmallArrowRight className="h-6 w-6" aria-hidden />}
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
      // Несколько дней месяца (Figma 1056:53076/53077): канонный мини-грид
      // MonthDaysGrid — те же клетки, что в пикере дня оплаты аренды;
      // мультивыбор 1..30 (31-е закрывается «последним днем месяца»).
      // Последний день — отдельный маркер-чекбокс, независимый от дней.
      const current: Recurrence =
        recurrence?.kind === 'monthly'
          ? recurrence
          : { kind: 'monthly', daysOfMonth: [], lastDay: false };
      const toggleDay = (day: number): Recurrence => ({
        kind: 'monthly',
        daysOfMonth: current.daysOfMonth.includes(day)
          ? current.daysOfMonth.filter((picked) => picked !== day)
          : [...current.daysOfMonth, day].sort((a, b) => a - b),
        lastDay: current.lastDay,
      });
      return (
        <>
          {heading('Выберите день', 'Можно выбрать несколько дней')}
          {/* Отступы по макету: 16px от заголовка к гриду, 8px от грида
              к строке последнего дня (узлы 1056:53375/53081). */}
          <div className="pt-4">
            <MonthDaysGrid
              days={30}
              selectedDays={new Set(current.daysOfMonth)}
              onDayToggle={(day) => onRecurrenceChange(toggleDay(day))}
            />
          </div>
          <div className="pt-2">
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
    case 'yearly':
      return (
        <YearlyBranch
          recurrence={recurrence?.kind === 'yearly' ? recurrence : undefined}
          today={today}
          onChange={onRecurrenceChange}
          onClose={() => onOpenBranch(null)}
        />
      );
  }
};

/** Годовая ветка «Каждый год» — канонический бесконечный календарь
 * CalendarDatePicker (как выбор даты в задачах; решение владельца
 * 2026-09-04 вместо чипа «Месяц Год» с колесами и грида одного месяца
 * Figma 1056:53547/50518): лента месяцев вперёд, прошлого нет. Дата
 * обязательна (required) — правила «без даты» не существует. Год в
 * правиле не хранится (yearly = месяц и день): от подтверждённой даты
 * остаются месяц и день; существующее правило календарь предвыбирает
 * ближайшим будущим вхождением (yearlyAnchorDate). */
function YearlyBranch({
  recurrence,
  today,
  onChange,
  onClose,
}: {
  readonly recurrence: Recurrence | undefined;
  readonly today: IsoDate;
  readonly onChange: (recurrence: Recurrence) => void;
  readonly onClose: () => void;
}): JSX.Element {
  return (
    <CalendarDatePicker
      required
      today={today}
      value={recurrence?.kind === 'yearly' ? yearlyAnchorDate(recurrence, today) : null}
      onClose={onClose}
      onConfirm={(date) => {
        // required исключает пустое подтверждение — страховка контракта.
        if (date === null) return;
        onChange({ kind: 'yearly', month: isoMonthNumber(date), day: isoDayOfMonth(date) });
        onClose();
      }}
    />
  );
}

'use client';

import type { JSX } from 'react';
import { SmallArrowRight, CheckBoxFalse, CheckBoxTrue } from '@/shared/assets/icons';
import { type IsoDate, type Recurrence } from '@/entities/payment';
import { CalendarDatePicker, ListRow, MonthDaysGrid } from '@/shared/ui/design';
import { isoDayOfMonth, isoMonthNumber } from '@/shared/lib/calendar';
import {
  PERIODICITY_OPTIONS,
  WEEKDAY_BUTTONS,
  pickPeriodicityKind,
  toggleWeekday,
  yearlyAnchorDate,
  type PeriodicityBranch,
  type PeriodicityKind,
} from '@/features/payments';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 3 визарда — периодичность без «Один раз» (решение #449, ошибка
 * дизайна Figma): меню день/неделя/месяц/год (Figma 3213:71290 — H1-заголовок
 * канона #1152, строки с шевронами, 16px от заголовка к списку) и ветки
 * дат — дни недели списком с чекбоксами (Figma 3213:71312), мини-грид
 * месяца — канонный MonthDaysGrid с «последним днем месяца» (3213:71350,
 * тикет #809), год — бесконечный календарь как в
 * задачах (решение владельца 2026-09-04, раньше был грид месяца
 * 829:11606/1056:53547); подтверждение года — кнопкой «Продолжить»
 * календаря и сразу завершает шаг у любого хоста (решение владельца
 * 2026-09-30, #995; правка выровнена с созданием — #1153): визард
 * ведёт на следующий шаг, правка применяет правило и возвращается на
 * форму. У ежедневного правила ветки нет — выбор сразу завершает шаг.
 */

/** Подпись типа периода в хедере открытой ветки (Figma 3213:71312):
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
  /** Ежедневное правило готово сразу — выбор завершает шаг: визард ведёт
   * на следующий шаг, правка применяет правило и закрывает страницу
   * (выровнено с созданием, #1153). */
  readonly onDailyPick: () => void;
  /** Годовое правило подтверждено календарём («Продолжить») — хост
   * завершает шаг с подтверждённым правилом: визард ведёт на следующий
   * шаг (решение владельца 30.09.2026, #995), правка применяет правило
   * и возвращается на форму (#1153). Правило передаётся явно: черновик
   * хоста на момент колбэка ещё не обновлён. */
  readonly onYearlyConfirm: (recurrence: Recurrence) => void;
  /** «Сегодня» клиентской проекции — якорь предвыбора годового правила. */
  readonly today: IsoDate;
  /** Заголовки шага (H1-канон #1152) рисует компонент; хост правки их
   * глушит — там контентных заголовков нет (решение владельца 07.10,
   * #1192). */
  readonly withHeading?: boolean;
};

export function PeriodicityStep({
  recurrence,
  openBranch,
  onOpenBranch,
  onRecurrenceChange,
  onDailyPick,
  onYearlyConfirm,
  today,
  withHeading = true,
}: PeriodicityStepProps): JSX.Element {
  const heading = (title: string, subtitle?: string): JSX.Element | null =>
    withHeading ? <WizardHeading title={title} subtitle={subtitle} variant="h1" /> : null;

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
    return (
      <>
        {heading('Периодичность платежа')}
        {/* 16px от заголовка к списку (кадр 3213:71292, gap 16). */}
        <div className="flex flex-col pt-4">
          {PERIODICITY_OPTIONS.map((option) => (
            <ListRow
              key={option.kind}
              className="py-3.5"
              title={option.label}
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
          {/* 16px от заголовка к списку (кадр 3213:71312). */}
          <div className="flex flex-col pt-4">
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
              к строке последнего дня (кадр 3213:71350). */}
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
          onYearlyConfirm={onYearlyConfirm}
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
 * ближайшим будущим вхождением (yearlyAnchorDate). Кнопка календаря —
 * «Продолжить»: подтверждение пишет правило хосту (onChange) и сразу
 * завершает шаг (onYearlyConfirm) — визард ведёт следующий шаг
 * (решение владельца 30.09.2026, #995), правка применяет правило и
 * возвращается на форму (#1153). Закрытие без подтверждения — «Назад»
 * пикера, ветку закрывает хост (onClose). */
function YearlyBranch({
  recurrence,
  today,
  onChange,
  onYearlyConfirm,
  onClose,
}: {
  readonly recurrence: Recurrence | undefined;
  readonly today: IsoDate;
  readonly onChange: (recurrence: Recurrence | undefined) => void;
  readonly onYearlyConfirm: (recurrence: Recurrence) => void;
  readonly onClose: () => void;
}): JSX.Element {
  return (
    <CalendarDatePicker
      required
      today={today}
      confirmLabel="Продолжить"
      value={recurrence?.kind === 'yearly' ? yearlyAnchorDate(recurrence, today) : null}
      onClose={onClose}
      onConfirm={(date) => {
        // required исключает пустое подтверждение — страховка контракта.
        if (date === null) return;
        const yearly: Recurrence = {
          kind: 'yearly',
          month: isoMonthNumber(date),
          day: isoDayOfMonth(date),
        };
        onChange(yearly);
        onYearlyConfirm(yearly);
      }}
    />
  );
}

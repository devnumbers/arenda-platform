'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ArrowRight, CheckBoxFalse, CheckBoxTrue, ChevronDown } from '@/shared/assets/icons';
import { type IsoDate, type Recurrence } from '@/entities/payment';
import {
  ListRow,
  MonthYearPicker,
} from '@/shared/ui/design';
import { type CalendarMonthRef, calendarMonthOf } from '@/shared/lib/calendar';
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
import { isoDayOfMonth, isoMonthNumber, isoYear } from '@/shared/lib/calendar';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 3 визарда — периодичность без «Один раз» (решение #449, ошибка
 * дизайна Figma): меню день/неделя/месяц/год (Figma 1049:48174 — строки
 * с круглой стрелкой вправо) и ветки дат — дни недели списком с
 * чекбоксами (Figma 1056:52140/52895), мини-грид месяца с «последним
 * днем месяца» (823:11422/830:12311), месяц+день для года (829:11606).
 * У ежедневного правила ветки нет — выбор сразу завершает шаг.
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
  /** Замена регулярности целиком. */
  readonly onRecurrenceChange: (recurrence: Recurrence) => void;
  /** Ежедневное правило готово сразу — выбор ведёт на следующий шаг. */
  readonly onDailyPick: () => void;
  /** «Сегодня» клиентской проекции — дефолты якорей вида месяц/год. */
  readonly today: IsoDate;
  /** Заголовки шага рисует хост (шит правки #467 несёт их в ModalContent). */
  readonly withHeading?: boolean;
};

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
    if (kind === 'yearly') {
      // Годовая ветка открывается без предвыбора (Figma 1056:53547):
      // правило пишется после выбора дня, дефолт не создаётся.
      onOpenBranch('yearly');
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
    case 'yearly':
      return (
        <YearlyBranch
          recurrence={recurrence?.kind === 'yearly' ? recurrence : undefined}
          today={today}
          onChange={onRecurrenceChange}
        />
      );
  }
};

/** Календарь одного месяца ветки «Каждый год» (Figma 1056:53547/50518):
 * чип «Месяц Год» открывает канонический шит-пикер MonthYearPicker —
 * колесо месяцев и бесконечное колесо годов не раньше текущего (как в
 * задачах); ниже — календарь просматриваемого месяца, день выбирается
 * кликом. Год в правиле не хранится (yearly = месяц и день) — он якорит
 * просмотр, чтобы день нельзя было выбрать задним числом. Бесконечная
 * лента месяцев подряд (как в макете) не делается — месяц один, по
 * решению владельца. */
const WEEKDAY_HEADERS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'] as const;

type YearMonth = { readonly year: number; readonly month0: number };

function monthLength(year: number, month0: number): number {
  return new Date(Date.UTC(year, month0 + 1, 0)).getUTCDate();
}

function firstWeekdayMon0(year: number, month0: number): number {
  return (new Date(Date.UTC(year, month0, 1)).getUTCDay() + 6) % 7;
}

export type YearMonthCalendarValue = {
  readonly year: number;
  readonly month0: number;
  readonly day: number;
};

/** Календарь одного месяца с чипом «Месяц Год» и шитом-пикером (Figma
 * 1056:53547/50518): используется в годовой ветке и в выборе даты
 * окончания. День выбирается кликом; дни раньше minDate запрещены
 * (задним числом не выбираются). Год в значении настоящий — что видеть
 * в календаре и писать в чипе. Шит месяца и года — канонический
 * MonthYearPicker (как в задачах, унификация 2026-09-04). */
export function YearMonthCalendar({
  value,
  today,
  minDate,
  onPick,
  padded = true,
}: {
  readonly value: YearMonthCalendarValue | undefined;
  readonly today: IsoDate;
  /** Дни раньше этой даты недоступны. */
  readonly minDate?: IsoDate;
  readonly onPick: (value: YearMonthCalendarValue) => void;
  /** Свои горизонатльные отступы px-6: на странице — да, внутри модалки
   * (p-6 уже есть) — нет. */
  readonly padded?: boolean;
}): JSX.Element {
  const todayYear = isoYear(today);
  const todayMonth0 = isoMonthNumber(today) - 1;
  const [view, setView] = useState<YearMonth | null>(null);
  const [wheelOpen, setWheelOpen] = useState(false);

  const shown = view ?? initialYearMonthView(value, todayYear, todayMonth0);
  const shownLength = monthLength(shown.year, shown.month0);
  const selectedDay =
    value !== undefined && value.year === shown.year && value.month0 === shown.month0
      ? value.day
      : undefined;

  // Шит месяца/года: нижняя граница — календарный месяц minDate (или
  // «сегодня»), как в задачах: прошлые годы и месяцы в шите отсутствуют.
  const min: CalendarMonthRef =
    minDate !== undefined ? calendarMonthOf(minDate) : { year: todayYear, month0: todayMonth0 };

  return (
    <>
      <div className={padded ? 'px-6 pt-4' : 'pt-1'}>
        <button
          type="button"
          onClick={() => setWheelOpen(true)}
          className="inline-flex h-11 cursor-pointer items-center gap-2 rounded-pill bg-surface-muted px-5 text-base font-medium text-content outline-none transition-colors hover:bg-surface-muted-hover active:bg-surface-muted-hover focus-visible:ring-2 focus-visible:ring-primary"
        >
          {MONTH_LABELS[shown.month0]} {shown.year}
          <ChevronDown className="h-6 w-6 text-content-secondary" aria-hidden />
        </button>
      </div>

      <div className={padded ? 'grid grid-cols-7 gap-2 px-6' : 'grid grid-cols-7 gap-2'}>
        {WEEKDAY_HEADERS.map((name) => (
          <span
            key={name}
            className="text-center text-[13px] leading-[15px] text-content-secondary"
          >
            {name}
          </span>
        ))}
        {Array.from({ length: firstWeekdayMon0(shown.year, shown.month0) }, (_, index) => (
          <span key={`pad-${index}`} aria-hidden />
        ))}
        {Array.from({ length: shownLength }, (_, index) => index + 1).map((day) => {
          const iso = isoDateOf(shown.year, shown.month0, day);
          const disabled = minDate !== undefined && iso < minDate;
          const isToday =
            shown.year === todayYear && shown.month0 === todayMonth0 && day === isoDayOfMonth(today);
          const selected = selectedDay === day;
          return (
            <button
              key={day}
              type="button"
              aria-pressed={selected}
              disabled={disabled}
              onClick={() => onPick({ year: shown.year, month0: shown.month0, day })}
              className={
                selected
                  ? 'aspect-square w-full cursor-pointer rounded-xl bg-primary text-base font-medium leading-[18px] text-white outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary'
                  : isToday
                    ? 'aspect-square w-full cursor-pointer rounded-xl bg-primary/10 text-base font-medium leading-[18px] text-content outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent'
                    : 'aspect-square w-full cursor-pointer rounded-xl text-base font-medium leading-[18px] text-content outline-none transition-colors hover:bg-surface-muted active:bg-surface-muted-hover focus-visible:ring-2 focus-visible:ring-primary disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent'
              }
            >
              {day}
            </button>
          );
        })}
      </div>
      <MonthYearPicker
        open={wheelOpen}
        onOpenChange={setWheelOpen}
        month={shown.month0}
        year={shown.year}
        min={min}
        onConfirm={(month0, year) => {
          setView({ year, month0 });
          setWheelOpen(false);
        }}
      />
    </>
  );
}

function isoDateOf(year: number, month0: number, day: number): IsoDate {
  return new Date(Date.UTC(year, month0, day)).toISOString().slice(0, 10);
}

function initialYearMonthView(
  value: YearMonthCalendarValue | undefined,
  todayYear: number,
  todayMonth0: number,
): YearMonth {
  if (value !== undefined) {
    return { year: value.year, month0: value.month0 };
  }
  return { year: todayYear, month0: todayMonth0 };
}

/** Годовая ветка: обёртка календаря — выбранный год не хранится в правиле
 * (yearly = месяц и день), но нужен календарю; для месяца в прошлом
 * показываем ближайший будущий год. */
function YearlyBranch({
  recurrence,
  today,
  onChange,
}: {
  readonly recurrence: Recurrence | undefined;
  readonly today: IsoDate;
  readonly onChange: (recurrence: Recurrence) => void;
}): JSX.Element {
  const todayYear = isoYear(today);
  const todayMonth0 = isoMonthNumber(today) - 1;
  const [picked, setPicked] = useState<YearMonthCalendarValue | undefined>(() => {
    if (recurrence?.kind !== 'yearly') return undefined;
    const month0 = recurrence.month - 1;
    return {
      year: month0 >= todayMonth0 ? todayYear : todayYear + 1,
      month0,
      day: recurrence.day,
    };
  });

  return (
    <YearMonthCalendar
      value={picked}
      today={today}
      onPick={(value) => {
        setPicked(value);
        onChange({ kind: 'yearly', month: value.month0 + 1, day: value.day });
      }}
    />
  );
}

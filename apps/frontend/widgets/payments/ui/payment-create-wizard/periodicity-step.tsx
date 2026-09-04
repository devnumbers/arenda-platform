'use client';

import { useState, type Dispatch, type SetStateAction } from 'react';
import type { JSX } from 'react';
import { ArrowRight, CheckBoxFalse, CheckBoxTrue, ChevronDown } from '@/shared/assets/icons';
import { type IsoDate, type Recurrence } from '@/entities/payment';
import {
  Button,
  ListRow,
  Modal,
  ModalContent,
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
 * чип «Месяц Год» открывает шит-пикер — колесо месяцев с бесконечной
 * лентой (все 12 подряд) и колесо годов не раньше текущего; ниже —
 * календарь просматриваемого месяца, день выбирается кликом. Год в
 * правиле не хранится (yearly = месяц и день) — он якорит просмотр,
 * чтобы день нельзя было выбрать задним числом. Бесконечная лента
 * месяцев подряд (как в макете) не делается — месяц один, по решению
 * владельца. */
const WEEKDAY_HEADERS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'] as const;
const MONTH_WHEEL_HALF = 126;

type YearMonth = { readonly year: number; readonly month0: number };

function monthLength(year: number, month0: number): number {
  return new Date(Date.UTC(year, month0 + 1, 0)).getUTCDate();
}

function firstWeekdayMon0(year: number, month0: number): number {
  return (new Date(Date.UTC(year, month0, 1)).getUTCDay() + 6) % 7;
}

/** Лента месяцев для колеса пикера: текущий месяц ровно в центре
 * (±126 рядов), слева — остальные 11 по кругу без текущего, справа —
 * все 12 по кругу. Значения кодируют позицию «месяц:ряд», чтобы колесо
 * не прыгало к первому повтору при выборе. */
function monthWheelItems(centerMonth0: number): ReadonlyArray<{
  readonly value: string;
  readonly label: string;
}> {
  const items: { value: string; label: string }[] = [];
  for (let offset = -MONTH_WHEEL_HALF; offset <= MONTH_WHEEL_HALF; offset++) {
    const month0 =
      offset === 0
        ? centerMonth0
        : offset < 0
          ? (centerMonth0 - 1 - ((-offset - 1) % 11) + 132) % 12
          : (centerMonth0 + offset) % 12;
    items.push({
      value: `${month0}:${offset}`,
      label: MONTH_LABELS[month0] ?? '',
    });
  }
  return items;
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
 * в календаре и писать в чипе. */
export function YearMonthCalendar({
  value,
  today,
  minDate,
  onPick,
  wheelAsModal = true,
  padded = true,
}: {
  readonly value: YearMonthCalendarValue | undefined;
  readonly today: IsoDate;
  /** Дни раньше этой даты недоступны. */
  readonly minDate?: IsoDate;
  readonly onPick: (value: YearMonthCalendarValue) => void;
  /** Колесо месяца/года отдельной модалкой (по умолчанию) или подменой
   * контента — когда календарь уже внутри модалки. */
  readonly wheelAsModal?: boolean;
  /** Свои горизонатльные отступы px-6: на странице — да, внутри модалки
   * (p-6 уже есть) — нет. */
  readonly padded?: boolean;
}): JSX.Element {
  const todayYear = isoYear(today);
  const todayMonth0 = isoMonthNumber(today) - 1;
  const [view, setView] = useState<YearMonth | null>(null);
  const [pickerDraft, setPickerDraft] = useState<
    (YearMonth & { readonly monthValue: string; readonly centerMonth0: number }) | null
  >(null);
  const [yearsCount, setYearsCount] = useState(11);
  const wheelOpen = pickerDraft !== null;

  const shown = view ?? initialYearMonthView(value, todayYear, todayMonth0);
  const shownLength = monthLength(shown.year, shown.month0);
  const selectedDay =
    value !== undefined && value.year === shown.year && value.month0 === shown.month0
      ? value.day
      : undefined;

  const minYear = minDate !== undefined ? isoYear(minDate) : todayYear;

  return (
    <>
      {/* В модальном режиме календарь остаётся на странице под оверлеем
          шита; подмена контента — только для режима «календарь уже внутри
          модалки» (wheelAsModal=false). */}
      {(!wheelOpen || wheelAsModal) && (
      <>
      <div className={padded ? 'px-6 pt-4' : 'pt-1'}>
        <button
          type="button"
          onClick={() =>
            setPickerDraft({
              year: shown.year,
              month0: shown.month0,
              monthValue: `${shown.month0}:0`,
              centerMonth0: shown.month0,
            })
          }
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
      </>
      )}
      {wheelAsModal ? (
        <Modal open={wheelOpen} onOpenChange={(open) => !open && setPickerDraft(null)}>
          <ModalContent title="Месяц и год" titleSrOnly>
            {pickerDraft !== null && (
              <YearWheelBody
                pickerDraft={pickerDraft}
                setPickerDraft={setPickerDraft}
                yearsCount={yearsCount}
                setYearsCount={setYearsCount}
                minYear={minYear}
                onApply={(year, month0) => {
                  setView({ year, month0 });
                  setPickerDraft(null);
                }}
              />
            )}
          </ModalContent>
        </Modal>
      ) : (
        wheelOpen && (
          <>
            <YearWheelBody
              pickerDraft={pickerDraft}
              setPickerDraft={setPickerDraft}
              yearsCount={yearsCount}
              setYearsCount={setYearsCount}
              minYear={minYear}
              onApply={(year, month0) => {
                setView({ year, month0 });
                setPickerDraft(null);
              }}
            />
          </>
        )
      )}
    </>
  );
}

/** Колёса месяца и года с кнопками: «Отменить» закрывает колёса (в режиме
 * подмены — возвращает к календарю), «Выбрать» применяет месяц и год. */
type PickerDraft = YearMonth & {
  readonly monthValue: string;
  readonly centerMonth0: number;
};

function YearWheelBody({
  pickerDraft,
  setPickerDraft,
  yearsCount,
  setYearsCount,
  minYear,
  onApply,
}: {
  readonly pickerDraft: PickerDraft;
  readonly setPickerDraft: Dispatch<SetStateAction<PickerDraft | null>>;
  readonly yearsCount: number;
  readonly setYearsCount: Dispatch<SetStateAction<number>>;
  readonly minYear: number;
  readonly onApply: (year: number, month0: number) => void;
}): JSX.Element {
  return (
    <>
      <div className="flex gap-4">
        {/* Значение ряда возвращается в колесо ровно тем, что проскроллил
            пользователь: пересборка ленты вокруг нового месяца и подстановка
            «{месяц}:0» заставили бы колесо прыгнуть к чужому ряду (баг
            «декабрь → январь даёт ноябрь»). */}
        <WheelPicker
          items={monthWheelItems(pickerDraft.centerMonth0)}
          value={pickerDraft.monthValue}
          onValueChange={(monthValue) =>
            setPickerDraft((prev) => {
              if (prev === null) return prev;
              const month0 = Number(monthValue.split(':')[0]);
              return Number.isNaN(month0) ? prev : { ...prev, monthValue, month0 };
            })
          }
          label="Месяц"
          className="flex-1"
        />
        <WheelPicker
          items={Array.from({ length: yearsCount }, (_, index) => ({
            value: String(minYear + index),
            label: String(minYear + index),
          }))}
          value={String(pickerDraft.year)}
          onValueChange={(value) => {
            const year = Number(value);
            setPickerDraft((prev) => (prev === null ? prev : { ...prev, year }));
            // Выбрал предпоследний/последний год — добавляем ещё десять.
            if (year >= minYear + yearsCount - 3) {
              setYearsCount((count) => count + 10);
            }
          }}
          label="Год"
          className="flex-1"
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Button variant="secondary" className="w-full" onClick={() => setPickerDraft(null)}>
          Отменить
        </Button>
        <Button
          className="w-full"
          onClick={() => onApply(pickerDraft.year, pickerDraft.month0)}
        >
          Выбрать
        </Button>
      </div>
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

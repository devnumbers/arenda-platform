'use client';

import { useState, type JSX } from 'react';
import { clampMonthToMin, type CalendarMonthRef } from '@/shared/lib/calendar';
import { MONTH_LABELS } from './month-grid';
import {
  WheelPicker,
  type WheelPickerItem,
} from './wheel-picker';
import {
  WheelPickerSheet,
  type WheelPickerSheetAction,
} from './wheel-picker-sheet';

/** Шит выбора месяца и года двумя колёсами (Figma 848:8720, редизайн
 * 2026-09-04 на общий WheelPickerSheet по Figma 1539-82659). Колёса меняют
 * черновик, кнопки коммитят/закрывают разом: экран перелистывается на блок
 * с выбранным месяцем только по «Выбрать», «Отменить» закрывает без
 * изменений (пара кнопок — по макету 1539-82659, решение владельца).
 * Без min — годы ±3 вокруг выбранного. С min — нижняя граница (пикер даты
 * задач: будущее без прошлого): годы раньше min.year отсутствуют, в году
 * min — месяцы раньше min.month0; список годов расширяется вперёд
 * бесконечно — колесо удлиняется на 10 лет, когда прокрутка доезжает до
 * края (onNearEnd WheelPicker). Черновик живёт, пока шит смонтирован:
 * при open=false шит не рендерится. */
export type MonthYearPickerProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  /** 0..11, как у Date. */
  readonly month: number;
  readonly year: number;
  /** Нижняя граница выбора: ничего раньше этого месяца. */
  readonly min?: CalendarMonthRef;
  readonly onConfirm: (month: number, year: number) => void;
  readonly className?: string;
};

const YEARS_EXTENSION = 10;

export function MonthYearPicker({
  open,
  onOpenChange,
  month,
  year,
  min,
  onConfirm,
  className,
}: MonthYearPickerProps): JSX.Element | null {
  if (!open) {
    // Черновик колёс не переживает закрытие — тело смонтировано только
    // в открытом шите.
    return null;
  }
  return (
    <MonthYearSheet
      month={month}
      year={year}
      min={min}
      onConfirm={onConfirm}
      onClose={() => onOpenChange(false)}
      className={className}
    />
  );
}

/** Тело шита: черновик — локальное состояние, живёт при открытом шите. */
function MonthYearSheet({
  month,
  year,
  min,
  onConfirm,
  onClose,
  className,
}: {
  readonly month: number;
  readonly year: number;
  readonly min?: CalendarMonthRef;
  readonly onConfirm: (month: number, year: number) => void;
  readonly onClose: () => void;
  readonly className?: string;
}): JSX.Element {
  const [draftMonth, setDraftMonth] = useState(month);
  const [draftYear, setDraftYear] = useState(year);
  // Лента годов вперёд без конца: от нижней границы (min.year либо
  // год-3) до расширяемого края — onNearEnd колеса удлиняет её на 10.
  const [yearsEnd, setYearsEnd] = useState(() => year + 3);
  const yearStart = min?.year ?? year - 3;
  const yearItems: ReadonlyArray<WheelPickerItem> = Array.from(
    { length: Math.max(yearsEnd - yearStart + 1, 1) },
    (_, index) => {
      const value = yearStart + index;
      return { value: String(value), label: String(value) };
    },
  );
  // В минимальном году месяцы раньше границы не существуют — колесо
  // начинается с min.month0; значения — абсолютные 0..11.
  const minMonth0 = min !== undefined && draftYear === min.year ? min.month0 : 0;
  const monthItems: ReadonlyArray<WheelPickerItem> = MONTH_LABELS.map((label, index) => ({
    value: String(index),
    label,
  })).slice(minMonth0);
  const draftDisplayMonth =
    min !== undefined ? clampMonthToMin(min, draftYear, draftMonth) : draftMonth;

  const handleYearChange = (value: string): void => {
    const nextYear = Number(value);
    setDraftYear(nextYear);
    // Перескок в минимальный год прижимает месяц к границе — колесо
    // месяцев перестраивается, черновик не остаётся в недоступном.
    if (min !== undefined) {
      setDraftMonth((current) => clampMonthToMin(min, nextYear, current));
    }
  };

  const actions: ReadonlyArray<WheelPickerSheetAction> = [
    { label: 'Отменить', variant: 'secondary', onSelect: onClose },
    { label: 'Выбрать', onSelect: () => onConfirm(draftDisplayMonth, draftYear) },
  ];

  return (
    <WheelPickerSheet
      title="Месяц и год"
      open
      onOpenChange={(next) => {
        if (!next) {
          onClose();
        }
      }}
      actions={actions}
      className={className}
      columns={[
        <WheelPicker
          key="month"
          label="Месяц"
          strip={false}
          items={monthItems}
          value={String(draftDisplayMonth)}
          onValueChange={(value) => setDraftMonth(Number(value))}
        />,
        <WheelPicker
          key="year"
          label="Год"
          strip={false}
          items={yearItems}
          value={String(draftYear)}
          onValueChange={handleYearChange}
          onNearEnd={() => setYearsEnd((end) => end + YEARS_EXTENSION)}
        />,
      ]}
    />
  );
}

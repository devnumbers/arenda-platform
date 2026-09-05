'use client';

import { useState, type JSX } from 'react';
import {
  clampMonthToMax,
  clampMonthToMin,
  type CalendarMonthRef,
} from '@/shared/lib/calendar';
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
 * Границы независимы: min закрывает прошлое (пикер даты задач), max —
 * будущее (пикер периода операций, решение владельца 2026-09-04); без
 * границ — годы ±3 вокруг выбранного. У края без границы колесо годов
 * удлиняется на 10 лет (onNearEnd вперёд, onNearStart назад). Черновик
 * живёт, пока шит смонтирован: при open=false шит не рендерится. */
export type MonthYearPickerProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  /** 0..11, как у Date. */
  readonly month: number;
  readonly year: number;
  /** Нижняя граница выбора: ничего раньше этого месяца. */
  readonly min?: CalendarMonthRef;
  /** Верхняя граница выбора: ничего позже этого месяца. */
  readonly max?: CalendarMonthRef;
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
  max,
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
      max={max}
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
  max,
  onConfirm,
  onClose,
  className,
}: {
  readonly month: number;
  readonly year: number;
  readonly min?: CalendarMonthRef;
  readonly max?: CalendarMonthRef;
  readonly onConfirm: (month: number, year: number) => void;
  readonly onClose: () => void;
  readonly className?: string;
}): JSX.Element {
  const [draftMonth, setDraftMonth] = useState(month);
  const [draftYear, setDraftYear] = useState(year);
  // Лента годов тянется за края без границы: у края с min — от min.year
  // с удлинением вперёд (onNearEnd), у края с max — до max.year
  // с удлинением назад (onNearStart); без границ — ±3 вокруг выбранного.
  const [yearsStart, setYearsStart] = useState(() => year - 3);
  const [yearsEnd, setYearsEnd] = useState(() => year + 3);
  const startYear = min?.year ?? yearsStart;
  const endYear = max?.year ?? yearsEnd;
  const yearItems: ReadonlyArray<WheelPickerItem> = Array.from(
    { length: Math.max(endYear - startYear + 1, 1) },
    (_, index) => {
      const value = startYear + index;
      return { value: String(value), label: String(value) };
    },
  );
  // В граничном году месяцы за границей не существуют: колесо обрезается
  // с обеих сторон; значения — абсолютные 0..11.
  const minMonth0 = min !== undefined && draftYear === min.year ? min.month0 : 0;
  const maxMonth0 = max !== undefined && draftYear === max.year ? max.month0 : 11;
  const monthItems: ReadonlyArray<WheelPickerItem> = MONTH_LABELS.map((label, index) => ({
    value: String(index),
    label,
  })).slice(minMonth0, maxMonth0 + 1);
  // Прижатие черновика к обеим границам (какая задана): перескок года в
  // граничный поджимает месяц — черновик не остаётся в недоступном.
  const clampDraftMonth = (yearValue: number, monthValue: number): number => {
    let bounded = monthValue;
    if (min !== undefined) {
      bounded = clampMonthToMin(min, yearValue, bounded);
    }
    if (max !== undefined) {
      bounded = clampMonthToMax(max, yearValue, bounded);
    }
    return bounded;
  };
  const draftDisplayMonth = clampDraftMonth(draftYear, draftMonth);

  const handleYearChange = (value: string): void => {
    const nextYear = Number(value);
    setDraftYear(nextYear);
    setDraftMonth((current) => clampDraftMonth(nextYear, current));
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
          onNearStart={() => setYearsStart((start) => start - YEARS_EXTENSION)}
        />,
      ]}
    />
  );
}

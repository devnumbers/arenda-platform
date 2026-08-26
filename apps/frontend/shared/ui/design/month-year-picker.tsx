'use client';

import { useState, type JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Button } from './button';
import { MONTH_LABELS } from './month-grid';
import { WheelPicker, type WheelPickerItem } from './wheel-picker';

/** Пикер месяц/год двумя колёсами (тикет #459, Figma 848:8720) — содержимое
 * шита выбора месяца и года для календаря. Колёса меняют черновик, кнопка
 * «Выбрать» коммитит оба значения разом: экран перелистывается на блок с
 * выбранным месяцем только по кнопке. Годы по умолчанию — ±3 от
 * выбранного года, как в макете (2023..2029 вокруг 2026). Колёса стоят
 * вплотную: серые полосы выбора сливаются в одну ленту на всю ширину. */
export type MonthYearPickerProps = {
  /** 0..11, как у Date. */
  readonly month: number;
  readonly year: number;
  readonly years?: ReadonlyArray<number>;
  readonly onConfirm: (month: number, year: number) => void;
  readonly confirmLabel?: string;
  readonly className?: string;
};

export function MonthYearPicker({
  month,
  year,
  years,
  onConfirm,
  confirmLabel = 'Выбрать',
  className,
}: MonthYearPickerProps): JSX.Element {
  const [draftMonth, setDraftMonth] = useState(month);
  const [draftYear, setDraftYear] = useState(year);

  const yearList = years ?? Array.from({ length: 7 }, (_, index) => year - 3 + index);
  const monthItems: ReadonlyArray<WheelPickerItem> = MONTH_LABELS.map((label, index) => ({
    value: String(index),
    label,
  }));
  const yearItems: ReadonlyArray<WheelPickerItem> = yearList.map((value) => ({
    value: String(value),
    label: String(value),
  }));

  return (
    <div className={cn('flex flex-col gap-6', className)}>
      <div className="flex items-stretch">
        <WheelPicker
          className="min-w-0 flex-1"
          label="Месяц"
          items={monthItems}
          value={String(draftMonth)}
          onValueChange={(value) => setDraftMonth(Number(value))}
        />
        <WheelPicker
          className="min-w-0 flex-1"
          label="Год"
          items={yearItems}
          value={String(draftYear)}
          onValueChange={(value) => setDraftYear(Number(value))}
        />
      </div>
      <Button
        className="w-full"
        onClick={() => onConfirm(draftMonth, draftYear)}
      >
        {confirmLabel}
      </Button>
    </div>
  );
}

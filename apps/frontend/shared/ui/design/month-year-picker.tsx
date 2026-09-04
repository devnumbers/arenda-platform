'use client';

import { useState, type JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Button } from './button';
import { clampMonthToMin, type CalendarMonthRef } from './calendar-feed';
import { MONTH_LABELS } from './month-grid';
import { WheelPicker, type WheelPickerItem } from './wheel-picker';

/** Пикер месяц/год двумя колёсами (тикет #459, Figma 848:8720) — содержимое
 * шита выбора месяца и года для календаря. Колёса меняют черновик, кнопка
 * «Выбрать» коммитит оба значения разом: экран перелистывается на блок с
 * выбранным месяцем только по кнопке. Колёса стоят вплотную: серые полосы
 * выбора сливаются в одну ленту на всю ширину.
 *
 * Без min — годы ±3 вокруг выбранного, как в макете (2023..2029 вокруг
 * 2026). С min — нижняя граница (пикер даты задач: будущее без прошлого):
 * годы раньше min.year отсутствуют, в году min — месяцы раньше min.month0;
 * список годов расширяется вперёд бесконечно — колесо удлиняется на 10
 * лет, когда прокрутка доезжает до края (onNearEnd WheelPicker). */
export type MonthYearPickerProps = {
  /** 0..11, как у Date. */
  readonly month: number;
  readonly year: number;
  /** Нижняя граница выбора: ничего раньше этого месяца. */
  readonly min?: CalendarMonthRef;
  readonly onConfirm: (month: number, year: number) => void;
  readonly confirmLabel?: string;
  readonly className?: string;
};

const YEARS_EXTENSION = 10;

export function MonthYearPicker({
  month,
  year,
  min,
  onConfirm,
  confirmLabel = 'Выбрать',
  className,
}: MonthYearPickerProps): JSX.Element {
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
  const draftDisplayMonth = min !== undefined ? clampMonthToMin(min, draftYear, draftMonth) : draftMonth;

  const handleYearChange = (value: string): void => {
    const nextYear = Number(value);
    setDraftYear(nextYear);
    // Перескок в минимальный год прижимает месяц к границе — колесо
    // месяцев перестраивается, черновик не остаётся в недоступном.
    if (min !== undefined) {
      setDraftMonth((current) => clampMonthToMin(min, nextYear, current));
    }
  };

  return (
    <div className={cn('flex flex-col gap-6', className)}>
      <div className="flex items-stretch">
        <WheelPicker
          className="min-w-0 flex-1"
          label="Месяц"
          items={monthItems}
          value={String(draftDisplayMonth)}
          onValueChange={(value) => setDraftMonth(Number(value))}
        />
        <WheelPicker
          className="min-w-0 flex-1"
          label="Год"
          items={yearItems}
          value={String(draftYear)}
          onValueChange={handleYearChange}
          onNearEnd={() => setYearsEnd((end) => end + YEARS_EXTENSION)}
        />
      </div>
      <Button
        className="w-full"
        onClick={() => onConfirm(draftDisplayMonth, draftYear)}
      >
        {confirmLabel}
      </Button>
    </div>
  );
}

'use client';

import type { JSX } from 'react';
import { ChevronDown } from '@/shared/assets/icons';
import { ChipButton } from '@/shared/ui/design';

export type OperationsFilterChipsProps = {
  /** Лейбл чипа периода: дефолт — «Сентябрь 2026», выбор — «1 — 30 ноя». */
  readonly periodLabel: string;
  /** Лейбл чипа категорий: «Все категории», имя или счётчик. */
  readonly categoriesLabel: string;
  /** Активный фильтр категорий подсвечивает чип синим, как период. */
  readonly categoriesActive: boolean;
  readonly onOpenPeriod: () => void;
  readonly onOpenCategories: () => void;
};

/**
 * Строка чипов фильтров экранов операций (#474/#477): «Период» всегда
 * выбран (период применён всегда), «Категория» — синий только с активным
 * фильтром. Чипы открывают свои шиты выбора (#477, Figma 1492-41825).
 */
export function OperationsFilterChips({
  periodLabel,
  categoriesLabel,
  categoriesActive,
  onOpenPeriod,
  onOpenCategories,
}: OperationsFilterChipsProps): JSX.Element {
  return (
    <div className="flex gap-1.5 overflow-x-auto px-6">
      <ChipButton selected trailingIcon={<ChevronDown />} onClick={onOpenPeriod}>
        {periodLabel}
      </ChipButton>
      <ChipButton
        selected={categoriesActive}
        trailingIcon={<ChevronDown />}
        onClick={onOpenCategories}
      >
        {categoriesLabel}
      </ChipButton>
    </div>
  );
}

'use client';

import { useState, type JSX } from 'react';
import type {
  OperationsCategoryRow,
  OperationsPeriod,
} from '@/features/payments';
import {
  operationsCategoryChipLabel,
  operationsPeriodDefaultChipLabel,
  operationsPeriodRangeChipLabel,
} from '@/features/payments';
import { OperationsFilterChips } from './operations-filter-chips';
import { OperationsPeriodSheet } from './operations-period-sheet';
import { OperationsCategoriesSheet } from './operations-categories-sheet';

export type OperationsFiltersAreaProps = {
  /** Применённый период (дефолт — текущий месяц). */
  readonly period: OperationsPeriod;
  /** Период выбран явно (чип показывает диапазон, не «Месяц год»). */
  readonly periodExplicit: boolean;
  /** Применённые категории (слаги) и строки шита из разбивки периода. */
  readonly categories: ReadonlyArray<string>;
  readonly categoryRows: ReadonlyArray<OperationsCategoryRow>;
  readonly onApplyPeriod: (period: OperationsPeriod) => void;
  readonly onApplyCategories: (slugs: ReadonlyArray<string>) => void;
};

/**
 * Область фильтров экранов операций (#477): строка чипов «Период» /
 * «Категория» и оба шита выбора с их состоянием — открытым бывает один.
 * Чип периода показывает «Месяц год» для дефолта и диапазон («1 — 30 ноя»)
 * для явного выбора; контекстный чип внутри шита категорий — всегда
 * диапазон (Figma 1506-72116).
 */
export function OperationsFiltersArea({
  period,
  periodExplicit,
  categories,
  categoryRows,
  onApplyPeriod,
  onApplyCategories,
}: OperationsFiltersAreaProps): JSX.Element {
  const [sheet, setSheet] = useState<'period' | 'categories' | null>(null);
  // Счётчик открытых сессий: ключ монтирования шитов — каждое открытие
  // начинает черновик заново из применённых фильтров (закрытый шит при
  // смене ключа уже за порталом, анимация закрытия не страдает).
  const [session, setSession] = useState(0);

  const openSheet = (which: 'period' | 'categories'): void => {
    setSession((count) => count + 1);
    setSheet(which);
  };

  return (
    <>
      <OperationsFilterChips
        periodLabel={
          periodExplicit
            ? operationsPeriodRangeChipLabel(period)
            : operationsPeriodDefaultChipLabel(period)
        }
        categoriesLabel={operationsCategoryChipLabel(categories, categoryRows)}
        categoriesActive={categories.length > 0}
        onOpenPeriod={() => openSheet('period')}
        onOpenCategories={() => openSheet('categories')}
      />

      <OperationsPeriodSheet
        key={`period-${session}`}
        open={sheet === 'period'}
        onOpenChange={(next) => setSheet(next ? 'period' : null)}
        period={period}
        onApply={onApplyPeriod}
      />
      <OperationsCategoriesSheet
        key={`categories-${session}`}
        open={sheet === 'categories'}
        onOpenChange={(next) => setSheet(next ? 'categories' : null)}
        rows={categoryRows}
        selected={categories}
        periodLabel={operationsPeriodRangeChipLabel(period)}
        onApply={onApplyCategories}
      />
    </>
  );
}

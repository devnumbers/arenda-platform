import type { OperationsSummary } from '@/entities/payment';
import { categoryStyle } from './category-style';

/**
 * Сегменты полосы-разбивки на карточках «Расходы»/«Доходы» (Figma
 * 1510-77101, решение владельца 2026-09-01 — отменяет схему #474 «топ-4 +
 * остаток белым»): каждая категория с операциями — отдельная «пилюля»,
 * ширина пропорциональна сумме категории (контейнер раскладывает пилюли
 * по весам flex-grow, зазор 2px держит раздельно даже совпадающие
 * цвета каталога). Направление (income/expense) выбирает срез сводки;
 * порядок — серверный (по сумме убывание). Пустой итог — пустой список:
 * контейнер рисует единственную серую пилюлю (Figma: #D3D7D9).
 */
export type SummaryBarSegment = {
  readonly color: string;
  /** Сумма категории в копейках — вес пилюли при раскладке. */
  readonly weight: number;
};

export function summaryBarSegments(
  summary: OperationsSummary | undefined,
  type: 'income' | 'expense',
): ReadonlyArray<SummaryBarSegment> {
  if (summary === undefined) {
    return [];
  }
  return summary.categories
    .filter((category) => category.type === type && category.totalKopecks > 0)
    .map((category) => ({
      color: categoryStyle('default', category.slug).color,
      weight: category.totalKopecks,
    }));
}

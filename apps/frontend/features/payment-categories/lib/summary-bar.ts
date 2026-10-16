import type { OperationsSummary } from '@/entities/payment';
import { categoryStyle } from './category-style';

/**
 * Сегменты полосы-разбивки на карточках «Расходы»/«Доходы» (Figma
 * 1510-77101). Решение владельца 2026-10-02 (#1075, отменяет правило
 * 2026-09-01 «пилюля на категорию»): одна пилюля на резолвнутый цвет
 * каталога — категории одного цвета (`categoryStyle`) сливаются в одну
 * с суммарным весом, пилюли сортируются по сумме убывание. Бекенд не
 * участвует (ADR 0047: цвета — UI-метаданные фронтового каталога).
 * Направление (income/expense) выбирает срез сводки. Пустой итог —
 * пустой список: контейнер рисует единственную серую пилюлю (Figma:
 * #D3D7D9).
 */
export type SummaryBarSegment = {
  readonly color: string;
  /** Сумма категорий цвета в копейках — вес пилюли при раскладке. */
  readonly weight: number;
};

export function summaryBarSegments(
  summary: OperationsSummary | undefined,
  type: 'income' | 'expense',
): ReadonlyArray<SummaryBarSegment> {
  if (summary === undefined) {
    return [];
  }
  const weightByColor = new Map<string, number>();
  for (const category of summary.categories) {
    if (category.type !== type || category.totalKopecks <= 0) {
      continue;
    }
    const color = categoryStyle('default', category.slug).color;
    weightByColor.set(color, (weightByColor.get(color) ?? 0) + category.totalKopecks);
  }
  return [...weightByColor]
    .map(([color, weight]) => ({ color, weight }))
    .sort((a, b) => b.weight - a.weight);
}

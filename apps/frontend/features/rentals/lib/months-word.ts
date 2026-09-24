import { pluralize } from '@/shared/lib/pluralize';

/**
 * Единственный источник склонения месяцев слайса аренды; до гейта R1 жило
 * копиями в rental-view/past/complete.
 */
export function monthsWord(count: number): string {
  return pluralize(count, 'месяц', 'месяца', 'месяцев');
}

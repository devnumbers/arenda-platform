import { pluralize } from './pluralize';

/**
 * Единственный источник именительного склонения дня («1 день», «2 дня»,
 * «5 дней») — зеркало months-word: слово без числа; сборка «число + слово»
 * — на местах, как у месяцев. Строки с числом поверх канона: сроки аренды
 * (вместо «Меньше месяца», решение владельца 22.10) и formatOverdueDays.
 */
export function daysWord(count: number): string {
  return pluralize(count, 'день', 'дня', 'дней');
}

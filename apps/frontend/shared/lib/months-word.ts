import { pluralize } from './pluralize';

/**
 * Единственный источник именительного склонения месяца («1 месяц»,
 * «2 месяца», «5 месяцев») — общий для строк аренды и бейджа объекта
 * «Осталось N месяцев» (#851; до этого — months-word слайца аренды
 * и локальная тройка pluralize в property-badges).
 */
export function monthsWord(count: number): string {
  return pluralize(count, 'месяц', 'месяца', 'месяцев');
}

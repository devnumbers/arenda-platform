import { pluralize } from '@/shared/lib/pluralize';

/**
 * Человекочитаемый отсчёт по целому числу календарных дней:
 * 'Сегодня' (<= 0), 'Завтра' (1), 'Послезавтра' (2),
 * 'через N день/дня/дней' (3–6),
 * 'через N неделю/недели/недель[, M день/дня/дней]' (7–29),
 * 'через N месяц/месяца/месяцев[, M неделю/недели/недель]' (>= 30;
 * остаток меньше недели отбрасывается).
 */
export function formatCountdownLabel(days: number): string {
  if (days <= 0) {
    return 'Сегодня';
  }
  if (days === 1) {
    return 'Завтра';
  }
  if (days === 2) {
    return 'Послезавтра';
  }

  if (days < 7) {
    return `через ${days} ${pluralize(days, 'день', 'дня', 'дней')}`;
  }

  if (days < 30) {
    const weeks = Math.floor(days / 7);
    const remDays = days % 7;
    const label = `через ${weeks} ${pluralize(weeks, 'неделю', 'недели', 'недель')}`;
    if (remDays === 0) {
      return label;
    }
    return `${label}, ${remDays} ${pluralize(remDays, 'день', 'дня', 'дней')}`;
  }

  const months = Math.floor(days / 30);
  const remDays = days % 30;
  const label = `через ${months} ${pluralize(months, 'месяц', 'месяца', 'месяцев')}`;
  if (remDays >= 7) {
    const weeks = Math.floor(remDays / 7);
    return `${label}, ${weeks} ${pluralize(weeks, 'неделю', 'недели', 'недель')}`;
  }
  return label;
}

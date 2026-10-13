import type { IsoDate } from './calendar';
import { daysUntil, fullMonthsBetween } from './calendar';
import { formatOverdueDays } from './date-format';
import { monthsWord } from './months-word';
import { pluralize } from './pluralize';

/**
 * Единственная строка остатка аренды для всех поверхностей (#1387, слово
 * владельца 13.10): детализация, блок «Аренда» на объекте и бейдж списка
 * (до этого бейдж #584 держал своё правило «Осталось меньше месяца»).
 * Остаток месяцами и больше — как сегодня: полные календарные месяцы между
 * датами, клиентское зеркало серверного счётчика monthsRemaining (ADR 0053
 * §2; прецедент клиентской надстройки — бейдж #584). В последние недели —
 * календарные дни до планового окончания («Остался 1 день / Осталось
 * 5 дней аренды»); в сам день плана — «Заканчивается сегодня» (прецедент
 * «Платёж сегодня»). Прошедший план строки не даёт: сцену держат статусные
 * тексты («Срок аренды подошел к концу», «Аренда завершена»). Бессрочная
 * аренда остатка не имеет — undefined.
 */
export function rentalRemainingLine(
  today: IsoDate,
  plannedEndDate: IsoDate | null,
): string | undefined {
  if (plannedEndDate === null) {
    return undefined;
  }
  const months = fullMonthsBetween(today, plannedEndDate);
  if (months >= 1) {
    const remained = pluralize(months, 'Остался', 'Осталось', 'Осталось');
    return `${remained} ${months} ${monthsWord(months)} аренды`;
  }
  const days = daysUntil(today, plannedEndDate);
  if (days === 0) {
    return 'Заканчивается сегодня';
  }
  if (days > 0) {
    const remained = pluralize(days, 'Остался', 'Осталось', 'Осталось');
    return `${remained} ${formatOverdueDays(days)} аренды`;
  }
  return undefined;
}

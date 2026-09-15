import { operationsMonthOf } from '@/features/payments';
import { MONTH_PREPOSITIONAL } from '@/shared/lib/date-format';
import type { IsoDate } from '@/shared/lib/calendar';

/**
 * Заголовок секции «Операции в <месяц>» на детали объекта (тикет #589,
 * Figma 1185:40820): текущий календарный месяц в предложном падеже.
 * «Сегодня» — клиентское, та же оговорка про TZ, что в operations-month.
 */
export function operationsSectionTitle(today: IsoDate): string {
  return `Операции в ${MONTH_PREPOSITIONAL[operationsMonthOf(today).month]}`;
}

import type { IsoDate } from '@/entities/payment';

/**
 * Возраст просрочки в целых днях: today − date. Просрочку как статус
 * вычисляет сервер (ADR 0048); поля «дней просрочки» в контракте нет, для
 * подписи карточки «5 дней» разница берётся от клиентского «сегодня»
 * (entities/payment/lib/client-today) — расхождение ограничено краевыми часами суток.
 */
export function daysOverdue(operationDate: IsoDate, today: IsoDate): number {
  const diff = Date.parse(`${today}T00:00:00Z`) - Date.parse(`${operationDate}T00:00:00Z`);
  return Math.max(0, Math.round(diff / 86_400_000));
}

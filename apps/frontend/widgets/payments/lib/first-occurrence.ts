import type { IsoDate, Recurrence } from '@/entities/payment';
import { firstOccurrence } from '@/entities/payment';

/**
 * Первое вхождение для превью визарда (#464): правило без пауз, «сегодня» —
 * клиентская проекция (entities/payment/lib/client-today). Общая точка для
 * строки над кнопкой шага и текста экрана успеха.
 */
export function firstOccurrencePreview(
  recurrence: Recurrence,
  since: IsoDate,
  endDate?: IsoDate,
): IsoDate | null {
  return firstOccurrence({ recurrence, since, endDate, pauses: [] });
}

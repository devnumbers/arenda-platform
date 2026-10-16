import { addDays, cmp, type IsoDate } from '@/shared/lib/calendar';
import type { RentalPaymentDay } from '@/entities/rental';
import { firstPaymentDate } from './wizard-model';

/**
 * Нижняя граница календаря продления (#533/#804): контракт PATCH планового
 * окончания (ADR 0053 §3) — строго позже начала и не раньше сегодняшнего
 * дня по TZ собственника. Правило продления «строго позже текущего»
 * (домен «Продление», rentals/CONTEXT.md) жёстче контракта только у срочной
 * с будущим/сегодняшним окончанием; needs_attention (окончание в прошлом)
 * и бессрочная (окончания нет — задаётся первое) стартуют от «сегодня»,
 * а незапущенная бессрочная — от дня после начала: окончание не бывает
 * не позже начала. #1156: третья нижняя граница — первое вхождение дня
 * оплаты (#1150/#1154): продление до него оставило бы аренду без платежей
 * — дыра реальна у открытой/незапущенной аренды, у срочной новый конец
 * и так позже старого, а легаси-пару с дырой граница не удерживает.
 */
export function rentalExtendMinDate(
  currentEnd: IsoDate | null,
  startDate: IsoDate,
  today: IsoDate,
  paymentDay: RentalPaymentDay,
): IsoDate {
  const notBeforeToday =
    currentEnd !== null && cmp(currentEnd, today) >= 0 ? addDays(currentEnd, 1) : today;
  let min = notBeforeToday;
  const dayAfterStart = addDays(startDate, 1);
  if (cmp(dayAfterStart, min) > 0) {
    min = dayAfterStart;
  }
  const first = firstPaymentDate(startDate, paymentDay);
  return cmp(first, min) > 0 ? first : min;
}

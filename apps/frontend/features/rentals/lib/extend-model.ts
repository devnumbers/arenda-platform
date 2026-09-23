import { addDays, cmp, type IsoDate } from '@/shared/lib/calendar';

/**
 * Нижняя граница календаря продления (#533/#804): контракт PATCH планового
 * окончания (ADR 0053 §3) — строго позже начала и не раньше сегодняшнего
 * дня по TZ собственника. Правило продления «строго позже текущего»
 * (домен «Продление», rentals/CONTEXT.md) жёстче контракта только у срочной
 * с будущим/сегодняшним окончанием; needs_attention (окончание в прошлом)
 * и бессрочная (окончания нет — задаётся первое) стартуют от «сегодня»,
 * а незапущенная бессрочная — от дня после начала: окончание не бывает
 * не позже начала.
 */
export function rentalExtendMinDate(
  currentEnd: IsoDate | null,
  startDate: IsoDate,
  today: IsoDate,
): IsoDate {
  const notBeforeToday =
    currentEnd !== null && cmp(currentEnd, today) >= 0 ? addDays(currentEnd, 1) : today;
  const dayAfterStart = addDays(startDate, 1);
  return cmp(notBeforeToday, dayAfterStart) >= 0 ? notBeforeToday : dayAfterStart;
}

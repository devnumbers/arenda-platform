/**
 * Модель страницы «Платежи на оплату» (тикет #1347; макеты 1036:40828 и
 * ревизия владельца 13.10): строки окна напоминаний #1346 приходят из
 * GET /payments/due по реальным дням по возрастанию; страница группирует
 * их по daysLeft (канон due-days сущности), чип «Сначала новые/Сначала
 * старые» разворачивает порядок ГРУПП целиком — внутри дня порядок
 * серверный и не трогается. Состояние чипа — в адресе строки (конвенция
 * ?sort).
 */

import type { PaymentDueItem } from '@/entities/payment';
import { groupDueByDay } from '@/entities/payment';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';

/** Направление групп страницы. */
export type DueSort = 'new' | 'old';

/** Разбор ?sort= строки страницы (конвенция состояния в адресе, канон
 * parseEnumParam): неизвестное, отсутствующее и массивное — дефолт
 * «Сначала новые» (дни по возрастанию, «Сегодня» сверху). */
export function parseDueSortParams(
  sort: string | string[] | undefined,
): DueSort {
  return parseEnumParam(sort, ['new', 'old'], 'new');
}

/** Собственный параметр сортировки в адресе — знание этого модуля;
 * писатель (PaymentsDueGlobalScreen) импортирует отсюда. */
export const DUE_SORT_PARAMS = ['sort'] as const;

/** Сериализация в адрес: дефолт «Сначала новые» параметра не создаёт. */
export function serializeDueSortToParams(sort: DueSort): Record<string, string> {
  return sort === 'old' ? { sort: 'old' } : {};
}

/** Группы страницы в направлении чипа. «Сначала новые» — дни по
 * возрастанию (серверный порядок); «Сначала старые» — те же группы
 * задом наперёд («Через 7 дней» сверху), порядок строк внутри дня
 * сохраняется. */
export function dueGroupsInOrder(
  items: ReadonlyArray<PaymentDueItem>,
  sort: DueSort,
): ReturnType<typeof groupDueByDay> {
  const groups = groupDueByDay(items);
  return sort === 'new' ? groups : [...groups].reverse();
}

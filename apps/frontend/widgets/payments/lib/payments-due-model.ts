/**
 * Модель экранов «На оплату» (тикет #1347; макеты 1036:37525 хаб,
 * 1036:40828 страница, 1041:46762 пустое): строки окна напоминаний #1346
 * приходят из GET /payments/due в фиксированном порядке — бакеты по
 * daysLeft asc, «Сначала новые» внутри (dueDate desc). Страница
 * переворачивает направление внутри бакета чипом «Сначала новые/Сначала
 * старые» (состояние — в адресе строки, конвенция ?sort), хаб метит
 * карточки остатком дней строки.
 */

import type { PaymentDueItem } from '@/entities/payment';
import { groupDueByBucket } from '@/entities/payment';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';
import { pluralize } from '@/shared/lib/pluralize';

/** Направление строк внутри бакета страницы. */
export type DueSort = 'new' | 'old';

/** Разбор ?sort= строки страницы (конвенция состояния в адресе, канон
 * parseEnumParam): неизвестное, отсутствующее и массивное — дефолт
 * «Сначала новые» (серверный порядок #1346). */
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

/** Тай-брейк «Сначала старые» — зеркало серверного (id операции desc):
 * по возрастанию id. */
const byDueDateAsc = (a: PaymentDueItem, b: PaymentDueItem): number =>
  a.dueDate === b.dueDate
    ? a.operationId.localeCompare(b.operationId)
    : a.dueDate.localeCompare(b.dueDate);

/** Строки страницы в направлении чипа. «Сначала новые» — серверный порядок
 * как есть; «Сначала старые» переворачивает сроки только внутри каждого
 * бакета (группировка канона due-buckets держит фиксированный порядок
 * самих бакетов) — «Сегодня» остаётся первой группой страницы. */
export function dueListInOrder(
  items: ReadonlyArray<PaymentDueItem>,
  sort: DueSort,
): ReadonlyArray<PaymentDueItem> {
  if (sort === 'new') {
    return items;
  }
  return groupDueByBucket(items).flatMap((group) =>
    [...group.items].sort(byDueDateAsc),
  );
}

/** Метка даты на карточке хаба (макет 1036:37525, синяя строка под суммой):
 * остаток дней строки — «Сегодня» / «Завтра» / «Через N дней» (склонение
 * канона pluralize; за границами окна 0..7 не бывает — сервер метки не
 * строит, крайний остаток печатается как есть). */
export function dueDaysLabel(daysLeft: number): string {
  if (daysLeft <= 0) {
    return 'Сегодня';
  }
  if (daysLeft === 1) {
    return 'Завтра';
  }
  return `Через ${daysLeft} ${pluralize(daysLeft, 'день', 'дня', 'дней')}`;
}

import { OPERATIONS_FEED_SORT, type GlobalOperationScope } from '@/shared/api/query-keys';
import type { PaymentType } from '@/entities/payment';
import type { OperationsPeriod } from '@/features/payments';

/**
 * Модель страницы направления глобальной ленты (#548, Figma 1858-104152):
 * «Расходы»/«Доходы» — лента и сводка глобальной зоны (#541/#540), суженные
 * контрактом `type` (снапшот направления в каждой операции). Поверхности
 * операций читаются по фактической дате оплаты (решение #933/#994):
 * скоупы просят sort=paid_date (#992).
 */

/** Скоуп списка направления: фильтры ленты из адреса — объекты (#542),
 * период (null — весь период #671: дат в скоупе нет) и категории — плюс
 * тип направления. Пустые выборы проходят как есть — хук сам опускает
 * пустые параметры (#541). */
export function globalDirectionListScope(
  period: OperationsPeriod | null,
  propertyIds: ReadonlyArray<string>,
  categories: ReadonlyArray<string>,
  type: PaymentType,
): GlobalOperationScope {
  return {
    order: 'desc',
    sort: OPERATIONS_FEED_SORT,
    propertyIds,
    ...(period !== null ? { dateFrom: period.from, dateTo: period.to } : {}),
    categories,
    type,
  };
}

/** Скоуп сводки направления: тот же запрос без сужения категориями по
 * умолчанию; применённый категорийный фильтр (непустой выбор) сужает и
 * карточку — она зеркалит отфильтрованный список (решение владельца
 * 01.10, прежнее «сводка категорийный фильтр не принимает» #540 отменено).
 * Тип направления остаётся карточечным сегментом фронта — тоталы отдают
 * оба направления. */
export function globalDirectionSummaryScope(
  period: OperationsPeriod | null,
  propertyIds: ReadonlyArray<string>,
  categories: ReadonlyArray<string>,
  type: PaymentType,
): GlobalOperationScope {
  return {
    order: 'desc',
    sort: OPERATIONS_FEED_SORT,
    propertyIds,
    ...(period !== null ? { dateFrom: period.from, dateTo: period.to } : {}),
    ...(categories.length > 0 ? { categories } : {}),
    type,
  };
}

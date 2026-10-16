import { OPERATIONS_FEED_SORT, type GlobalOperationScope } from '@/shared/api/query-keys';
import type { PaymentType } from '@/entities/payment';
import type { OperationsPeriod } from '@/features/payments';

/**
 * Модель страницы «Выбрать категорию» глобальной ленты (#544) — копия
 * объектной страницы (#477) по всем видимым объектам: список категорий
 * с суммами периода строится из разбивки глобальной сводки (#540).
 */

/** Скоуп сводки страницы категорий: объекты (#542) и период ленты из
 * адреса (null — весь период #672: дат в скоупе нет); применённый
 * категорийный фильтр в запрос не попадает — иначе
 * разбивка сузилась бы до уже выбранных категорий и выбор было бы не
 * с кого менять. Направление (?type= с направленческой ленты, решение
 * владельца 01.10) сужает разбивку до категорий направления (контракт
 * #540 — type сужает только массив категорий); без него — все категории.
 * Пустой выбор объектов проходит как есть — хук сам
 * опускает пустой параметр (#541). */
export function globalCategoriesSummaryScope(
  period: OperationsPeriod | null,
  propertyIds: ReadonlyArray<string>,
  type?: PaymentType,
): GlobalOperationScope {
  return {
    order: 'desc',
    sort: OPERATIONS_FEED_SORT,
    propertyIds,
    ...(period !== null ? { dateFrom: period.from, dateTo: period.to } : {}),
    ...(type !== undefined ? { type } : {}),
  };
}

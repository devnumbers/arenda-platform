import type { GlobalOperationScope } from '@/shared/api/query-keys';
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
 * с кого менять. Пустой выбор объектов проходит как есть — хук сам
 * опускает пустой параметр (#541). */
export function globalCategoriesSummaryScope(
  period: OperationsPeriod | null,
  propertyIds: ReadonlyArray<string>,
): GlobalOperationScope {
  return {
    order: 'desc',
    propertyIds,
    ...(period !== null ? { dateFrom: period.from, dateTo: period.to } : {}),
  };
}

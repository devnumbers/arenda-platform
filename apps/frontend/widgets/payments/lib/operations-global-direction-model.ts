import type { GlobalOperationScope } from '@/shared/api/query-keys';
import type { PaymentType } from '@/entities/payment';
import type { OperationsPeriod } from '@/features/payments';

/**
 * Модель страницы направления глобальной ленты (#548, Figma 1858-104152):
 * «Расходы»/«Доходы» — лента и сводка глобальной зоны (#541/#540), суженные
 * контрактом `type` (снапшот направления в каждой операции).
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
    propertyIds,
    ...(period !== null ? { dateFrom: period.from, dateTo: period.to } : {}),
    categories,
    type,
  };
}

/** Скоуп сводки направления: тот же запрос без категорийного сужения —
 * карточка показывает направление целиком (сводка категорийный фильтр не
 * принимает — контракт #540). */
export function globalDirectionSummaryScope(
  period: OperationsPeriod | null,
  propertyIds: ReadonlyArray<string>,
  type: PaymentType,
): GlobalOperationScope {
  return {
    order: 'desc',
    propertyIds,
    ...(period !== null ? { dateFrom: period.from, dateTo: period.to } : {}),
    type,
  };
}

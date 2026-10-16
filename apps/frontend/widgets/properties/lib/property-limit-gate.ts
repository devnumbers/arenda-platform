import { ApiError } from '@/shared/api/errors';

/** Гейт лимита объектов для сценариев создания (карта #1459, тикет #1469):
 * закодированный 402 property_limit_reached (#1468) разводится на
 * интерстишал /profile/tariff/property-limit, всё остальное — прежняя
 * генерик-ошибка (канон #1452: закодированное проходит как есть,
 * незакодированное маскируется). */
export function isPropertyLimitReachedError(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    error.status === 402 &&
    error.code === 'property_limit_reached'
  );
}

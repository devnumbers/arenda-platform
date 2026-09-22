import type { ParticipantSortOrder } from '@/entities/participants';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';

/**
 * Состояние чипа «Имя» списка «Ваши участники» (#697) в адресе (конвенция
 * «Состояние страницы в адресе», #785): направление переживает перезагрузку
 * и шаринг ссылки. Сортировка по полю одна (имя) — в адресе только
 * направление.
 */

/** Дефолтное направление «Ваших участников» — «А→Я»: сервер приходит
 * name ASC (#697), дефолт в адресе не живёт. */
export const DEFAULT_PARTICIPANTS_LIST_ORDER: ParticipantSortOrder = 'asc';

/** Разбор ?order= списка «Ваши участники»: неизвестное и отсутствующее
 * значения — дефолт «А→Я». */
export function parseParticipantsListOrderParams(
  order: string | string[] | undefined,
): ParticipantSortOrder {
  return parseEnumParam(order, ['asc', 'desc'], DEFAULT_PARTICIPANTS_LIST_ORDER);
}

/** Собственный параметр направления в адресе — знание этого модуля;
 * писатель (ParticipantsListScreen) импортирует отсюда. */
export const PARTICIPANTS_LIST_ORDER_PARAMS = ['order'] as const;

/** Патч направления для адреса: дефолтные значения параметров не создают
 * (пустой query — голый pathname); пишется через useUrlParams с
 * own: PARTICIPANTS_LIST_ORDER_PARAMS (#785). */
export function serializeParticipantsListOrderToParams(
  order: ParticipantSortOrder,
): Record<string, string> {
  return order === DEFAULT_PARTICIPANTS_LIST_ORDER ? {} : { order };
}

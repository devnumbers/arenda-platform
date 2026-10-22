/**
 * URL порции GET /history (контракт #708, ADR 0061 §7). Курсор
 * двусторонний и взаимоисключающий: before_cursor продолжает ленту в
 * прошлое (скролл вверх), after_cursor запрашивает записи моложе
 * загруженных (prepend свежих, канон InfiniteQueryTail). Скоуп фильтров —
 * те же поля, что у шита #711: период, основные действия, виды, актёры,
 * объекты, поиск.
 */

import type { HistoryFeedScope } from '@/shared/api/query-keys';

/** Порция ленты (канон #452: по 50; потолок сервера 100). */
export const HISTORY_PAGE_SIZE = 50;

/** pageParam бесконечного запроса: направление задаётся тем, какой курсор
 * несёт. undefined — самая новая страница. */
export type HistoryFeedPageParam = {
  readonly before?: string;
  readonly after?: string;
};

export function historyFeedUrl(
  scope: HistoryFeedScope,
  cursor: HistoryFeedPageParam | undefined,
  limit: number = HISTORY_PAGE_SIZE,
): string {
  // Страница «Действия участника» (карта #1582): под-ресурс участника,
  // скоуп (uuid юзера AND его текущие ноги) сервер выводит сам — группы
  // скоупа в запрос не едут даже если затесались.
  const base =
    scope.participantId !== undefined
      ? `/participants/${encodeURIComponent(scope.participantId)}/history`
      : '/history';
  const query = new URLSearchParams();
  query.set('limit', String(limit));
  if (cursor?.before) {
    query.set('before_cursor', cursor.before);
  }
  if (cursor?.after) {
    query.set('after_cursor', cursor.after);
  }
  if (scope.dateFrom) {
    query.set('date_from', scope.dateFrom);
  }
  if (scope.dateTo) {
    query.set('date_to', scope.dateTo);
  }
  if (scope.actions?.length) {
    query.set('actions', scope.actions.join(','));
  }
  if (scope.kinds?.length) {
    query.set('kinds', scope.kinds.join(','));
  }
  // actor_ids странице не указ — актёра выводит резолвер скоупа; property_ids
  // («Объекты» шита, аменда владельца 22.10.2026: «объекты опять же только
  // те, в которые приглашён участник») сужает ноги пересечением на сервере.
  if (scope.participantId === undefined && scope.actorIds?.length) {
    query.set('actor_ids', scope.actorIds.join(','));
  }
  if (scope.propertyIds?.length) {
    query.set('property_ids', scope.propertyIds.join(','));
  }
  if (scope.q) {
    query.set('q', scope.q);
  }
  return `${base}?${query.toString()}`;
}

/** URL опций шита фильтров GET /history/filters (#708). property_ids
 * сужает область (прибитые объектом страницы #840/#841: участники —
 * собственник и соучастники объекта, D6); без него — глобальная область
 * читателя (участники только моих объектов, изоляция D14 #1305). */
export function historyFiltersUrl(propertyIds?: ReadonlyArray<string>): string {
  if (propertyIds === undefined || propertyIds.length === 0) {
    return '/history/filters';
  }
  return `/history/filters?property_ids=${propertyIds.join(',')}`;
}

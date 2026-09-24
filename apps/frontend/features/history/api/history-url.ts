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
): string {
  const query = new URLSearchParams();
  query.set('limit', String(HISTORY_PAGE_SIZE));
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
  if (scope.actorIds?.length) {
    query.set('actor_ids', scope.actorIds.join(','));
  }
  if (scope.propertyIds?.length) {
    query.set('property_ids', scope.propertyIds.join(','));
  }
  if (scope.q) {
    query.set('q', scope.q);
  }
  return `/history?${query.toString()}`;
}

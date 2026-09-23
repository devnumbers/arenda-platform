'use client';

import { keepPreviousData, useInfiniteQuery, useQuery, type UseInfiniteQueryResult, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import type { HistoryEntry, HistoryFilterOptions } from '@/entities/history';
import { mapHistoryFilterOptions, mapHistoryItem } from '@/entities/history';
import { historyKeys, type HistoryFeedScope } from '@/shared/api/query-keys';

import {
  historyFeedUrl,
  type HistoryFeedPageParam,
} from './history-url';

type HistoryPageDto = components['schemas']['HistoryPageResponse'];
type HistoryFiltersDto = components['schemas']['HistoryFiltersResponse'];

/**
 * Порция ленты: строки по возрастанию времени внутри страницы сервер
 * отдаёт по убыванию (контракт #708) — экран разворачивает; nextCursor
 * ведёт в прошлое (в before_cursor следующего запроса), prevCursor — в
 * будущее (в after_cursor, prepend свежих).
 */
export type HistoryFeedPage = {
  readonly items: readonly HistoryEntry[];
  readonly nextCursor: string | null;
  readonly prevCursor: string | null;
};

/**
 * Лента «История действий» (#709): двусторонний keyset по (created_at, id)
 * — мессенджерская модель «новые снизу». Порции уходят в прошлое стороной
 * next: fetchNextPage читает СТАРШЕ (before_cursor), поэтому «следующая»
 * страница в модели react-query рисуется НАД загруженными — prepend старых
 * при прокрутке вверх. Сторона prev (after_cursor) — prepend свежих
 * записей, появившихся после загрузки; сам prepend свежих подключит
 * realtime-карта (#714). select разворачивает ПЛОСКИЙ список страниц в
 * хронологию — страницы приходят [новейшая, …, старейшая], внутри
 * страницы строки по убыванию; реверс целого даёт «старые сверху, новые
 * снизу» независимо от того, с какой стороны rides порция (реверс
 * постранично оставил бы новую страницу на её месте в массиве — баг
 * живой приёмки #709: prepend-страница легла в низ ленты).
 *
 * Смена скоупа (поиск #710: q в ключе) держит прежнюю выдачу, пока едет
 * запрос с новым ключом (keepPreviousData, канон поиска #601/#609) —
 * лента под полем не мигает скелетоном на каждый шаг набора.
 */
export function useHistoryFeed(
  scope: HistoryFeedScope = {},
  options: { readonly enabled?: boolean } = {},
): UseInfiniteQueryResult<HistoryEntry[], ApiError> {
  return useInfiniteQuery({
    queryKey: historyKeys.feed(scope),
    queryFn: ({ pageParam }) => fetchHistoryPage(scope, pageParam),
    initialPageParam: undefined as HistoryFeedPageParam | undefined,
    getNextPageParam: (lastPage): HistoryFeedPageParam | undefined =>
      lastPage.nextCursor ? { before: lastPage.nextCursor } : undefined,
    getPreviousPageParam: (firstPage): HistoryFeedPageParam | undefined =>
      firstPage.prevCursor ? { after: firstPage.prevCursor } : undefined,
    select: (data) => data.pages.flatMap((page) => page.items).reverse(),
    placeholderData: keepPreviousData,
    // Фильтры #711, выбранные «в ноль», означают пустой результат —
    // серверный контракт пустой список не отличает от «все», запрос не
    // делается (historyFeedScope возвращает null).
    enabled: options.enabled ?? true,
  });
}

/** Общее горло порции GET /history (#708): cursor — двусторонний keyset,
 * undefined читает самую новую страницу. */
async function fetchHistoryPage(
  scope: HistoryFeedScope,
  cursor: HistoryFeedPageParam | undefined,
): Promise<HistoryFeedPage> {
  const response = await apiClient<HistoryPageDto>(historyFeedUrl(scope, cursor));
  return {
    items: response.items.map(mapHistoryItem),
    nextCursor: response.next_cursor ?? null,
    prevCursor: response.prev_cursor ?? null,
  };
}

/**
 * Опции шита фильтров (GET /history/filters, #708): ленте нужны фото
 * объектов для шапок групп — item фото не несёт (ADR 0061 §7). Скоуп
 * property_ids экран #709 не передаёт; #711/#712 сузят.
 */
export function useHistoryFilters(
  propertyIds: ReadonlyArray<string> = [],
): UseQueryResult<HistoryFilterOptions, ApiError> {
  return useQuery({
    queryKey: historyKeys.filters([...propertyIds]),
    queryFn: async () => {
      const query = new URLSearchParams();
      if (propertyIds.length > 0) {
        query.set('property_ids', propertyIds.join(','));
      }
      const suffix = query.size > 0 ? `?${query.toString()}` : '';
      const response = await apiClient<HistoryFiltersDto>(`/history/filters${suffix}`);
      return mapHistoryFilterOptions(response);
    },
  });
}

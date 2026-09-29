import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import type { HistoryEntry, HistoryFilterOptions } from '@/entities/history';
import { mapHistoryFilterOptions, mapHistoryItem } from '@/entities/history';
import { historyKeys, type HistoryFeedScope } from '@/shared/api/query-keys';

import {
  historyFeedUrl,
  type HistoryFeedPageParam,
} from './history-url';

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

type HistoryPageDto = components['schemas']['HistoryPageResponse'];
type HistoryFiltersDto = components['schemas']['HistoryFiltersResponse'];

/** Двойной модуль API-слоя history (без 'use client'): чистые fetch-функции
 * и queryOptions-фабрики канона #887 — общий источник ключ+fetch для
 * клиентских хуков и серверного префетча. Хуки — в hooks.ts ('use client'). */

/** Общее горло порции GET /history (#708): cursor — двусторонний keyset,
 * undefined читает самую новую страницу. Через него идут и keyset-обход
 * react-query, и live-догон ленты (#718) — чтение порции в кодовой базе
 * одно. */
export async function fetchHistoryPage(
  scope: HistoryFeedScope,
  cursor: HistoryFeedPageParam | undefined,
  transport: ApiTransport = apiClient,
): Promise<HistoryFeedPage> {
  const response = await transport<HistoryPageDto>(historyFeedUrl(scope, cursor));
  return {
    items: response.items.map(mapHistoryItem),
    nextCursor: response.next_cursor ?? null,
    prevCursor: response.prev_cursor ?? null,
  };
}


/** Конфиг keyset-обхода ленты (канон #887) — возвращаемый тип фабрики
 * historyFeedQueryOptions: экспорты features/ несут явные возвращаемые
 * типы (apps/frontend/AGENTS.md), члены — их выведенная форма. */
export type HistoryFeedQueryConfig = {
  readonly queryKey: ReturnType<typeof historyKeys.feed>;
  readonly queryFn: (context: {
    readonly pageParam?: HistoryFeedPageParam;
  }) => Promise<HistoryFeedPage>;
  readonly initialPageParam: HistoryFeedPageParam | undefined;
  readonly getNextPageParam: (
    lastPage: HistoryFeedPage,
  ) => HistoryFeedPageParam | undefined;
};


/** Опции ленты истории (канон #887): один источник ключ+fetch для хука
 * и серверного префетча; дефолтный срез — пустой скоуп. */
export function historyFeedQueryOptions({
  scope = {},
  transport,
}: {
  readonly scope?: HistoryFeedScope;
  readonly transport?: ApiTransport;
}): HistoryFeedQueryConfig {
  return {
    queryKey: historyKeys.feed(scope),
    queryFn: ({ pageParam }: { pageParam?: HistoryFeedPageParam }) =>
      fetchHistoryPage(scope, pageParam, transport),
    initialPageParam: undefined as HistoryFeedPageParam | undefined,
    getNextPageParam: (lastPage): HistoryFeedPageParam | undefined =>
      lastPage.nextCursor ? { before: lastPage.nextCursor } : undefined,
  };
}


/** Чистый fetch опций шита фильтров — общее горло хука и серверного
 * префетча #887. */
export async function fetchHistoryFilters(
  transport: ApiTransport = apiClient,
): Promise<HistoryFilterOptions> {
  const response = await transport<HistoryFiltersDto>('/history/filters');
  return mapHistoryFilterOptions(response);
}


/** Опции шита фильтров (канон #887): один источник ключ+fetch для хука
 * и серверного префетча. */
export function historyFiltersQueryOptions(
  transport: ApiTransport = apiClient,
): UseQueryOptions<HistoryFilterOptions, ApiError, HistoryFilterOptions, ReturnType<typeof historyKeys.filters>> {
  return queryOptions({
    queryKey: historyKeys.filters(),
    queryFn: () => fetchHistoryFilters(transport),
  });
}


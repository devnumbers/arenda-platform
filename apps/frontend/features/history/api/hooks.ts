'use client';

import { useEffect, useRef } from 'react';
import { keepPreviousData, useInfiniteQuery, useQuery, useQueryClient, type InfiniteData, type UseInfiniteQueryResult, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import type { HistoryEntry, HistoryFilterOptions } from '@/entities/history';
import { mapHistoryFilterOptions, mapHistoryItem } from '@/entities/history';
import { historyKeys, type HistoryFeedScope } from '@/shared/api/query-keys';
import { subscribeRealtimeEntity } from '@/shared/api/realtime-subscriptions';

import {
  historyFeedUrl,
  HISTORY_PAGE_SIZE,
  type HistoryFeedPageParam,
} from './history-url';
import { createLiveFeedFrameHandler } from './live-feed-frames';

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
 *
 * Лента живая (карта #714, тикет #718): хук подписан на кадры history
 * realtime-стрима (ADR 0062) — точный потребитель вместо blanket-инвалидации
 * (реестр realtime-subscriptions; провайдер подавляет инвалидацию семейства,
 * пока подписка жива). Кадр догоняет ленту СНИЗУ — свежие строки вливаются
 * в первую страницу кэша (mergeFreshIntoFirstPage), не перечитывая окно:
 * keyset-страницы иммутабельны, а перечитывание сдвинуло бы границы окна и
 * дёрнуло читающего старые строки. Правила догона — в
 * createLiveFeedFrameHandler; на открытии стрима (переподключение, возврат
 * видимости) ленту перечитывает onOpen-инвалидация провайдера — окно
 * реанкеруется целиком.
 */
export function useHistoryFeed(
  scope: HistoryFeedScope = {},
  options: { readonly enabled?: boolean } = {},
): UseInfiniteQueryResult<HistoryEntry[], ApiError> {
  const queryClient = useQueryClient();
  // Аннотация на константе: она же контекстный тип вывода TError — без неё
  // наблюдатель выводит дефолтный Error вместо ApiError.
  const query: UseInfiniteQueryResult<HistoryEntry[], ApiError> = useInfiniteQuery({
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
  useLiveFeedFrames(scope, query, queryClient);
  return query;
}

/** Структурный срез результата useInfiniteQuery, нужный живой подписке:
 * полные дженерики наблюдателя в сигнатуру не тащим. */
type LiveFeedQuery = {
  readonly isEnabled: boolean;
  readonly isPending: boolean;
  /** SELECT-данные (#709) — плоская хронология; длина = «есть загруженные
   * строки» (к границе можно вливать свежие). */
  readonly data?: HistoryEntry[];
  refetch(): Promise<unknown>;
};

/** Вливает свежие строки в первую страницу кэша (канон prepend'а TanStack):
 * страница растёт без добавления страницы — pageParams[0] остаётся undefined,
 * поэтому последующий refetch ленты перечитывает её целиком и консистентен
 * (prepend отдельной страницей через fetchPreviousPage ломал refetch:
 * неполная prepend-страница не несёт next_cursor — контракт #708 отдаёт его
 * только у полных порций — и обход страниц обрывался, обрезая ленту;
 * найдено живой приёмкой #718). Свежие строки НОВЕЕ всех загруженных —
 * в начало массива items первой страницы (порядок внутри страницы — по
 * убыванию времени); prevCursor первой страницы сдвигается на свежайшую
 * строку (граница следующего догона), nextCursor не трогается — граница
 * в прошлое неизменна. */
export function mergeFreshIntoFirstPage(
  old: InfiniteData<HistoryFeedPage> | undefined,
  freshItems: HistoryEntry[],
  freshPrevCursor: string | null,
): InfiniteData<HistoryFeedPage> | undefined {
  if (old === undefined || old.pages.length === 0) {
    return old;
  }
  const first = old.pages[0];
  if (first === undefined) {
    return old;
  }
  const mergedFirst: HistoryFeedPage = {
    items: [...freshItems, ...first.items],
    nextCursor: first.nextCursor,
    prevCursor: freshPrevCursor ?? first.prevCursor,
  };
  return { ...old, pages: [mergedFirst, ...old.pages.slice(1)] };
}

/** Подписка ленты на кадры history (тикет #718): обработчик живёт столько же,
 * сколько хук, и читает актуальные query/scope через рефы (синхронизация —
 * в эффекте, до подписки: SSE-кадры — макротаски, к их приходу эффекты
 * последнего коммита сброшены); кадр догоняет ленту снизу (логика —
 * createLiveFeedFrameHandler). */
function useLiveFeedFrames(
  scope: HistoryFeedScope,
  query: LiveFeedQuery,
  queryClient: ReturnType<typeof useQueryClient>,
): void {
  const queryRef = useRef(query);
  const scopeRef = useRef(scope);
  useEffect(() => {
    queryRef.current = query;
    scopeRef.current = scope;
  });
  useEffect(() => {
    const handler = createLiveFeedFrameHandler({
      state: () => ({
        enabled: queryRef.current.isEnabled && !queryRef.current.isPending,
        hasPages: (queryRef.current.data?.length ?? 0) > 0,
        scopePropertyIds: scopeRef.current.propertyIds,
      }),
      fetchFresh: async () => {
        const key = historyKeys.feed(scopeRef.current);
        const raw = queryClient.getQueryData<InfiniteData<HistoryFeedPage>>(key);
        const boundary = raw?.pages[0]?.prevCursor;
        if (raw === undefined || raw.pages.length === 0 || boundary === null) {
          return null;
        }
        const response = await apiClient<HistoryPageDto>(
          historyFeedUrl(scopeRef.current, { after: boundary }),
        );
        const freshItems = response.items.map(mapHistoryItem);
        if (freshItems.length === 0) {
          return { hasMore: false };
        }
        queryClient.setQueryData<InfiniteData<HistoryFeedPage>>(
          key,
          (old) => mergeFreshIntoFirstPage(old, freshItems, response.prev_cursor ?? null),
        );
        return { hasMore: freshItems.length >= HISTORY_PAGE_SIZE };
      },
      refetch: () => queryRef.current.refetch(),
    });
    return subscribeRealtimeEntity('history', {
      onFrame: (frame) => {
        // Пока лента — точный потребитель семейства, blanket-путь молчит:
        // немонтированные кэши historyKeys (другие скоупы ленты, опции
        // шита) помечаются устаревшими сами — без перечитывания активных
        // запросов (refetchType 'none' не трогает смонтированную ленту,
        // keyset-окно не сдвигается); повторный маунт в окно staleTime
        // перечитает, навигация устаревших строк не увидит.
        void queryClient.invalidateQueries({
          queryKey: historyKeys.all,
          refetchType: 'none',
        });
        handler(frame);
      },
    });
  }, [queryClient]);
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
 * объектов для шапок групп — item фото не несёт (ADR 0061 §7). Опции
 * всегда всей области чтения: «Действия участника» (#712) прибивает
 * человека скоупом ленты, а не опций — объектные шапки и фильтр «Объекты»
 * работают по всей области.
 */
export function useHistoryFilters(): UseQueryResult<HistoryFilterOptions, ApiError> {
  return useQuery({
    queryKey: historyKeys.filters(),
    queryFn: async () => {
      const response = await apiClient<HistoryFiltersDto>('/history/filters');
      return mapHistoryFilterOptions(response);
    },
  });
}

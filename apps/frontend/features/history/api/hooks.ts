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
 * пока подписка жива). Кадр догоняет ленту СНИЗУ — prepend свежих страниц
 * по стороне prev двустороннего keyset — не перечитывая окно: keyset-страницы
 * иммутабельны, а перечитывание сдвинуло бы границы окна и дёрнуло читающего
 * старые строки. Правила prepend'а — в createLiveFeedFrameHandler; на открытии
 * стрима (переподключение, возврат видимости) ленту перечитывает
 * onOpen-инвалидация провайдера — окно реанкеруется целиком.
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
  readonly hasPreviousPage: boolean;
  fetchPreviousPage(options: { cancelRefetch: boolean }): Promise<unknown>;
  refetch(): Promise<unknown>;
};

/** Подписка ленты на кадры history (тикет #718): обработчик живёт столько же,
 * сколько хук, и читает актуальные query/scope через рефы (синхронизация —
 * в эффекте, до подписки: SSE-кадры — макротаски, к их приходу эффекты
 * последнего коммита сброшены); prepend догоняет свежие страницы снизу
 * (логика — createLiveFeedFrameHandler). Результат fetchPreviousPage несёт
 * SELECT-данные (плоский массив, #709) — курсор свежей границы читается из
 * сырых страниц кэша. */
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
        hasPreviousPage: queryRef.current.hasPreviousPage,
        scopePropertyIds: scopeRef.current.propertyIds,
      }),
      prepend: async () => {
        await queryRef.current.fetchPreviousPage({ cancelRefetch: false });
        const raw = queryClient.getQueryData<InfiniteData<HistoryFeedPage>>(
          historyKeys.feed(scopeRef.current),
        );
        return { firstPrevCursor: raw?.pages[0]?.prevCursor ?? null };
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

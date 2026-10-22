import { describe, expect, it } from 'vitest';

import { HISTORY_PAGE_SIZE, historyFeedUrl, type HistoryFeedPageParam } from './history-url';

describe('historyFeedUrl', () => {
  it('первая страница: только лимит, курсоров нет', () => {
    expect(historyFeedUrl({}, undefined)).toBe(`/history?limit=${HISTORY_PAGE_SIZE}`);
  });

  it('продолжение в прошлое — before_cursor (скролл вверх)', () => {
    const cursor: HistoryFeedPageParam = { before: 'blob-old' };
    expect(historyFeedUrl({}, cursor)).toBe(`/history?limit=${HISTORY_PAGE_SIZE}&before_cursor=blob-old`);
  });

  it('prepend свежих — after_cursor (канон InfiniteQueryTail)', () => {
    const cursor: HistoryFeedPageParam = { after: 'blob-new' };
    expect(historyFeedUrl({}, cursor)).toBe(`/history?limit=${HISTORY_PAGE_SIZE}&after_cursor=blob-new`);
  });

  it('скоуп фильтров сериализуется в query (контракт #708)', () => {
    const url = historyFeedUrl(
      {
        dateFrom: '2026-09-01',
        dateTo: '2026-09-22',
        actions: ['added', 'deleted'],
        kinds: ['payment', 'operation'],
        actorIds: ['11111111-1111-4111-8111-111111111111'],
        propertyIds: ['33333333-3333-4333-8333-333333333333'],
        q: 'аренда',
      },
      undefined,
    );
    expect(url).toBe(
      `/history?limit=${HISTORY_PAGE_SIZE}` +
        '&date_from=2026-09-01&date_to=2026-09-22' +
        '&actions=added%2Cdeleted&kinds=payment%2Coperation' +
        '&actor_ids=11111111-1111-4111-8111-111111111111' +
        '&property_ids=33333333-3333-4333-8333-333333333333&q=%D0%B0%D1%80%D0%B5%D0%BD%D0%B4%D0%B0',
    );
  });

  it('пустые списки скоупа не попадают в query', () => {
    expect(historyFeedUrl({ actions: [], kinds: [] }, undefined)).toBe(`/history?limit=${HISTORY_PAGE_SIZE}`);
  });

  it('лимит порции переопределяется: проверка существования записей несёт limit=1', () => {
    expect(
      historyFeedUrl({ propertyIds: ['33333333-3333-4333-8333-333333333333'] }, undefined, 1),
    ).toBe('/history?limit=1&property_ids=33333333-3333-4333-8333-333333333333');
  });

  it('страница участника: под-ресурс /participants/{id}/history, скоуп выводит сервер (карта #1582)', () => {
    expect(
      historyFeedUrl({ participantId: '12111111-1111-4111-8111-111111111121' }, undefined),
    ).toBe(`/participants/12111111-1111-4111-8111-111111111121/history?limit=${HISTORY_PAGE_SIZE}`);
  });

  it('страница участника: курсоры и поиск едут, скоуп-группы игнорируются (их выводит сервер)', () => {
    const url = historyFeedUrl(
      {
        // Параметр маршрута Next декодирует — билдер кодирует сам (канон
        // ROUTES.participant).
        participantId: 'pending@example.com',
        dateFrom: '2026-10-01',
        actorIds: ['11111111-1111-4111-8111-111111111111'],
        propertyIds: ['33333333-3333-4333-8333-333333333333'],
        q: 'гараж',
      },
      { before: 'blob-old' },
    );
    expect(url).toBe(
      '/participants/pending%40example.com/history' +
        `?limit=${HISTORY_PAGE_SIZE}&before_cursor=blob-old` +
        '&date_from=2026-10-01&q=%D0%B3%D0%B0%D1%80%D0%B0%D0%B6',
    );
  });
});

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
});

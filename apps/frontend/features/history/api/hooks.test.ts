import { describe, expect, it } from 'vitest';
import type { HistoryEntry } from '@/entities/history';
import { mergeFreshIntoFirstPage, novelFeedEntryIds } from './hooks';
import type { HistoryFeedPage } from './queries';

function entry(id: string): HistoryEntry {
  return { id } as HistoryEntry;
}

function page(items: string[], prevCursor: string | null, nextCursor: string | null): HistoryFeedPage {
  return { items: items.map(entry), prevCursor, nextCursor };
}

describe('mergeFreshIntoFirstPage — влитие свежих строк в первую страницу (#718)', () => {
  it('свежие строки (новее всех) встают в голову первой страницы', () => {
    const old = {
      pages: [page(['b2', 'b1'], 'cursor-b2', 'cursor-b1'), page(['a2', 'a1'], 'cursor-a2', 'cursor-a1')],
      pageParams: [undefined, { before: 'cursor-b1' }],
    };

    const merged = mergeFreshIntoFirstPage(old, [entry('c1')], 'cursor-c1');

    expect(merged?.pages[0]?.items.map((item) => item.id)).toStrictEqual(['c1', 'b2', 'b1']);
  });

  it('prevCursor первой страницы сдвигается на свежайшую строку, nextCursor не трогается', () => {
    const old = {
      pages: [page(['b2', 'b1'], 'cursor-b2', 'cursor-b1')],
      pageParams: [undefined],
    };

    const merged = mergeFreshIntoFirstPage(old, [entry('c1')], 'cursor-c1');

    expect(merged?.pages[0]?.prevCursor).toBe('cursor-c1');
    expect(merged?.pages[0]?.nextCursor).toBe('cursor-b1');
    // Остальные страницы и параметры не тронуты.
    expect(merged?.pages).toHaveLength(1);
    expect(merged?.pageParams).toStrictEqual(old.pageParams);
  });

  it('догон после завершившегося refetch не задваивает строки', () => {
    // Гонка с onOpen-инвалидацией провайдера: refetch перечитал ленту и
    // перезаписал кэш между чтением boundary и setQueryData — первая
    // страница уже содержит свежую строку; повторный влив без дедупа дал
    // бы два одинаковых id и сломал бы React key строк.
    const old = {
      pages: [page(['c1', 'b2', 'b1'], 'cursor-c1', 'cursor-b1')],
      pageParams: [undefined],
    };

    const merged = mergeFreshIntoFirstPage(old, [entry('c1'), entry('d1')], 'cursor-d1');

    const ids = merged?.pages[0]?.items.map((item) => item.id) ?? [];
    expect(ids.filter((id) => id === 'c1')).toHaveLength(1);
    expect(ids).toStrictEqual(['d1', 'c1', 'b2', 'b1']);
    expect(merged?.pages[0]?.prevCursor).toBe('cursor-d1');
  });

  it('без свежего курсора остаётся прежняя граница первой страницы', () => {
    const old = {
      pages: [page(['b1'], 'cursor-b1', null)],
      pageParams: [undefined],
    };

    const merged = mergeFreshIntoFirstPage(old, [entry('c1')], null);

    expect(merged?.pages[0]?.prevCursor).toBe('cursor-b1');
  });

  it('пустой кэш возвращается как есть', () => {
    expect(mergeFreshIntoFirstPage(undefined, [entry('c1')], 'cursor-c1')).toBeUndefined();
  });
});

describe('novelFeedEntryIds — свежие строки, которых ещё нет в ленте (#880: метка «новое»)', () => {
  it('догон без пересечения — все строки новы', () => {
    const old = { pages: [page(['b2', 'b1'], 'cursor-b2', 'cursor-b1')], pageParams: [undefined] };
    expect(novelFeedEntryIds(old, [entry('c1'), entry('c2')])).toStrictEqual(['c1', 'c2']);
  });

  it('гонка с onOpen-перечитыванием: уже влитые строки не считаются новыми', () => {
    const old = { pages: [page(['c1', 'b2', 'b1'], 'cursor-c1', 'cursor-b1')], pageParams: [undefined] };
    expect(novelFeedEntryIds(old, [entry('c1'), entry('c2')])).toStrictEqual(['c2']);
  });

  it('пустая лента или отсутствие кэша — всё, что привез догон', () => {
    expect(novelFeedEntryIds(undefined, [entry('c1')])).toStrictEqual(['c1']);
    expect(novelFeedEntryIds({ pages: [], pageParams: [] }, [entry('c1')])).toStrictEqual(['c1']);
  });

  it('без свежих строк — пусто', () => {
    const old = { pages: [page(['b1'], null, null)], pageParams: [undefined] };
    expect(novelFeedEntryIds(old, [])).toStrictEqual([]);
  });
});

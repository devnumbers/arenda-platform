import { describe, expect, it } from 'vitest';
import type { HistoryEntry } from '@/entities/history';
import { mergeFreshIntoFirstPage, type HistoryFeedPage } from './hooks';

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

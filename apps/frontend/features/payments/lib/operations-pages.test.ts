import { describe, expect, it } from 'vitest';
import { OPERATIONS_PAGE_SIZE, operationsOffsetNextPageParam } from './operations-pages';

describe('operationsOffsetNextPageParam', () => {
  it('полная порция — offset следующей = число прочитанных страниц × порция', () => {
    const firstPage = Array.from({ length: OPERATIONS_PAGE_SIZE }, (_, i) => ({ id: i }));
    const secondPage = Array.from({ length: OPERATIONS_PAGE_SIZE }, (_, i) => ({ id: i }));

    expect(operationsOffsetNextPageParam(firstPage, [firstPage])).toBe(OPERATIONS_PAGE_SIZE);
    expect(operationsOffsetNextPageParam(secondPage, [firstPage, secondPage])).toBe(
      OPERATIONS_PAGE_SIZE * 2,
    );
  });

  it('неполная порция — лента кончилась', () => {
    const shortPage = Array.from({ length: OPERATIONS_PAGE_SIZE - 1 }, (_, i) => ({ id: i }));

    expect(operationsOffsetNextPageParam(shortPage, [[]])).toBeUndefined();
    expect(operationsOffsetNextPageParam([], [])).toBeUndefined();
  });
});

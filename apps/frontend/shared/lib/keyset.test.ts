import { describe, expect, it } from 'vitest';
import { keysetNextPageParam } from './keyset';

describe('keysetNextPageParam', () => {
  it('возвращает курсор следующей порции', () => {
    expect(keysetNextPageParam({ nextCursor: 'cursor-2' })).toBe('cursor-2');
  });

  it('null — лента исчерпана, продолжения нет', () => {
    expect(keysetNextPageParam({ nextCursor: null })).toBeUndefined();
  });

  it('undefined курсор тоже означает конец ленты', () => {
    expect(keysetNextPageParam({ nextCursor: undefined })).toBeUndefined();
  });
});

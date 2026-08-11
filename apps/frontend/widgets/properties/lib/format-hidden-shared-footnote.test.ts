import { describe, expect, it } from 'vitest';
import { formatHiddenSharedFootnote } from './format-hidden-shared-footnote';

describe('formatHiddenSharedFootnote', () => {
  it('uses singular noun and verb for 1', () => {
    expect(formatHiddenSharedFootnote(1)).toBe('1 общий объект скрыт — превышен лимит объектов.');
  });

  it('uses paucal noun and plural verb for 2-4', () => {
    expect(formatHiddenSharedFootnote(2)).toBe('2 общих объекта скрыто — превышен лимит объектов.');
    expect(formatHiddenSharedFootnote(4)).toBe('4 общих объекта скрыто — превышен лимит объектов.');
  });

  it('uses plural noun and verb for 5+', () => {
    expect(formatHiddenSharedFootnote(5)).toBe('5 общих объектов скрыто — превышен лимит объектов.');
  });

  it('uses singular forms for 21 (mod-10 edge)', () => {
    expect(formatHiddenSharedFootnote(21)).toBe('21 общий объект скрыт — превышен лимит объектов.');
  });
});

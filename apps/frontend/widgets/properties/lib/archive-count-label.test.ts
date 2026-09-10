import { describe, expect, it } from 'vitest';
import { archiveCountLabel } from './archive-count-label';

/** Подзаголовок-счётчик экрана «Архивные объекты» (тикет #587, макет
 * 1603:92102 — «4 объекта»): число + pluralize из shared/lib. */
describe('archiveCountLabel', () => {
  it('1 — «1 объект»', () => {
    expect(archiveCountLabel(1)).toBe('1 объект');
  });

  it('2/4 — «объекта»', () => {
    expect(archiveCountLabel(2)).toBe('2 объекта');
    expect(archiveCountLabel(4)).toBe('4 объекта');
  });

  it('5–20 — «объектов»', () => {
    expect(archiveCountLabel(5)).toBe('5 объектов');
    expect(archiveCountLabel(11)).toBe('11 объектов');
    expect(archiveCountLabel(20)).toBe('20 объектов');
  });

  it('21/101 — снова «объект»', () => {
    expect(archiveCountLabel(21)).toBe('21 объект');
    expect(archiveCountLabel(101)).toBe('101 объект');
  });

  it('0 — «0 объектов»', () => {
    expect(archiveCountLabel(0)).toBe('0 объектов');
  });
});

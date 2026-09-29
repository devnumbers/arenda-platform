import { describe, expect, it } from 'vitest';
import { resolveFreshSuspendedIds } from './fresh-suspended';

/** Семантика снапшота подвесших (#880): снапшот id прошлого рендера —
 * null = доставка ещё не случилась (холодный вход, первая доставка без
 * анимации); свежая подвеска — дельта снапшотов. Ключевой случай — доставка
 * ещё не пришла (data === undefined: холодный вход /properties без
 * префетча, уход экрана в refetch): снапшот ни сеется пустым, ни тратится,
 * иначе первая настоящая доставка помечала бы свежими все подвесшие разом. */

describe('resolveFreshSuspendedIds', () => {
  it('data undefined, prev null — снапшот не сеется, fresh пуст', () => {
    const { next, fresh } = resolveFreshSuspendedIds(null, undefined);
    expect(next).toBeNull();
    expect(fresh.size).toBe(0);
  });

  it('data undefined, prev определён — снапшот не тратится (next = prev), fresh пуст', () => {
    const prev = new Set(['a']);
    const { next, fresh } = resolveFreshSuspendedIds(prev, undefined);
    expect(next).toBe(prev);
    expect(fresh.size).toBe(0);
  });

  it('первая доставка при prev null — снапшот сеется, fresh пуст (без анимации)', () => {
    const { next, fresh } = resolveFreshSuspendedIds(null, [
      { propertyId: 'a' },
      { propertyId: 'b' },
    ]);
    expect(next).toEqual(new Set(['a', 'b']));
    expect(fresh.size).toBe(0);
  });

  it('prev определён — fresh = дельта prev→ids, next = текущие id', () => {
    const prev = new Set(['a']);
    const { next, fresh } = resolveFreshSuspendedIds(prev, [
      { propertyId: 'a' },
      { propertyId: 'b' },
    ]);
    expect(fresh).toEqual(new Set(['b']));
    expect(next).toEqual(new Set(['a', 'b']));
  });

  it('перечитывание без новых id — fresh пуст (живой blur-in не дублируется)', () => {
    const prev = new Set(['a', 'b']);
    const { fresh } = resolveFreshSuspendedIds(prev, [
      { propertyId: 'a' },
      { propertyId: 'b' },
    ]);
    expect(fresh.size).toBe(0);
  });
});

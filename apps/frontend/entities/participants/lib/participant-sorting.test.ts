import { describe, expect, it } from 'vitest';

import { compareRuText, filterByRuQuery, sortByRuText, sortOrderPickerGroups } from './participant-sorting';

describe('compareRuText / sortByRuText', () => {
  const rows = ['Яна', 'борис', 'Анна'];

  it('русская коллация без регистра, asc/desc', () => {
    expect(compareRuText('Анна', 'борис')).toBeLessThan(0);
    expect(sortByRuText(rows, (r) => r, 'asc')).toEqual(['Анна', 'борис', 'Яна']);
    expect(sortByRuText(rows, (r) => r, 'desc')).toEqual(['Яна', 'борис', 'Анна']);
  });

  it('вход не мутируется', () => {
    const source = ['Яна', 'борис'];
    sortByRuText(source, (r) => r, 'asc');
    expect(source).toEqual(['Яна', 'борис']);
  });
});

describe('filterByRuQuery', () => {
  const rows = [{ t: 'Анна' }, { t: 'Борис' }];

  it('пустой после trim запрос возвращает копию', () => {
    const out = filterByRuQuery(rows, (r) => r.t, '  ');
    expect(out).toEqual(rows);
    expect(out).not.toBe(rows);
  });

  it('подстрока без регистра', () => {
    expect(filterByRuQuery(rows, (r) => r.t, 'анн')).toEqual([{ t: 'Анна' }]);
    expect(filterByRuQuery(rows, (r) => r.t, 'x')).toEqual([]);
  });
});

describe('sortOrderPickerGroups', () => {
  it('поле единственное, направление с selected по order', () => {
    const groups = sortOrderPickerGroups('Имя', 'asc', () => undefined);
    expect(groups).toHaveLength(2);
    expect(groups[0]?.options[0]?.label).toBe('Имя');
    expect(groups[1]?.options.map((o) => o.selected)).toEqual([true, false]);
  });
});

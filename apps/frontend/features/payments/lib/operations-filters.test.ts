import { describe, expect, it } from 'vitest';

import type { OperationsCategorySummary } from '@/entities/payment';

import {
  operationsCategoryChipLabel,
  operationsCategoryRows,
  operationsFiltersHref,
  operationsFiltersParams,
  operationsPeriodChipLabel,
  operationsPeriodRangeChipLabel,
  readOperationsFilters,
  shiftOperationsPeriod,
} from './operations-filters';

const TODAY = '2026-09-02';

const paramsOf = (record: Record<string, string>) => ({
  get: (name: string) => record[name] ?? null,
});

describe('readOperationsFilters', () => {
  it('пустой URL — дефолт: период null (весь период #676), без категорий', () => {
    expect(readOperationsFilters(paramsOf({}), TODAY)).toEqual({
      period: null,
      categories: [],
    });
  });

  it('читает период и категории', () => {
    const filters = readOperationsFilters(
      paramsOf({ from: '2026-08-10', to: '2026-09-01', category: 'rent,mortgage' }),
      TODAY,
    );
    expect(filters.period).toEqual({ from: '2026-08-10', to: '2026-09-01' });
    expect(filters.categories).toEqual(['rent', 'mortgage']);
  });

  it('целиком будущий период отбрасывается — экраны операций показывают только paid', () => {
    const filters = readOperationsFilters(
      paramsOf({ from: '2026-10-01', to: '2026-10-31' }),
      TODAY,
    );
    expect(filters.period).toBeNull();
  });

  it('конец раньше начала и мусор в датах — период отбрасывается', () => {
    expect(readOperationsFilters(paramsOf({ from: '2026-09-10', to: '2026-09-01' }), TODAY).period).toBeNull();
    expect(readOperationsFilters(paramsOf({ from: 'завтра', to: '2026-09-30' }), TODAY).period).toBeNull();
    expect(readOperationsFilters(paramsOf({ from: '2026-13-01', to: '2026-13-30' }), TODAY).period).toBeNull();
    expect(readOperationsFilters(paramsOf({ from: '2026-02-30', to: '2026-03-30' }), TODAY).period).toBeNull();
  });

  it('период без одной из границ нечитаем', () => {
    expect(readOperationsFilters(paramsOf({ from: '2026-08-01' }), TODAY).period).toBeNull();
    expect(readOperationsFilters(paramsOf({ to: '2026-08-31' }), TODAY).period).toBeNull();
  });

  it('категории: пробелы и пустые слаги отбрасываются, дубли схлопываются', () => {
    expect(readOperationsFilters(paramsOf({ category: ' rent , ,rent,mortgage' }), TODAY).categories).toEqual([
      'rent',
      'mortgage',
    ]);
  });

  it('будущий хвост диапазона обрезается по «сегодня»', () => {
    const filters = readOperationsFilters(
      paramsOf({ from: '2026-08-10', to: '2026-10-31' }),
      TODAY,
    );
    expect(filters.period).toEqual({ from: '2026-08-10', to: '2026-09-02' });
  });
});

describe('operationsFiltersParams', () => {
  it('дефолт — пустой набор параметров', () => {
    expect(operationsFiltersParams({ period: null, categories: [] })).toEqual({});
  });

  it('явный выбор пишет from/to, даже если совпадает с текущим месяцем', () => {
    expect(
      operationsFiltersParams({
        period: { from: '2026-09-01', to: '2026-09-30' },
        categories: [],
      }),
    ).toEqual({ from: '2026-09-01', to: '2026-09-30' });
  });

  it('свой диапазон — from/to, категории — comma-list', () => {
    expect(
      operationsFiltersParams({
        period: { from: '2026-08-10', to: '2026-09-19' },
        categories: ['rent', 'mortgage'],
      }),
    ).toEqual({ from: '2026-08-10', to: '2026-09-19', category: 'rent,mortgage' });
  });
});

describe('shiftOperationsPeriod', () => {
  it('целый месяц сдвигается в соседний целый месяц', () => {
    expect(shiftOperationsPeriod({ from: '2026-09-01', to: '2026-09-30' }, -1)).toEqual({
      from: '2026-08-01',
      to: '2026-08-31',
    });
    expect(shiftOperationsPeriod({ from: '2026-11-01', to: '2026-11-30' }, 1)).toEqual({
      from: '2026-12-01',
      to: '2026-12-31',
    });
    expect(shiftOperationsPeriod({ from: '2026-01-01', to: '2026-01-31' }, -1)).toEqual({
      from: '2025-12-01',
      to: '2025-12-31',
    });
  });

  it('произвольный диапазон сдвигается на свою же длину: 3–14 августа назад — 22 июля — 2 августа (решение владельца #472)', () => {
    expect(shiftOperationsPeriod({ from: '2026-08-03', to: '2026-08-14' }, -1)).toEqual({
      from: '2026-07-22',
      to: '2026-08-02',
    });
    expect(shiftOperationsPeriod({ from: '2026-08-03', to: '2026-08-14' }, 1)).toEqual({
      from: '2026-08-15',
      to: '2026-08-26',
    });
  });

  it('окна стыкуются: назад и вперёд возвращает исходный период, без нахлёста и дыр', () => {
    const period = { from: '2026-08-03', to: '2026-08-14' };
    expect(shiftOperationsPeriod(shiftOperationsPeriod(period, -1), 1)).toEqual(period);
    expect(shiftOperationsPeriod(shiftOperationsPeriod(period, 1), -1)).toEqual(period);
  });

  it('один день листается по одному дню', () => {
    expect(shiftOperationsPeriod({ from: '2026-11-05', to: '2026-11-05' }, -1)).toEqual({
      from: '2026-11-04',
      to: '2026-11-04',
    });
    expect(shiftOperationsPeriod({ from: '2026-11-05', to: '2026-11-05' }, 1)).toEqual({
      from: '2026-11-06',
      to: '2026-11-06',
    });
  });

  it('диапазон через границу года сдвигается своей длиной', () => {
    expect(shiftOperationsPeriod({ from: '2025-12-25', to: '2026-01-05' }, -1)).toEqual({
      from: '2025-12-13',
      to: '2025-12-24',
    });
  });
});

describe('operationsFiltersHref', () => {
  it('явные фильтры — from/to/category в query поверх базы', () => {
    expect(
      operationsFiltersHref(
        '/properties/p/operations/income',
        { period: { from: '2026-08-03', to: '2026-08-14' }, categories: ['internet'] },
      ),
    ).toBe('/properties/p/operations/income?from=2026-08-03&to=2026-08-14&category=internet');
  });

  it('дефолтные фильтры — чистая база без query', () => {
    expect(
      operationsFiltersHref('/properties/p/operations', { period: null, categories: [] }),
    ).toBe('/properties/p/operations');
  });

  it('только категории — без from/to', () => {
    expect(
      operationsFiltersHref('/properties/p/operations', { period: null, categories: ['rent'] }),
    ).toBe('/properties/p/operations?category=rent');
  });
});

describe('operationsPeriodRangeChipLabel', () => {
  it('явный выбор всегда в формате диапазона — «1 — 30 ноя» (Figma 1506-72116)', () => {
    expect(operationsPeriodRangeChipLabel({ from: '2026-11-01', to: '2026-11-30' })).toBe(
      '1 — 30 ноя',
    );
  });

  it('явный выбор через месяцы одного года — «10 окт — 19 ноя»', () => {
    expect(operationsPeriodRangeChipLabel({ from: '2026-10-10', to: '2026-11-19' })).toBe(
      '10 окт — 19 ноя',
    );
  });

  it('один день — «5 ноя»', () => {
    expect(operationsPeriodRangeChipLabel({ from: '2026-11-05', to: '2026-11-05' })).toBe('5 ноя');
  });

  it('разные годы — полные даты через точку (Figma 1510-74149)', () => {
    expect(operationsPeriodRangeChipLabel({ from: '2025-01-01', to: '2026-01-01' })).toBe(
      '01.01.2025 — 01.01.2026',
    );
  });
});

describe('operationsPeriodChipLabel', () => {
  it('без периода (#670) — нейтральный «Период» (весь период)', () => {
    expect(operationsPeriodChipLabel(null)).toBe('Период');
  });

  it('с применённым периодом — лейбл диапазона', () => {
    expect(operationsPeriodChipLabel({ from: '2026-11-01', to: '2026-11-30' })).toBe('1 — 30 ноя');
    expect(operationsPeriodChipLabel({ from: '2026-11-05', to: '2026-11-05' })).toBe('5 ноя');
  });
});

describe('operationsCategoryRows', () => {
  it('собирает направления одного слага в строку с суммой', () => {
    const summary: ReadonlyArray<OperationsCategorySummary> = [
      { slug: 'rent', label: 'Арендная плата', type: 'income', totalKopecks: 256_000_00 },
      { slug: 'mortgage', label: 'Ипотека', type: 'expense', totalKopecks: 150_000_00 },
      { slug: 'rent', label: 'Арендная плата', type: 'expense', totalKopecks: 5_000_00 },
    ];
    expect(operationsCategoryRows(summary)).toEqual([
      { slug: 'rent', label: 'Арендная плата', totalKopecks: 261_000_00 },
      { slug: 'mortgage', label: 'Ипотека', totalKopecks: 150_000_00 },
    ]);
  });
});

describe('operationsCategoryChipLabel', () => {
  const rows = [
    { slug: 'rent', label: 'Арендная плата', totalKopecks: 1 },
    { slug: 'mortgage', label: 'Ипотека', totalKopecks: 2 },
  ];

  it('пустой выбор — «Все категории»', () => {
    expect(operationsCategoryChipLabel([], rows)).toBe('Все категории');
  });

  it('одна категория — её имя', () => {
    expect(operationsCategoryChipLabel(['mortgage'], rows)).toBe('Ипотека');
  });

  it('несколько — счётчик со склонением', () => {
    expect(operationsCategoryChipLabel(['rent', 'mortgage'], rows)).toBe('2 категории');
    expect(operationsCategoryChipLabel(['a'], rows)).toBe('1 категория');
    expect(operationsCategoryChipLabel(['a', 'b', 'c'], rows)).toBe('3 категории');
    expect(operationsCategoryChipLabel(['a', 'b', 'c', 'd', 'e'], rows)).toBe('5 категорий');
    expect(operationsCategoryChipLabel(['a', 'b', 'c', 'd', 'e', 'f'], rows)).toBe('6 категорий');
  });

  it('выбранный слаг вне разбивки периода — счётчик вместо имени', () => {
    expect(operationsCategoryChipLabel(['unknown'], rows)).toBe('1 категория');
  });
});


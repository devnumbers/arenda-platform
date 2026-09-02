import { describe, expect, it } from 'vitest';

import type { OperationsCategorySummary } from '@/entities/payment';

import {
  booleanRunSegments,
  defaultOperationsPeriod,
  operationsCategoryChipLabel,
  operationsCategoryRows,
  operationsFiltersParams,
  operationsPeriodBoundLabel,
  operationsPeriodDefaultChipLabel,
  operationsPeriodRangeChipLabel,
  pickOperationsPeriodDay,
  readOperationsFilters,
  settledOperationsPeriod,
  shiftOperationsPeriod,
} from './operations-filters';

const TODAY = '2026-09-02';

const paramsOf = (record: Record<string, string>) => ({
  get: (name: string) => record[name] ?? null,
});

describe('defaultOperationsPeriod', () => {
  it('текущий календарный месяц «сегодня»', () => {
    expect(defaultOperationsPeriod(TODAY)).toEqual({ from: '2026-09-01', to: '2026-09-30' });
  });
});

describe('readOperationsFilters', () => {
  it('пустой URL — дефолт: текущий месяц без категорий', () => {
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
  it('целый месяц сдвигается в соседний с границами месяца', () => {
    expect(shiftOperationsPeriod({ from: '2026-09-01', to: '2026-09-30' }, -1)).toEqual({
      from: '2026-08-01',
      to: '2026-08-31',
    });
    expect(shiftOperationsPeriod({ from: '2026-11-01', to: '2026-11-30' }, 1)).toEqual({
      from: '2026-12-01',
      to: '2026-12-31',
    });
  });

  it('через границу года в обе стороны', () => {
    expect(shiftOperationsPeriod({ from: '2026-01-01', to: '2026-01-31' }, -1)).toEqual({
      from: '2025-12-01',
      to: '2025-12-31',
    });
    expect(shiftOperationsPeriod({ from: '2025-12-10', to: '2026-01-20' }, 1)).toEqual({
      from: '2026-01-10',
      to: '2026-02-20',
    });
  });

  it('граница-последний-день месяца якорится к последнему дню результата', () => {
    // Полный месяц при листании остаётся полным месяцем (стрелки #475).
    expect(shiftOperationsPeriod({ from: '2026-09-01', to: '2026-09-30' }, -1)).toEqual({
      from: '2026-08-01',
      to: '2026-08-31',
    });
    // 31-е зажимается в короткий месяц, конец-последний-день — на конец марта.
    expect(shiftOperationsPeriod({ from: '2026-01-31', to: '2026-02-28' }, 1)).toEqual({
      from: '2026-02-28',
      to: '2026-03-31',
    });
  });
});

describe('лейблы чипа периода', () => {
  it('дефолт (не явный выбор) — «Месяц год»', () => {
    expect(operationsPeriodDefaultChipLabel({ from: '2026-09-01', to: '2026-09-30' })).toBe(
      'Сентябрь 2026',
    );
  });

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

describe('operationsPeriodBoundLabel', () => {
  it('текущий год — день и склонённый месяц («с 1 ноября»)', () => {
    expect(operationsPeriodBoundLabel('2026-11-01', TODAY)).toBe('1 ноября');
    expect(operationsPeriodBoundLabel('2026-09-19', TODAY)).toBe('19 сентября');
  });

  it('другой год — полная дата через точку («по 01.01.2026»)', () => {
    expect(operationsPeriodBoundLabel('2025-01-01', TODAY)).toBe('01.01.2025');
  });
});

describe('черновик выбора периода', () => {
  it('открывается по применённому периоду — конец не пуст', () => {
    const draft = { start: '2026-09-01', end: '2026-09-30' as const };
    expect(settledOperationsPeriod(draft)).toEqual({ from: '2026-09-01', to: '2026-09-30' });
  });

  it('первый тап задаёт границу без конца; применение сводится к одному дню', () => {
    const draft = pickOperationsPeriodDay({ start: '2026-09-01', end: '2026-09-30' }, '2026-08-15');
    expect(draft).toEqual({ start: '2026-08-15', end: null });
    expect(settledOperationsPeriod(draft)).toEqual({ from: '2026-08-15', to: '2026-08-15' });
  });

  it('второй тап правее — завершает диапазон, следующий тап перезапускает', () => {
    let draft = pickOperationsPeriodDay({ start: '2026-09-01', end: null }, '2026-09-19');
    expect(draft).toEqual({ start: '2026-09-01', end: '2026-09-19' });
    draft = pickOperationsPeriodDay(draft, '2026-09-20');
    expect(draft).toEqual({ start: '2026-09-20', end: null });
  });

  it('второй тап левее — перезапуск с новой границей', () => {
    expect(pickOperationsPeriodDay({ start: '2026-09-10', end: null }, '2026-09-05')).toEqual({
      start: '2026-09-05',
      end: null,
    });
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

describe('booleanRunSegments', () => {
  it('находит непрерывные отрезки true', () => {
    expect(booleanRunSegments([false, true, true, true, false, true, true])).toEqual([
      [1, 3],
      [5, 6],
    ]);
  });

  it('полная неделя и пустая неделя', () => {
    expect(booleanRunSegments([true, true, true, true, true, true, true])).toEqual([[0, 6]]);
    expect(booleanRunSegments([false, false, false, false, false, false, false])).toEqual([]);
  });

  it('одиночный день', () => {
    expect(booleanRunSegments([false, false, true, false, false, false, false])).toEqual([[2, 2]]);
  });
});

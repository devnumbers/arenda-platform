import { describe, expect, it } from 'vitest';

import { operationsFeedGate } from './operations-feed-gate';

const emptySummary = {
  incomeTotalKopecks: 0,
  expenseTotalKopecks: 0,
  categories: [],
};

const summaryWithExpenses = {
  incomeTotalKopecks: 0,
  expenseTotalKopecks: 200000,
  categories: [
    { slug: 'internet', label: 'Интернет', type: 'expense' as const, totalKopecks: 200000 },
  ],
};

const ok = <T,>(data: T) => ({ data, isError: false });
const loading = () => ({ data: undefined, isError: false });
const failed = () => ({ data: undefined, isError: true });

describe('operationsFeedGate', () => {
  it('все три запроса в полёте — скелетон, «не было никогда» не утверждаем', () => {
    const gate = operationsFeedGate(loading(), loading(), loading());
    expect(gate.pending).toBe(true);
    expect(gate.neverHad).toBe(false);
    expect(gate.categoryRows).toEqual([]);
  });

  it('все загружены, сводка с категориями, all-time пуст — neverHad, разбивка из периодной сводки', () => {
    const gate = operationsFeedGate(
      ok([]),
      ok(summaryWithExpenses),
      ok(emptySummary),
    );
    expect(gate.pending).toBe(false);
    expect(gate.neverHad).toBe(true);
    expect(gate.categoryRows).toHaveLength(1);
    expect(gate.categoryRows[0]?.slug).toBe('internet');
  });

  it('всё загружено, операции были — обычная лента', () => {
    const gate = operationsFeedGate(ok([]), ok(summaryWithExpenses), ok(summaryWithExpenses));
    expect(gate.pending).toBe(false);
    expect(gate.neverHad).toBe(false);
    expect(gate.categoryRows).toHaveLength(1);
  });

  it('ошибка списка без данных — не скелетон и не «не было никогда»', () => {
    const gate = operationsFeedGate(failed(), loading(), loading());
    expect(gate.pending).toBe(false);
    expect(gate.neverHad).toBe(false);
  });

  it('ошибка сводки при живом списке — не скелетон, разбивка пуста, neverHad решает all-time', () => {
    const gate = operationsFeedGate(ok([]), failed(), ok(emptySummary));
    expect(gate.pending).toBe(false);
    expect(gate.categoryRows).toEqual([]);
    expect(gate.neverHad).toBe(true);
  });

  it('периодная сводка ещё в полёте при загруженной all-time — ждём скелетоном', () => {
    const gate = operationsFeedGate(ok([]), loading(), ok(summaryWithExpenses));
    expect(gate.pending).toBe(true);
    expect(gate.neverHad).toBe(false);
  });
});

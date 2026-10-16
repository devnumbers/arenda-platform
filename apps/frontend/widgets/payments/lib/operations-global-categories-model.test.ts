import { describe, expect, it } from 'vitest';
import { globalCategoriesSummaryScope } from './operations-global-categories-model';

/** Модель страницы «Выбрать категорию» глобальной ленты (#544): сводка
 * за списком категорий — по объектам (#542) и периоду ленты из адреса.
 * Поверхности операций читаются по фактической дате — скоуп просит
 * sort=paid_date (решение #933/#994). */

describe('globalCategoriesSummaryScope', () => {
  const period = { from: '2026-09-01', to: '2026-09-30' } as const;

  it('несёт фильтры ленты из адреса (объекты + период)', () => {
    expect(globalCategoriesSummaryScope(period, ['p1', 'p2'])).toEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1', 'p2'],
      dateFrom: '2026-09-01',
      dateTo: '2026-09-30',
    });
  });

  it('пустой выбор объектов проходит как есть — хук сам опускает параметр', () => {
    expect(globalCategoriesSummaryScope(period, [])).toEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: [],
      dateFrom: '2026-09-01',
      dateTo: '2026-09-30',
    });
  });

  it('применённый категорийный фильтр в сводку не попадает — иначе список выбора сузился бы до уже выбранных', () => {
    const scope = globalCategoriesSummaryScope(period, ['p1']);
    expect('categories' in scope).toBe(false);
  });

  it('период null (дефолт «весь период» #672) — дат в скоупе нет, суммы за всё время', () => {
    expect(globalCategoriesSummaryScope(null, ['p1'])).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
    });
  });

  it('направление с направленческой ленты сужает разбивку (type в скоупе, контракт #540)', () => {
    expect(globalCategoriesSummaryScope(period, ['p1'], 'expense')).toMatchObject({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
      dateFrom: '2026-09-01',
      dateTo: '2026-09-30',
      type: 'expense',
    });
    expect(globalCategoriesSummaryScope(null, [], 'income')).toMatchObject({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: [],
      type: 'income',
    });
  });

  it('без направления — type в скоупе нет (главная лента, все категории)', () => {
    const scope = globalCategoriesSummaryScope(period, ['p1']);
    expect('type' in scope).toBe(false);
  });
});

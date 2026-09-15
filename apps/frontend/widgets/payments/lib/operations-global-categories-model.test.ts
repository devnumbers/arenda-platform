import { describe, expect, it } from 'vitest';
import { globalCategoriesSummaryScope } from './operations-global-categories-model';

/** Модель страницы «Выбрать категорию» глобальной ленты (#544): сводка
 * за списком категорий — по объектам (#542) и периоду ленты из адреса. */

describe('globalCategoriesSummaryScope', () => {
  const period = { from: '2026-09-01', to: '2026-09-30' } as const;

  it('несёт фильтры ленты из адреса (объекты + период)', () => {
    expect(globalCategoriesSummaryScope(period, ['p1', 'p2'])).toEqual({
      order: 'desc',
      propertyIds: ['p1', 'p2'],
      dateFrom: '2026-09-01',
      dateTo: '2026-09-30',
    });
  });

  it('пустой выбор объектов проходит как есть — хук сам опускает параметр', () => {
    expect(globalCategoriesSummaryScope(period, [])).toEqual({
      order: 'desc',
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
      propertyIds: ['p1'],
    });
  });
});

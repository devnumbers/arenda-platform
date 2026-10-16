import { describe, expect, it } from 'vitest';
import { globalDirectionListScope, globalDirectionSummaryScope } from './operations-global-direction-model';

/** Модель страницы направления глобальной ленты (#548): «Расходы»/«Доходы» —
 * та же серверная область, что у ленты (#541/#540), суженная контрактом
 * `type`. Поверхности операций читаются по фактической дате — скоупы
 * просят sort=paid_date (решение #933/#994). */

describe('globalDirectionListScope', () => {
  const period = { from: '2026-11-01', to: '2026-11-30' } as const;

  it('несёт фильтры ленты из адреса (объекты + период + категории) и тип направления', () => {
    expect(globalDirectionListScope(period, ['p1', 'p2'], ['internet'], 'expense')).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1', 'p2'],
      dateFrom: '2026-11-01',
      dateTo: '2026-11-30',
      categories: ['internet'],
      type: 'expense',
    });
  });

  it('пустой выбор объектов и категорий проходит как есть — хук сам опускает параметры', () => {
    expect(globalDirectionListScope(period, [], [], 'income')).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: [],
      dateFrom: '2026-11-01',
      dateTo: '2026-11-30',
      categories: [],
      type: 'income',
    });
  });

  it('период null (дефолт «весь период» #671) — дат в скоупе нет', () => {
    expect(globalDirectionListScope(null, ['p1'], [], 'income')).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
      categories: [],
      type: 'income',
    });
  });
});

describe('globalDirectionSummaryScope', () => {
  it('категорийный фильтр сужает и карточку — она зеркалит отфильтрованный список (решение владельца 01.10)', () => {
    expect(
      globalDirectionSummaryScope({ from: '2026-11-01', to: '2026-11-30' }, ['p1'], ['tv'], 'expense'),
    ).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
      dateFrom: '2026-11-01',
      dateTo: '2026-11-30',
      categories: ['tv'],
      type: 'expense',
    });
  });

  it('пустой выбор категорий — без сужения, карточка показывает направление целиком', () => {
    expect(globalDirectionSummaryScope({ from: '2026-11-01', to: '2026-11-30' }, ['p1'], [], 'expense')).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
      dateFrom: '2026-11-01',
      dateTo: '2026-11-30',
      type: 'expense',
    });
  });

  it('период null — сводка за весь период, дат в скоупе нет (#671)', () => {
    expect(globalDirectionSummaryScope(null, ['p1'], [], 'expense')).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
      type: 'expense',
    });
  });
});

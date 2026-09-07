import { describe, expect, it } from 'vitest';
import { globalDirectionListScope, globalDirectionSummaryScope } from './operations-global-direction-model';

/** Модель страницы направления глобальной ленты (#548): «Расходы»/«Доходы» —
 * та же серверная область, что у ленты (#541/#540), суженная контрактом
 * `type`. */

describe('globalDirectionListScope', () => {
  const period = { from: '2026-11-01', to: '2026-11-30' } as const;

  it('несёт фильтры ленты из адреса (объекты + период + категории) и тип направления', () => {
    expect(globalDirectionListScope(period, ['p1', 'p2'], ['internet'], 'expense')).toEqual({
      order: 'desc',
      propertyIds: ['p1', 'p2'],
      dateFrom: '2026-11-01',
      dateTo: '2026-11-30',
      categories: ['internet'],
      type: 'expense',
    });
  });

  it('пустой выбор объектов и категорий проходит как есть — хук сам опускает параметры', () => {
    expect(globalDirectionListScope(period, [], [], 'income')).toEqual({
      order: 'desc',
      propertyIds: [],
      dateFrom: '2026-11-01',
      dateTo: '2026-11-30',
      categories: [],
      type: 'income',
    });
  });
});

describe('globalDirectionSummaryScope', () => {
  it('тот же запрос с типом, но без категорийного сужения — карточка показывает направление целиком (контракт #540)', () => {
    expect(globalDirectionSummaryScope({ from: '2026-11-01', to: '2026-11-30' }, ['p1'], 'expense')).toEqual({
      order: 'desc',
      propertyIds: ['p1'],
      dateFrom: '2026-11-01',
      dateTo: '2026-11-30',
      type: 'expense',
    });
  });
});

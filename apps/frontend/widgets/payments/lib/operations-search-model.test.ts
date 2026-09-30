import { describe, expect, it } from 'vitest';
import {
  globalSearchListScope,
  globalSearchSummaryScope,
  searchCategoryChips,
  searchListScope,
  searchSummaryScope,
} from './operations-search-model';

/** Модель экрана поиска операций (#476): чипы совпавших категорий из
 * разбивки сводки и скоупы серверных запросов (только оплаченные — решение
 * карты #472). Поверхности операций читаются по фактической дате — скоупы
 * просят sort=paid_date (решение #933/#994). */

const categories = [
  { slug: 'security', label: 'Охрана', type: 'expense', totalKopecks: 550000 },
  { slug: 'cleaning', label: 'Клининг', type: 'expense', totalKopecks: 1250000 },
] as const;

/** Баг #950: одна категория в двух направлениях — сводка отдаёт две строки
 * (артефакт размерности GROUP BY), чип должен быть один. */
const bothDirections = [
  { slug: 'damage-compensation', label: 'Возмещение ущерба', type: 'income', totalKopecks: 300000 },
  { slug: 'damage-compensation', label: 'Возмещение ущерба', type: 'expense', totalKopecks: 200000 },
] as const;

const security = { slug: 'security', label: 'Охрана', type: 'expense', totalKopecks: 550000 } as const;

describe('searchCategoryChips', () => {
  it('переносит разбивку сводки в чипы без выбора', () => {
    expect(searchCategoryChips(categories, null)).toEqual([
      { slug: 'security', label: 'Охрана', selected: false },
      { slug: 'cleaning', label: 'Клининг', selected: false },
    ]);
  });

  it('помечает выбранный чип', () => {
    expect(searchCategoryChips(categories, 'cleaning')).toEqual([
      { slug: 'security', label: 'Охрана', selected: false },
      { slug: 'cleaning', label: 'Клининг', selected: true },
    ]);
  });

  it('выбранный слаг, которого нет в разбивке, ничего не выбирает', () => {
    expect(searchCategoryChips(categories, 'rent').every((chip) => !chip.selected)).toBe(true);
  });

  it('категория в обоих направлениях — один чип (баг #950, сводка отдаёт строку на слаг+направление)', () => {
    expect(searchCategoryChips([...bothDirections, security], null)).toEqual([
      { slug: 'damage-compensation', label: 'Возмещение ущерба', selected: false },
      { slug: 'security', label: 'Охрана', selected: false },
    ]);
  });

  it('склейка не ломает выбор чипа', () => {
    expect(searchCategoryChips(bothDirections, 'damage-compensation')).toEqual([
      { slug: 'damage-compensation', label: 'Возмещение ущерба', selected: true },
    ]);
  });

  it('пустая разбивка — пустые чипы', () => {
    expect(searchCategoryChips([], 'security')).toEqual([]);
  });
});

describe('searchListScope', () => {
  it('только оплаченные, новые сверху, с поисковым запросом', () => {
    expect(searchListScope('охра', null)).toEqual({
      status: 'paid',
      order: 'desc',
      sort: 'paid_date',
      search: 'охра',
    });
  });

  it('выбранный чип сужает список одной категорией', () => {
    expect(searchListScope('охра', 'security')).toEqual({
      status: 'paid',
      order: 'desc',
      sort: 'paid_date',
      search: 'охра',
      categories: ['security'],
    });
  });
});

describe('searchSummaryScope', () => {
  it('тот же запрос без сужения по чипу', () => {
    expect(searchSummaryScope('охра')).toEqual({
      status: 'paid',
      order: 'desc',
      sort: 'paid_date',
      search: 'охра',
    });
  });
});

describe('globalSearchListScope', () => {
  const period = { from: '2026-09-01', to: '2026-09-30' } as const;

  it('несёт фильтры ленты из адреса (объекты + период) и запрос серверу', () => {
    expect(globalSearchListScope(period, ['p1', 'p2'], 'охра', null)).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1', 'p2'],
      dateFrom: '2026-09-01',
      dateTo: '2026-09-30',
      search: 'охра',
    });
  });

  it('выбранный чип сужает список одной категорией', () => {
    expect(globalSearchListScope(period, [], 'охра', 'security')).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: [],
      dateFrom: '2026-09-01',
      dateTo: '2026-09-30',
      search: 'охра',
      categories: ['security'],
    });
  });

  it('период null (дефолт «весь период» #673) — дат в скоупе нет', () => {
    expect(globalSearchListScope(null, ['p1'], 'охра', null)).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
      search: 'охра',
    });
  });
});

describe('globalSearchSummaryScope', () => {
  it('тот же запрос ленты без сужения по чипу', () => {
    expect(globalSearchSummaryScope({ from: '2026-09-01', to: '2026-09-30' }, ['p1'], 'охра')).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
      dateFrom: '2026-09-01',
      dateTo: '2026-09-30',
      search: 'охра',
    });
  });

  it('период null — сводка за весь период, дат в скоупе нет (#673)', () => {
    expect(globalSearchSummaryScope(null, ['p1'], 'охра')).toStrictEqual({
      order: 'desc',
      sort: 'paid_date',
      propertyIds: ['p1'],
      search: 'охра',
    });
  });
});

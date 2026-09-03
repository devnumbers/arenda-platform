import { describe, expect, it } from 'vitest';
import {
  searchCategoryChips,
  searchListScope,
  searchSummaryScope,
} from './operations-search-model';

/** Модель экрана поиска операций (#476): чипы совпавших категорий из
 * разбивки сводки и скоупы серверных запросов (только оплаченные — решение
 * карты #472). */

const categories = [
  { slug: 'security', label: 'Охрана', type: 'expense', totalKopecks: 550000 },
  { slug: 'cleaning', label: 'Клининг', type: 'expense', totalKopecks: 1250000 },
] as const;

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

  it('пустая разбивка — пустые чипы', () => {
    expect(searchCategoryChips([], 'security')).toEqual([]);
  });
});

describe('searchListScope', () => {
  it('только оплаченные, новые сверху, с поисковым запросом', () => {
    expect(searchListScope('охра', null)).toEqual({
      status: 'paid',
      order: 'desc',
      search: 'охра',
    });
  });

  it('выбранный чип сужает список одной категорией', () => {
    expect(searchListScope('охра', 'security')).toEqual({
      status: 'paid',
      order: 'desc',
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
      search: 'охра',
    });
  });
});

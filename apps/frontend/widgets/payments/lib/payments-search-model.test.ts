import { describe, expect, it } from 'vitest';
import type { GlobalPayment, PaymentSearchCategoryView } from '@/entities/payment';
import { makeGlobalPayment } from './global-payment-fixtures';
import {
  SEARCH_ROWS_COLLAPSED_LIMIT,
  effectiveChipKey,
  filterPaymentsByChip,
  searchCategoryChips,
  searchResultRows,
} from './payments-search-model';

const rentChip: PaymentSearchCategoryView = {
  category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
  type: 'expense',
  count: 2,
};

const furnitureChip: PaymentSearchCategoryView = {
  category: { source: 'default', slug: 'furniture', label: 'Аренда мебели' },
  type: 'expense',
  count: 1,
};

const customChip: PaymentSearchCategoryView = {
  category: { source: 'custom', id: '0198b6a7-user', label: 'Своя категория' },
  type: 'income',
  count: 3,
};

describe('searchCategoryChips — чипы matchedCategories (#581, Figma 860:22842)', () => {
  it('ключ дефолтной категории — по слагу, направление — часть ключа', () => {
    const chips = searchCategoryChips([rentChip], null);
    expect(chips).toStrictEqual([
      { key: 'default:rent:expense', label: 'Арендная плата', selected: false },
    ]);
  });

  it('ключ пользовательской категории — по id', () => {
    const chips = searchCategoryChips([customChip], null);
    expect(chips).toStrictEqual([
      { key: 'custom:0198b6a7-user:income', label: 'Своя категория', selected: false },
    ]);
  });

  it('выбранный чип помечается, порядок сервера (по числу совпадений) сохраняется', () => {
    const chips = searchCategoryChips(
      [furnitureChip, customChip, rentChip],
      'custom:0198b6a7-user:income',
    );
    expect(chips.map((chip) => chip.label)).toStrictEqual([
      'Аренда мебели',
      'Своя категория',
      'Арендная плата',
    ]);
    expect(chips.map((chip) => chip.selected)).toStrictEqual([false, true, false]);
  });
});

describe('effectiveChipKey — выбор живёт, пока чип есть в выдаче', () => {
  const categories = [rentChip, furnitureChip];

  it('возвращает выбор, пока он среди чипов', () => {
    expect(effectiveChipKey(categories, 'default:rent:expense')).toBe('default:rent:expense');
  });

  it('после смены запроса исчезнувший чип молча перестаёт сужать', () => {
    expect(effectiveChipKey(categories, 'custom:0198b6a7-user:income')).toBeNull();
  });

  it('пустой выбор остаётся пустым', () => {
    expect(effectiveChipKey(categories, null)).toBeNull();
  });
});

describe('filterPaymentsByChip — сужение списка выбранным чипом', () => {
  const rent = makeGlobalPayment({
    id: 'rent',
    type: 'expense',
    category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
  });
  const sameSlugOtherType = makeGlobalPayment({
    id: 'rent-income',
    type: 'income',
    category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
  });
  const otherCategory = makeGlobalPayment({ id: 'other' });
  const custom = makeGlobalPayment({
    id: 'custom',
    type: 'income',
    category: { source: 'custom', id: '0198b6a7-user', label: 'Своя категория' },
  });

  const items: ReadonlyArray<GlobalPayment> = [
    rent,
    sameSlugOtherType,
    otherCategory,
    custom,
  ];

  it('оставляет платежи категории и направления чипа', () => {
    expect(filterPaymentsByChip(items, rentChip)).toStrictEqual([rent]);
  });

  it('пользовательская категория сужает по id, не по слагу', () => {
    expect(filterPaymentsByChip(items, customChip)).toStrictEqual([custom]);
  });

  it('чип без совпадений даёт пустую выдачу (снятие фильтра — дело экрана)', () => {
    const all: ReadonlyArray<GlobalPayment> = [rent, otherCategory];
    expect(filterPaymentsByChip(all, furnitureChip)).toStrictEqual([]);
  });
});

describe('searchResultRows — «Показать все»/«Свернуть» (706:12649 → 706:13008)', () => {
  const many: ReadonlyArray<GlobalPayment> = Array.from(
    { length: SEARCH_ROWS_COLLAPSED_LIMIT + 2 },
    (_, index) => makeGlobalPayment({ id: `p${index}` }),
  );

  it('короткая выдача показывается целиком без кнопки', () => {
    const short: ReadonlyArray<GlobalPayment> = many.slice(0, SEARCH_ROWS_COLLAPSED_LIMIT);
    const result = searchResultRows(short, false);
    expect(result.rows).toStrictEqual(short);
    expect(result.hasMore).toBe(false);
  });

  it('свернуто — первые три строки и кнопка «Показать все»', () => {
    const result = searchResultRows(many, false);
    expect(result.rows).toStrictEqual(many.slice(0, SEARCH_ROWS_COLLAPSED_LIMIT));
    expect(result.hasMore).toBe(true);
  });

  it('раскрыто — вся выдача, кнопка «Свернуть»', () => {
    const result = searchResultRows(many, true);
    expect(result.rows).toStrictEqual(many);
    expect(result.hasMore).toBe(true);
  });
});

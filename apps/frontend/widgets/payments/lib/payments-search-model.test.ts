import { describe, expect, it } from 'vitest';
import type { PaymentSearchCategoryView } from '@/entities/payment';
import {
  effectiveChipKey,
  searchCategoryChips,
  searchChipFilter,
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

describe('searchChipFilter — чип едет в серверный запрос (порции по 50)', () => {
  it('дефолтная категория фильтрует по слагу каталога', () => {
    expect(searchChipFilter(rentChip)).toStrictEqual({ category: 'rent', type: 'expense' });
  });

  it('пользовательская категория — по id', () => {
    expect(searchChipFilter(customChip)).toStrictEqual({ category: '0198b6a7-user', type: 'income' });
  });
});

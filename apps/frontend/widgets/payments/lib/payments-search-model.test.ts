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

const rentChipIncome: PaymentSearchCategoryView = {
  category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
  type: 'income',
  count: 1,
};

const parkingChip: PaymentSearchCategoryView = {
  category: { source: 'default', slug: 'parking', label: 'Парковка' },
  type: 'expense',
  count: 2,
};

const parkingChipIncome: PaymentSearchCategoryView = {
  category: { source: 'default', slug: 'parking', label: 'Парковка' },
  type: 'income',
  count: 1,
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
  it('ключ дефолтной категории — по слагу, направление в ключ не входит', () => {
    const chips = searchCategoryChips([rentChip], null);
    expect(chips).toStrictEqual([
      { key: 'default:rent', label: 'Арендная плата', selected: false },
    ]);
  });

  it('ключ пользовательской категории — по id', () => {
    const chips = searchCategoryChips([customChip], null);
    expect(chips).toStrictEqual([
      { key: 'custom:0198b6a7-user', label: 'Своя категория', selected: false },
    ]);
  });

  it('одна категория в двух направлениях склеивается в один чип (дубль подписи, решение владельца)', () => {
    const chips = searchCategoryChips([parkingChip, parkingChipIncome], null);
    expect(chips).toStrictEqual([
      { key: 'default:parking', label: 'Парковка', selected: false },
    ]);
  });

  it('склейка не теряет остальные категории и порядок сервера (по числу совпадений)', () => {
    const chips = searchCategoryChips(
      [parkingChip, parkingChipIncome, furnitureChip, customChip],
      'custom:0198b6a7-user',
    );
    expect(chips.map((chip) => chip.label)).toStrictEqual([
      'Парковка',
      'Аренда мебели',
      'Своя категория',
    ]);
    expect(chips.map((chip) => chip.selected)).toStrictEqual([false, false, true]);
  });

  it('выбор помечается по ключу без направления', () => {
    const chips = searchCategoryChips([rentChip, rentChipIncome], 'default:rent');
    expect(chips.map((chip) => chip.selected)).toStrictEqual([true]);
  });
});

describe('effectiveChipKey — выбор живёт, пока чип есть в выдаче', () => {
  const categories = [rentChip, furnitureChip];

  it('возвращает выбор, пока он среди чипов', () => {
    expect(effectiveChipKey(categories, 'default:rent')).toBe('default:rent');
  });

  it('после смены запроса исчезнувший чип молча перестаёт сужать', () => {
    expect(effectiveChipKey(categories, 'custom:0198b6a7-user')).toBeNull();
  });

  it('пустой выбор остаётся пустым', () => {
    expect(effectiveChipKey(categories, null)).toBeNull();
  });
});

describe('searchChipFilter — чип едет в серверный запрос (порции по 50)', () => {
  it('дефолтная категория фильтрует по слагу каталога, без направления', () => {
    expect(searchChipFilter(parkingChip)).toStrictEqual({ category: 'parking' });
  });

  it('пользовательская категория — по id, без направления', () => {
    expect(searchChipFilter(customChip)).toStrictEqual({ category: '0198b6a7-user' });
  });
});

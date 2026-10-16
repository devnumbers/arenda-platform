import { describe, expect, it } from 'vitest';
import type { PaymentCategoryView } from '@/entities/payment';
import {
  effectiveChipKey,
  searchCategoryChipKey,
  searchCategoryChips,
  searchChipFilter,
} from './payments-search-model';

const rentCategory: PaymentCategoryView = {
  source: 'default',
  slug: 'rent',
  label: 'Арендная плата',
};

const parkingCategory: PaymentCategoryView = {
  source: 'default',
  slug: 'parking',
  label: 'Парковка',
};

const furnitureCategory: PaymentCategoryView = {
  source: 'default',
  slug: 'furniture',
  label: 'Аренда мебели',
};

const customCategory: PaymentCategoryView = {
  source: 'custom',
  id: '0198b6a7-user',
  label: 'Своя категория',
};

describe('searchCategoryChips — чипы matchedCategories (#581, Figma 860:22842)', () => {
  it('ключ дефолтной категории — по слагу', () => {
    const chips = searchCategoryChips([rentCategory], null);
    expect(chips).toStrictEqual([
      { key: 'default:rent', label: 'Арендная плата', selected: false },
    ]);
  });

  it('ключ пользовательской категории — по id', () => {
    const chips = searchCategoryChips([customCategory], null);
    expect(chips).toStrictEqual([
      { key: 'custom:0198b6a7-user', label: 'Своя категория', selected: false },
    ]);
  });

  it('сервер отдаёт одну категорию одним чипом (#602), порядок сервера сохраняется', () => {
    const chips = searchCategoryChips(
      [parkingCategory, furnitureCategory, customCategory],
      'custom:0198b6a7-user',
    );
    expect(chips.map((chip) => chip.label)).toStrictEqual([
      'Парковка',
      'Аренда мебели',
      'Своя категория',
    ]);
    expect(chips.map((chip) => chip.selected)).toStrictEqual([false, false, true]);
  });

  it('выбор помечается по ключу', () => {
    const chips = searchCategoryChips([rentCategory, furnitureCategory], 'default:rent');
    expect(chips.map((chip) => chip.selected)).toStrictEqual([true, false]);
  });

  it('searchCategoryChipKey согласован с ключами чипов', () => {
    expect(searchCategoryChipKey(customCategory)).toBe('custom:0198b6a7-user');
    expect(searchCategoryChipKey(rentCategory)).toBe('default:rent');
  });
});

describe('effectiveChipKey — выбор живёт, пока чип есть в выдаче', () => {
  const categories = [rentCategory, furnitureCategory];

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
  it('дефолтная категория фильтрует по слагу каталога', () => {
    expect(searchChipFilter(parkingCategory)).toStrictEqual({ category: 'parking' });
  });

  it('пользовательская категория — по id', () => {
    expect(searchChipFilter(customCategory)).toStrictEqual({ category: '0198b6a7-user' });
  });
});

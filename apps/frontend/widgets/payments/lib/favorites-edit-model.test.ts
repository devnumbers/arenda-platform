import { describe, expect, it } from 'vitest';
import type { GlobalPayment } from '@/entities/payment';
import {
  hasFavoritesEdits,
  moveFavorite,
  remainingFavoriteIds,
} from './favorites-edit-model';

const payment = (id: string): GlobalPayment => ({
  id,
  propertyId: `property-${id}`,
  propertyName: 'Моя квартира',
  title: `Платёж ${id}`,
  amountKopecks: 100_000,
  type: 'expense',
  category: { source: 'default', slug: 'bold-internet', label: 'Интернет' },
  autoPay: false,
  isFavorite: true,
  favoriteOrder: null,
  today: '2026-09-09',
  nearestDate: '2026-09-10',
  overdueOperationCount: 0,
  overdueDays: null,
  oldestOverdueOperationId: null,
});

describe('moveFavorite', () => {
  it('поднимает элемент: 0 → 2', () => {
    const items = [payment('a'), payment('b'), payment('c')];
    expect(moveFavorite(items, 0, 2).map((item) => item.id)).toEqual(['b', 'c', 'a']);
  });

  it('опускает элемент: 2 → 0', () => {
    const items = [payment('a'), payment('b'), payment('c')];
    expect(moveFavorite(items, 2, 0).map((item) => item.id)).toEqual(['c', 'a', 'b']);
  });

  it('та же позиция — порядок не меняется', () => {
    const items = [payment('a'), payment('b')];
    expect(moveFavorite(items, 1, 1).map((item) => item.id)).toEqual(['a', 'b']);
  });

  it('невалидные индексы возвращают исходный порядок', () => {
    const items = [payment('a'), payment('b')];
    expect(moveFavorite(items, -1, 0)).toBe(items);
    expect(moveFavorite(items, 0, 2)).toBe(items);
  });
});

describe('remainingFavoriteIds', () => {
  it('убирает помеченных и сохраняет порядок черновика', () => {
    const order = [payment('a'), payment('b'), payment('c')];
    expect(remainingFavoriteIds(order, new Set(['b']))).toEqual(['a', 'c']);
  });

  it('без помеченных — весь черновик', () => {
    const order = [payment('b'), payment('a')];
    expect(remainingFavoriteIds(order, new Set())).toEqual(['b', 'a']);
  });
});

describe('hasFavoritesEdits', () => {
  const initial = [payment('a'), payment('b'), payment('c')];

  it('нет изменений — false', () => {
    expect(hasFavoritesEdits(initial, initial, new Set())).toBe(false);
  });

  it('перестановка — true', () => {
    expect(hasFavoritesEdits(moveFavorite(initial, 0, 2), initial, new Set())).toBe(true);
  });

  it('одна пометка на удаление — true', () => {
    expect(hasFavoritesEdits(initial, initial, new Set(['b']))).toBe(true);
  });
});

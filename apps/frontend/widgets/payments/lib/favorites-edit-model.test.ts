import { describe, expect, it } from 'vitest';
import type { GlobalPayment } from '@/entities/payment';
import {
  favoriteDragAnnouncement,
  hasFavoritesEdits,
  moveFavorite,
  selectFavoriteSelection,
  selectedFavoritesTitle,
  toggleFavoriteSelection,
} from './favorites-edit-model';
import { makeGlobalPayment } from './global-payment-fixtures';

const payment = (id: string): GlobalPayment =>
  makeGlobalPayment({
    id,
    propertyId: `property-${id}`,
    title: `Платёж ${id}`,
    amountKopecks: 100_000,
    category: { source: 'default', slug: 'bold-internet', label: 'Интернет' },
    isFavorite: true,
    today: '2026-09-09',
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

describe('hasFavoritesEdits', () => {
  const initial = [payment('a'), payment('b'), payment('c')];

  it('нет изменений — false', () => {
    expect(hasFavoritesEdits(initial, initial)).toBe(false);
  });

  it('перестановка — true', () => {
    expect(hasFavoritesEdits(moveFavorite(initial, 0, 2), initial)).toBe(true);
  });
});

describe('selectedFavoritesTitle', () => {
  it.each([
    [1, 'Выбрано 1 платёж'],
    [2, 'Выбрано 2 платежа'],
    [3, 'Выбрано 3 платежа'],
    [5, 'Выбрано 5 платежей'],
    [11, 'Выбрано 11 платежей'],
    [21, 'Выбрано 21 платёж'],
    [22, 'Выбрано 22 платежа'],
    [25, 'Выбрано 25 платежей'],
  ])('%i — «%s»', (count, expected) => {
    expect(selectedFavoritesTitle(count)).toBe(expected);
  });
});

describe('toggleFavoriteSelection', () => {
  it('не выбран — добавляет', () => {
    expect(toggleFavoriteSelection(new Set(['a']), 'b')).toEqual(new Set(['a', 'b']));
  });

  it('выбран — убирает', () => {
    expect(toggleFavoriteSelection(new Set(['a', 'b']), 'a')).toEqual(new Set(['b']));
  });

  it('не мутирует исходное множество', () => {
    const ids = new Set(['a']);
    toggleFavoriteSelection(ids, 'a');
    expect(ids).toEqual(new Set(['a']));
  });
});

describe('selectFavoriteSelection', () => {
  it('добавляет, не снимая уже выбранных', () => {
    expect(selectFavoriteSelection(new Set(['a']), 'b')).toEqual(new Set(['a', 'b']));
    expect(selectFavoriteSelection(new Set(['a', 'b']), 'a')).toEqual(new Set(['a', 'b']));
  });
});

describe('favoriteDragAnnouncement', () => {
  it('grab: позиция и подсказка клавиш', () => {
    expect(favoriteDragAnnouncement('grab', 'Аренда', 1, 3)).toBe(
      '«Аренда», позиция 1 из 3. Стрелки вверх и вниз — переместить, пробел — отпустить.',
    );
  });

  it('move: новая позиция', () => {
    expect(favoriteDragAnnouncement('move', 'Аренда', 2, 3)).toBe(
      '«Аренда», позиция 2 из 3.',
    );
  });

  it('release: позиция и подсказка сохранения', () => {
    expect(favoriteDragAnnouncement('release', 'Аренда', 3, 3)).toBe(
      '«Аренда», позиция 3 из 3. Изменения порядка применятся кнопкой «Сохранить».',
    );
  });
});

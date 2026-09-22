import { describe, expect, it } from 'vitest';
import type { NotificationCategory } from './types';
import {
  NOTIFICATION_SETTINGS_CATEGORIES,
  type NotificationCategoryPreferences,
  allCategoriesEnabled,
} from './settings-catalog';

describe('NOTIFICATION_SETTINGS_CATEGORIES', () => {
  it('содержит ровно четыре настраиваемые категории (решение #738)', () => {
    expect(NOTIFICATION_SETTINGS_CATEGORIES).toEqual([
      'rental',
      'payments_operations',
      'tasks',
      'shared_access',
    ]);
  });

  it('вместе с сервисными категориями покрывает весь каталог v1 без пересечений', () => {
    const serviceCategories: readonly NotificationCategory[] = ['tariff', 'system'];
    const union = new Set<string>([...NOTIFICATION_SETTINGS_CATEGORIES, ...serviceCategories]);
    const catalog: readonly NotificationCategory[] = [
      'rental',
      'payments_operations',
      'tasks',
      'shared_access',
      'tariff',
      'system',
    ];
    expect(union.size).toBe(catalog.length);
    for (const category of catalog) {
      expect(union.has(category)).toBe(true);
    }
  });
});

describe('allCategoriesEnabled', () => {
  it('даёт дефолт «всё включено» (решение #738)', () => {
    const expected: NotificationCategoryPreferences = {
      rental: true,
      payments_operations: true,
      tasks: true,
      shared_access: true,
    };
    expect(allCategoriesEnabled()).toEqual(expected);
  });
});

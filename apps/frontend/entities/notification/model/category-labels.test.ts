import { describe, expect, it } from 'vitest';
import { notificationCategoryLabel } from './category-labels';

describe('notificationCategoryLabel', () => {
  it('даёт канонические имена всех шести категорий каталога v1', () => {
    expect(notificationCategoryLabel('rental')).toBe('Аренда');
    expect(notificationCategoryLabel('payments_operations')).toBe('Платежи и операции');
    expect(notificationCategoryLabel('tasks')).toBe('Задачи');
    expect(notificationCategoryLabel('shared_access')).toBe('Совместный доступ');
    expect(notificationCategoryLabel('tariff')).toBe('Тариф');
    expect(notificationCategoryLabel('system')).toBe('Системные уведомления');
  });
});

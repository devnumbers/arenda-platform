import { describe, expect, it } from 'vitest';
import { allCategoriesEnabled, NOTIFICATION_CATEGORIES } from '@/entities/notification';
import type { NotificationCategory } from '@/entities/notification';
import type { PushDevicePreferences } from '@/features/push-notifications';
import { toastAllowedByPush } from './toast-gate';

/** Флаги категорий устройства одним вызовом. */
function device(
  categories: Partial<Record<NotificationCategory, boolean>> = {},
): PushDevicePreferences {
  return {
    categories: { ...allCategoriesEnabled(), ...categories },
  };
}

describe('toastAllowedByPush — тост-гейт push-настройки категории (#790, решение #737)', () => {
  it('нет состояния устройства (нет подписки, не загружено) — тост разрешён: дефолт «всё включено»', () => {
    // Мастера-флага нет (спека #1028 §0): выключенное устройство — строка
    // подписки отсутствует, для гейта это то же «состояния нет».
    expect(toastAllowedByPush(undefined, 'rental')).toBe(true);
  });

  it('сервисные категории Тариф и Системные вне настроек — тост всегда разрешён', () => {
    expect(toastAllowedByPush(device(), 'tariff')).toBe(true);
    expect(toastAllowedByPush(device(), 'system')).toBe(true);
    expect(toastAllowedByPush(device({ rental: false }), 'tariff')).toBe(true);
  });

  it('категория выключена в настройках устройства — тост заглушен', () => {
    expect(toastAllowedByPush(device({ rental: false }), 'rental')).toBe(false);
    expect(toastAllowedByPush(device({ tasks: false }), 'tasks')).toBe(false);
  });

  it('категория включена — тост разрешён', () => {
    for (const category of NOTIFICATION_CATEGORIES) {
      expect(toastAllowedByPush(device(), category)).toBe(true);
    }
  });
});

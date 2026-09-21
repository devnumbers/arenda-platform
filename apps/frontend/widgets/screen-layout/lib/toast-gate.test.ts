import { describe, expect, it } from 'vitest';
import { allCategoriesEnabled, NOTIFICATION_CATEGORIES } from '@/entities/notification';
import type { NotificationCategory } from '@/entities/notification';
import type { PushDevicePreferences } from '@/features/push-notifications';
import { toastAllowedByPush } from './toast-gate';

/** Состояние устройства: мастер + флаги категорий одним вызовом. */
function device(
  enabled: boolean,
  categories: Partial<Record<NotificationCategory, boolean>> = {},
): PushDevicePreferences {
  return {
    enabled,
    categories: { ...allCategoriesEnabled(), ...categories },
  };
}

describe('toastAllowedByPush — тост-гейт push-настройки категории (#790, решение #737)', () => {
  it('нет состояния устройства (нет подписки, не загружено) — тост разрешён: дефолт «всё включено»', () => {
    expect(toastAllowedByPush(undefined, 'rental')).toBe(true);
  });

  it('сервисные категории Тариф и Системные вне настроек — тост всегда разрешён', () => {
    // Мастер выключен и флагов нет — сервисные категории от настроек не зависят.
    expect(toastAllowedByPush(device(false), 'tariff')).toBe(true);
    expect(toastAllowedByPush(device(false), 'system')).toBe(true);
  });

  it('мастер-тумблер выключен — настраиваемые категории заглушены', () => {
    expect(toastAllowedByPush(device(false), 'rental')).toBe(false);
    expect(toastAllowedByPush(device(false), 'payments_operations')).toBe(false);
    expect(toastAllowedByPush(device(false), 'tasks')).toBe(false);
    expect(toastAllowedByPush(device(false), 'shared_access')).toBe(false);
  });

  it('категория выключена в настройках устройства — тост заглушен', () => {
    expect(toastAllowedByPush(device(true, { rental: false }), 'rental')).toBe(false);
  });

  it('категория включена и мастер включён — тост разрешён', () => {
    for (const category of NOTIFICATION_CATEGORIES) {
      expect(toastAllowedByPush(device(true), category)).toBe(true);
    }
  });
});

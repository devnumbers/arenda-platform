import {
  NOTIFICATION_SETTINGS_CATEGORIES,
  type NotificationCategory,
  type NotificationSettingsCategory,
} from '@/entities/notification';
import type { PushDevicePreferences } from '@/features/push-notifications';

/** Настраиваемые категории одним Set — сервисные (Тариф, Системные) мимо. */
const SETTINGS_CATEGORY_SET: ReadonlySet<NotificationCategory> = new Set<NotificationCategory>(
  NOTIFICATION_SETTINGS_CATEGORIES,
);

function isSettingsCategory(
  category: NotificationCategory,
): category is NotificationSettingsCategory {
  return SETTINGS_CATEGORY_SET.has(category);
}

/**
 * Тост-гейт push-настройки категории (#790, решение #737): тост — живое
 * отображение событий, разрешённых push-настройкой устройства. Сервисные
 * категории (Тариф, Системные) вне настроек — всегда разрешены; состояния
 * устройства нет (подписки в браузере нет, настройки ещё не загружены) —
 * дефолт «всё включено» (решение #738); у настраиваемой категории — её
 * флаг. Мастера-флага в модели нет (спека #1028 §0): выключенное
 * устройство не имеет строки подписки — для гейта это то же «состояния
 * нет». Ленту и бейдж гейт не касается: лента пишется независимо от
 * настроек (ADR 0058).
 */
export function toastAllowedByPush(
  preferences: PushDevicePreferences | undefined,
  category: NotificationCategory,
): boolean {
  if (!isSettingsCategory(category)) return true;
  if (preferences === undefined) return true;
  return preferences.categories[category];
}

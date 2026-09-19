/**
 * Каталог экрана настроек (#746, решение #738, ADR 0056): четыре
 * настраиваемые категории × два канала (email на аккаунте, push на
 * устройстве). Тариф и Системные всегда включены и в настройках не
 * показываются — вне каталога. Описания групп — с макетов 1789-100250.
 */

/** Четыре настраиваемые категории — в порядке макета. Кортеж — единый
 * источник: union и экранная матрица выводятся из него, дрейфа с каталогом
 * v1 (#737) не бывает — см. дрейф-тест. */
export const NOTIFICATION_SETTINGS_CATEGORIES = [
  'rental',
  'payments_operations',
  'tasks',
  'shared_access',
] as const;

export type NotificationSettingsCategory =
  (typeof NOTIFICATION_SETTINGS_CATEGORIES)[number];

/** Флаги четырёх настраиваемых категорий одного канала — зеркало схемы
 * NotificationCategoryPreferences (контракт #743). */
export type NotificationCategoryPreferences =
  Record<NotificationSettingsCategory, boolean>;

/** Дефолт канала «всё включено» (решение #738): нет строки — всё включено. */
export function allCategoriesEnabled(): NotificationCategoryPreferences {
  return {
    rental: true,
    payments_operations: true,
    tasks: true,
    shared_access: true,
  };
}

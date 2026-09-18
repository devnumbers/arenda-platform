/**
 * Экранные имена категорий уведомлений (каталог v1, решение #737): секция
 * «Категория + дата-время» на странице уведомления (#745) и группы экрана
 * настроек (#746). Тариф и Системные — сервисные, вне настроек, но имя
 * нужно странице. Канонические имена — по каталогу #737; макеты местами
 * пишут «Платёж» вместо полного имени группы — канон важнее.
 */

import type { NotificationCategory } from './types';

const CATEGORY_LABELS: Record<NotificationCategory, string> = {
  rental: 'Аренда',
  payments_operations: 'Платежи и операции',
  tasks: 'Задачи',
  shared_access: 'Совместный доступ',
  tariff: 'Тариф',
  system: 'Системные уведомления',
};

export function notificationCategoryLabel(category: NotificationCategory): string {
  return CATEGORY_LABELS[category];
}

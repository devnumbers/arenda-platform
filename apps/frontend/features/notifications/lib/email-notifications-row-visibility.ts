import type { NotificationSettingsCategory } from '@/entities/notification';

/**
 * Видимость ряда «Уведомления на почту» (карта #1551, канон #1552): глобальный
 * мастер категории решает, существует ли ряд вовсе. Жив только при
 * подтверждённом «включено»; пока GET матрицы в полёте ряд показывают
 * (скелетон свитча — экран не блокируем), при ответе «off» и при нечитаемом
 * ответе (сбой) — схлопывается, и команда сохранения галочку не пишет
 * (дифф без поля). Один предикат для рендера ряда (NotifyEmailRow) и для
 * решения «едет ли notifyEmail в команду» (визард создания, правка) —
 * расхождений быть не может.
 */
export function emailNotificationsRowVisible(
  pending: boolean,
  masterEnabled: boolean | undefined,
): boolean {
  return pending || masterEnabled === true;
}

/** Категория матрицы, гейтящая видимость платёжных рядов. */
export const PAYMENTS_EMAIL_CATEGORY: NotificationSettingsCategory =
  'payments_operations';

/**
 * Внешний вид категории для рендера: тип `CategoryView` ответа сервера
 * разреживает ссылку (дефолтный слаг или пользовательская категория), а
 * иконка и цвет живут в каталоге фронта (#447). Пользовательские категории
 * и «прочее» (слаг, убранный из каталога) рисуются дефолтом пользовательских.
 *
 * На shared-слое с решения #1348: иконку категории рендерит и слой
 * entities (NotificationCategoryIcon в уведомлениях), а cross-slice
 * импорты выше shared запрещены. Фича payment-categories переэкспортирует
 * это через свой публичный API.
 */

import type { PaymentCategoryEntry } from '@/shared/model/payment-category-catalog';
import {
  paymentCategoryBySlug,
  userCategoryDefault,
} from '@/shared/model/payment-category-catalog';

export type CategoryStyle = Readonly<
  Pick<PaymentCategoryEntry, 'icon' | 'color'>
>;

/**
 * Иконка и цвет подложки по ссылке на категорию из контракта:
 * `source: 'custom'` или отсутствующий в каталоге слаг → userCategoryDefault.
 */
export function categoryStyle(source: 'default' | 'custom', slug?: string): CategoryStyle {
  if (source === 'default' && slug !== undefined) {
    const entry = paymentCategoryBySlug(slug);
    if (entry !== undefined) {
      return { icon: entry.icon, color: entry.color };
    }
  }
  return userCategoryDefault;
}

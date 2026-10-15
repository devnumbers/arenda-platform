/**
 * Re-export баррель: реализация с решения #1348 живёт на shared-слое
 * (`shared/model/payment-category-style.ts`) — иконку категории рендерит
 * и слой entities (NotificationCategoryIcon в уведомлениях), а cross-slice
 * импорты выше shared запрещены. Публичный API фичи не меняется.
 */
export { categoryStyle, type CategoryStyle } from '@/shared/model/payment-category-style';

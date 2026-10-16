/**
 * Re-export баррель: реестр с решения #1348 живёт на shared-слое
 * (`shared/ui/payment-category-icon-registry.ts`) — глиф категории рендерит
 * и слой entities (NotificationCategoryIcon в уведомлениях), а cross-slice
 * импорты выше shared запрещены. Публичный API фичи не меняется.
 */
export {
  categoryIconComponents,
  categoryIconComponent,
} from '@/shared/ui/payment-category-icon-registry';

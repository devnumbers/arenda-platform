/**
 * Re-export баррель исторического пути артефакта: сам кодогенерат
 * (tools/payment-categories/generate.mjs) с решением #1348 живёт на
 * shared-слое — `shared/model/payment-category-catalog.ts`. Причина —
 * FSD-границы: иконку категории в уведомлениях рендерит слой entities
 * (NotificationCategoryIcon), а cross-slice импорты выше shared
 * запрещены; shared — единственный дом, читаемый и payments-фичей,
 * и notifications-сущностью. Внутренние импорты фичи и внешние
 * потребители продолжают читать каталог отсюда.
 */
export * from '@/shared/model/payment-category-catalog';

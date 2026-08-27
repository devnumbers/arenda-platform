/**
 * Единый реестр ключей react-query. Живёт в shared/api, потому что ключи —
 * контракт кросс-фичевой инвалидации кэша: мутация одной фичи инвалидирует
 * запросы другой через эти же фабрики, а cross-slice импорты между фичами
 * запрещены (enforced by boundaries/dependencies в eslint.config.mjs).
 */

// features/access
export const accessKeys = {
  all: ['property-access'] as const,
  list: (propertyId: string) =>
    [...accessKeys.all, 'list', propertyId] as const,
};

// features/auth
export const authKeys = {
  all: ['auth'] as const,
  me: ['auth', 'me'] as const,
};

// features/billing
export const billingKeys = {
  tariffs: ['billing', 'tariffs'] as const,
  subscription: ['billing', 'subscription'] as const,
  paymentMethods: ['billing', 'payment-methods'] as const,
  payments: ['billing', 'payments'] as const,
  payment: (id: string) => ['billing', 'payments', id] as const,
};

// features/properties
export const propertyKeys = {
  all: ['properties'] as const,
  list: ['properties', 'list'] as const,
  detail: (id: string) => ['properties', 'detail', id] as const,
  addressSuggestions: (query: string) =>
    [...propertyKeys.all, 'address-suggestions', query] as const,
};

/** Статусный фильтр операций, проходящий в query параметром `status`. */
export type PaymentOperationStatusFilter = 'planned' | 'paid' | 'overdue';

// features/payments
export const paymentKeys = {
  all: ['payments'] as const,
  list: (propertyId: string) =>
    [...paymentKeys.all, 'list', propertyId] as const,
  detail: (propertyId: string, paymentId: string) =>
    [...paymentKeys.all, 'detail', propertyId, paymentId] as const,
};

export const paymentOperationKeys = {
  all: ['payment-operations'] as const,
  /** Просроченные операции объекта (сервер считает overdue по TZ собственника). */
  overdueByProperty: (propertyId: string) =>
    [...paymentOperationKeys.all, 'overdue', propertyId] as const,
  /** Операции платежа по статусу — гасилки «Оплатить» и секция страницы платежа. */
  byPaymentWithStatus: (
    propertyId: string,
    paymentId: string,
    status: PaymentOperationStatusFilter,
  ) => [...paymentOperationKeys.all, 'by-payment', propertyId, paymentId, status] as const,
};

// features/property-contacts
export const propertyContactKeys = {
  all: ['property-contacts'] as const,
  list: (propertyId: string) =>
    [...propertyContactKeys.all, 'list', propertyId] as const,
  detail: (propertyId: string, contactId: string) =>
    [...propertyContactKeys.all, 'detail', propertyId, contactId] as const,
};

// features/subscription
export const subscriptionKeys = {
  subscription: ['subscription'] as const,
};

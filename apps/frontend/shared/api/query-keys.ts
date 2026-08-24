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

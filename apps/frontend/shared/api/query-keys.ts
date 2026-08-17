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

// features/finance
export const financeKeys = {
  reports: () => ['finance', 'report'] as const,
  report: (from?: string, to?: string) =>
    [...financeKeys.reports(), from ?? 'all', to ?? 'all'] as const,
};

// features/free-reminders
export const freeReminderKeys = {
  all: ['free-reminders'] as const,
  detail: (id: string) => [...freeReminderKeys.all, 'detail', id] as const,
  byProperty: (propertyId: string) =>
    [...freeReminderKeys.all, 'by-property', propertyId] as const,
  upcoming: (propertyId: string) =>
    [...freeReminderKeys.all, 'upcoming', propertyId] as const,
};

// features/leases
export const leaseKeys = {
  all: ['leases'] as const,
  detail: (id: string) => [...leaseKeys.all, id] as const,
  byProperty: (propertyId: string) =>
    [...leaseKeys.all, 'property', propertyId] as const,
  reminders: (id: string) => [...leaseKeys.all, id, 'reminders'] as const,
};

// features/operation-categories
export type OperationCategoryType = 'income' | 'expense';

export const categoryKeys = {
  all: ['operation-categories'] as const,
  list: (type: OperationCategoryType) =>
    ['operation-categories', 'list', type] as const,
};

// features/operations
export type OperationKeyFilters = {
  type?: string | string[];
  status?: string | string[];
  category_id?: string | string[];
  property_id?: string;
  from?: string;
  to?: string;
  recurring_operation_id?: string;
  lease_id?: string;
  sort?: string;
  limit?: string;
  offset?: string;
  exclude_archived_properties?: string;
};

export const operationKeys = {
  lists: () => ['operations', 'list'] as const,
  infiniteLists: () => ['operations', 'infinite'] as const,
  byProperty: (
    propertyId: string,
    filters?: Omit<OperationKeyFilters, 'property_id'>,
  ) => ['properties', propertyId, 'operations', filters ?? {}] as const,
  detail: (id: string) => ['operations', 'detail', id] as const,
  operations: (filters: OperationKeyFilters) =>
    [...operationKeys.lists(), filters] as const,
  infiniteOperations: (filters: Omit<OperationKeyFilters, 'offset'>) =>
    [...operationKeys.infiniteLists(), filters] as const,
  summary: (propertyId: string) =>
    ['properties', propertyId, 'operations', 'summary'] as const,
  recurringByProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
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

// features/recurring-operations
export const recurringOperationKeys = {
  lists: () => ['recurring-operations', 'list'] as const,
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
  detail: (id: string) => ['recurring-operations', 'detail', id] as const,
  recurringOperations: () => recurringOperationKeys.lists(),
};

// features/reminders
export const reminderKeys = {
  all: ['reminders'] as const,
  list: (limit: number, offset: number) =>
    [...reminderKeys.all, 'list', limit, offset] as const,
  detail: (id: string) => [...reminderKeys.all, id] as const,
  calendar: (from: string, to: string) =>
    [...reminderKeys.all, 'calendar', from, to] as const,
};

// features/subscription
export const subscriptionKeys = {
  subscription: ['subscription'] as const,
};

// features/tenant-contacts
export const tenantContactKeys = {
  all: ['tenant-contacts'] as const,
  detail: (id: string) => [...tenantContactKeys.all, id] as const,
};

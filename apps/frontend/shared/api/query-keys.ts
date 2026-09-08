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

// features/contacts
export const contactKeys = {
  all: ['contacts'] as const,
  /**
   * Список книги контактов (ADR 0054). propertyId null — плоский список
   * всей видимой книги (глобальная страница контактов), иначе — срез объекта.
   * search — серверный фильтр ('' = без); sort/order — серверная сортировка
   * плоского списка (значения — параметры GET /contacts).
   */
  list: (
    propertyId: string | null,
    search = '',
    sort = 'name',
    order = 'asc',
  ) => [...contactKeys.all, 'list', propertyId, search, sort, order] as const,
  /** Карточка контакта (экран #510). */
  detail: (contactId: string) => [...contactKeys.all, 'detail', contactId] as const,
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

/** Направление сортировки операций по дате вхождения (query `order`). */
export type PaymentOperationOrder = 'asc' | 'desc';

/**
 * Скоуп операций объекта для экранов «Операции объекта» (#474): статус,
 * направление, категории (слаги), границы периода и поисковый запрос
 * (#476). Целиком уходит в ключ react-query и в query-параметры запроса.
 */
export type PaymentOperationScope = {
  readonly status: PaymentOperationStatusFilter;
  readonly order: PaymentOperationOrder;
  readonly type?: 'income' | 'expense';
  readonly categories?: ReadonlyArray<string>;
  /** Границы периода включительно, 'YYYY-MM-DD'. */
  readonly dateFrom?: string;
  readonly dateTo?: string;
  /** Поиск операций (#476): подстрока по названию/категории, числовой
   * запрос — и по сумме; '' и undefined — без поиска. */
  readonly search?: string;
};

// features/payments
export const paymentKeys = {
  all: ['payments'] as const,
  /** Список правил объекта; search — серверный фильтр по названию ('' = без). */
  list: (propertyId: string, search = '') =>
    [...paymentKeys.all, 'list', propertyId, search] as const,
  detail: (propertyId: string, paymentId: string) =>
    [...paymentKeys.all, 'detail', propertyId, paymentId] as const,
};

export const paymentOperationKeys = {
  all: ['payment-operations'] as const,
  /** Одна операция — страница операции. */
  byId: (propertyId: string, operationId: string) =>
    [...paymentOperationKeys.all, 'by-id', propertyId, operationId] as const,
  /** Просроченные операции объекта (сервер считает overdue по TZ собственника). */
  overdueByProperty: (propertyId: string, search = '') =>
    [...paymentOperationKeys.all, 'overdue', propertyId, search] as const,
  /** Операции платежа по статусу — гасилки «Оплатить» и секция страницы платежа. */
  byPaymentWithStatus: (
    propertyId: string,
    paymentId: string,
    status: PaymentOperationStatusFilter,
  ) => [...paymentOperationKeys.all, 'by-payment', propertyId, paymentId, status] as const,
  /** Порции операций платежа (подэкраны #466): статус и направление — часть ключа. */
  byPaymentPaged: (
    propertyId: string,
    paymentId: string,
    status: PaymentOperationStatusFilter,
    order: PaymentOperationOrder,
  ) =>
    [
      ...paymentOperationKeys.all,
      'by-payment-paged',
      propertyId,
      paymentId,
      status,
      order,
    ] as const,
  /** Порции операций объекта: статус, направление и поиск — часть ключа. */
  byPropertyPaged: (
    propertyId: string,
    status: PaymentOperationStatusFilter,
    order: PaymentOperationOrder,
    search = '',
  ) =>
    [
      ...paymentOperationKeys.all,
      'by-property-paged',
      propertyId,
      status,
      order,
      search,
    ] as const,
  /** Порции операций объекта со скоупом экранов операций (#474): весь
   * скоуп — часть ключа, переключение фильтра читает свой кэш. */
  byPropertyScopedPaged: (propertyId: string, scope: PaymentOperationScope) =>
    [
      ...paymentOperationKeys.all,
      'by-property-scoped-paged',
      propertyId,
      scope.status,
      scope.order,
      scope.type ?? '',
      scope.categories?.join(',') ?? '',
      scope.dateFrom ?? '',
      scope.dateTo ?? '',
      scope.search ?? '',
    ] as const,
  /** Сводка периода объекта (#474): статус/тип/период/поиск — часть ключа. */
  summary: (propertyId: string, scope: PaymentOperationScope) =>
    [
      ...paymentOperationKeys.all,
      'summary',
      propertyId,
      scope.status,
      scope.type ?? '',
      scope.dateFrom ?? '',
      scope.dateTo ?? '',
      scope.search ?? '',
    ] as const,
};

// features/tasks
export const taskKeys = {
  all: ['tasks'] as const,
  /** Активные задачи объекта (секции Просроченные/Сегодня/даты/Без даты). */
  active: (propertyId: string) => [...taskKeys.all, 'active', propertyId] as const,
  /** Журнал выполненных (сворачиваемая секция; total — счётчик «Выполненные N»). */
  completed: (propertyId: string) => [...taskKeys.all, 'completed', propertyId] as const,
  /** Правило задачи — экран «Изменить задачу» (#502). */
  rule: (propertyId: string, ruleId: string) =>
    [...taskKeys.all, 'rule', propertyId, ruleId] as const,
  /** Правило без объекта — плоская форма «Изменить задачу» (#537). */
  propertylessRule: (ruleId: string) =>
    [...taskKeys.all, 'rule', 'without-property', ruleId] as const,
  /** Страница безобъектного среза GET /tasks — источник «сегодня» для
   * плоской формы правки (#537); лента #523 читает свой ключ global. */
  propertylessTasks: () => [...taskKeys.all, 'without-property'] as const,
  /** Глобальная лента GET /tasks (#521, экран #523): бакет completed и
   * фильтр (#524/#547; «Общие задачи» + union — решение владельца
   * 2026-09-07) — сегменты ключа, смена фильтра перечитывает ленту. */
  global: (
    completed: boolean,
    propertyIds: ReadonlyArray<string>,
    withoutProperty: boolean,
  ) => [...taskKeys.all, 'global', completed, propertyIds, withoutProperty] as const,
};

/**
 * Скоуп глобальной ленты операций (#541, контракт /operations #540):
 * paid-only — статусного фильтра нет, лента несёт только оплаченные факты.
 * Мультивыбор объектов, категории (слаги), границы периода, поисковый
 * запрос и тип направления (страницы «Расходы»/«Доходы» #548) целиком
 * уходят в ключ react-query и в query-параметры запроса.
 */
export type GlobalOperationScope = {
  readonly order: PaymentOperationOrder;
  readonly propertyIds?: ReadonlyArray<string>;
  readonly categories?: ReadonlyArray<string>;
  /** Границы периода включительно, 'YYYY-MM-DD'. */
  readonly dateFrom?: string;
  readonly dateTo?: string;
  readonly search?: string;
  /** Направление (страницы «Расходы»/«Доходы» #548); без значения — все. */
  readonly type?: 'income' | 'expense';
  /** Операции архивных объектов включены (#549, ?archived=1); без значения
   * — архив исключён (контракт #540). */
  readonly includeArchived?: boolean;
};

// features/payments (глобальная лента операций)
export const globalOperationKeys = {
  all: ['global-operations'] as const,
  /** Порции глобальной ленты (#541): весь скоуп — часть ключа. */
  listPaged: (scope: GlobalOperationScope) =>
    [
      ...globalOperationKeys.all,
      'list-paged',
      scope.order,
      scope.propertyIds?.join(',') ?? '',
      scope.categories?.join(',') ?? '',
      scope.dateFrom ?? '',
      scope.dateTo ?? '',
      scope.search ?? '',
      scope.type ?? '',
      scope.includeArchived ?? false,
    ] as const,
  /** Глобальная сводка (#540): период, объекты и поиск — часть ключа;
   * категории в ключ не входят — сводка категорийный фильтр не принимает
   * (решение владельца #539), а чипы поиска читают сводку поискового
   * скоупа. */
  summary: (scope: GlobalOperationScope) =>
    [
      ...globalOperationKeys.all,
      'summary',
      scope.propertyIds?.join(',') ?? '',
      scope.dateFrom ?? '',
      scope.dateTo ?? '',
      scope.search ?? '',
      scope.type ?? '',
      scope.includeArchived ?? false,
    ] as const,
};

// features/subscription
export const subscriptionKeys = {
  subscription: ['subscription'] as const,
};

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
  /** Активные сессии GET /me/sessions (#728) — экран «Устройства» (#730);
   * ревокации и «все другие» инвалидируют этот ключ. */
  sessions: ['auth', 'sessions'] as const,
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

// features/participants — «Совместный доступ» (карта #692)
export const participantsKeys = {
  all: ['participants'] as const,
  /** Счётчики хаба GET /participants/summary (#693): участники читающего
   * и чужие объекты с активным доступом. */
  summary: ['participants', 'summary'] as const,
  /** Список «Ваши участники» GET /participants (#693, экран #697). */
  list: () => [...participantsKeys.all, 'list'] as const,
  /** Страница участника GET /participants/{id} (#693, экран #698);
   * id — uuid юзера либо pending-почта (encodeURIComponent на потребителе). */
  detail: (participantId: string) =>
    [...participantsKeys.all, 'detail', participantId] as const,
};

// features/properties
export const propertyKeys = {
  all: ['properties'] as const,
  list: ['properties', 'list'] as const,
  detail: (id: string) => ['properties', 'detail', id] as const,
  /** Поиск GET /properties/search (#601): запрос уходит в ключ — смена
   * запроса начинает свежий keyset-обход с пустого курсора. */
  search: (query: string) => [...propertyKeys.all, 'search', query] as const,
  addressSuggestions: (query: string) =>
    [...propertyKeys.all, 'address-suggestions', query] as const,
};

/** Статусный фильтр операций, проходящий в query параметром `status`. */
export type PaymentOperationStatusFilter = 'planned' | 'paid' | 'overdue';

/** Направление сортировки операций по ключу даты списка (query `order`;
 * сам ключ — `sort`). */
export type PaymentOperationOrder = 'asc' | 'desc';

/** Ключ даты операционного списка (query `sort`, #992): плановая дата
 * вхождения или фактическая дата оплаты; период-фильтры и сводки следуют
 * тому же ключу (payments/CONTEXT.md «Операция»). */
export type OperationsSortKey = 'date' | 'paid_date';

/** Сорт операционных поверхностей (решение #933/#994): сортировка,
 * период-фильтры и сводки лент операций читают фактическую дату оплаты.
 * Дефолт контракта 'date' остаётся планировочным поверхностям (история
 * платежа, просрочки, проекция). */
export const OPERATIONS_FEED_SORT: OperationsSortKey = 'paid_date';

/**
 * Скоуп операций объекта для экранов «Операции объекта» (#474): статус,
 * направление, категории (слаги), границы периода и поисковый запрос
 * (#476). Целиком уходит в ключ react-query и в query-параметры запроса.
 */
export type PaymentOperationScope = {
  readonly status: PaymentOperationStatusFilter;
  readonly order: PaymentOperationOrder;
  /** Ключ даты списка и сводки (#992): операционные поверхности просят
   * 'paid_date' (решение #933/#994), платёжная история остаётся на
   * дефолте 'date' (планировочная поверхность). */
  readonly sort?: OperationsSortKey;
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
  /** Порции журнала изменений платежа (ADR 0065, подэкран «История
   * платежа» #1195): keyset (created_at, id) DESC, курсор в queryFn. */
  changesPaged: (propertyId: string, paymentId: string) =>
    [...paymentKeys.all, 'changes-paged', propertyId, paymentId] as const,
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
  /** Префикс всех статусных списков операций платежа — снять кэш удалённого
   * платежа (#535). */
  byPaymentPrefix: (propertyId: string, paymentId: string) =>
    [...paymentOperationKeys.all, 'by-payment', propertyId, paymentId] as const,
  /** Порции операций платежа (подэкраны #466): статус, направление и
   * ключ даты — часть ключа (сорт #992; истории платёжных фактов просят
   * paid_date — решение владельца 30.09, дополнение #994). */
  byPaymentPaged: (
    propertyId: string,
    paymentId: string,
    status: PaymentOperationStatusFilter,
    order: PaymentOperationOrder,
    sort = '',
  ) =>
    [
      ...paymentOperationKeys.all,
      'by-payment-paged',
      propertyId,
      paymentId,
      status,
      order,
      sort,
    ] as const,
  /** Префикс всех порций операций платежа — снять кэш удалённого платежа (#535). */
  byPaymentPagedPrefix: (propertyId: string, paymentId: string) =>
    [...paymentOperationKeys.all, 'by-payment-paged', propertyId, paymentId] as const,
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
      scope.sort ?? '',
      scope.type ?? '',
      scope.categories?.join(',') ?? '',
      scope.dateFrom ?? '',
      scope.dateTo ?? '',
      scope.search ?? '',
    ] as const,
  /** Сводка периода объекта (#474): статус/тип/период/поиск/категории —
   * часть ключа: сводка сужается категорией вместе со списком (решение
   * владельца 01.10), один ключ — один ответ сервера. */
  summary: (propertyId: string, scope: PaymentOperationScope) =>
    [
      ...paymentOperationKeys.all,
      'summary',
      propertyId,
      scope.status,
      scope.sort ?? '',
      scope.type ?? '',
      scope.categories?.join(',') ?? '',
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
  /** Ключ даты ленты и сводки (#992): глобальная лента платёжных фактов
   * читается по paid_date (решение #933/#994). */
  readonly sort?: OperationsSortKey;
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
      scope.sort ?? '',
      scope.propertyIds?.join(',') ?? '',
      scope.categories?.join(',') ?? '',
      scope.dateFrom ?? '',
      scope.dateTo ?? '',
      scope.search ?? '',
      scope.type ?? '',
      scope.includeArchived ?? false,
    ] as const,
  /** Глобальная сводка (#540): период, объекты, поиск и категории — часть
   * ключа: сводка сужается категорией вместе со списком (решение владельца
   * 01.10, прежнее «категории в ключ не входят» #539 отменено), один ключ —
   * один ответ сервера; чипы поиска читают сводку поискового скоупа — у
   * него категорий в скоупе нет, их ключ не меняется. */
  summary: (scope: GlobalOperationScope) =>
    [
      ...globalOperationKeys.all,
      'summary',
      scope.sort ?? '',
      scope.propertyIds?.join(',') ?? '',
      scope.categories?.join(',') ?? '',
      scope.dateFrom ?? '',
      scope.dateTo ?? '',
      scope.search ?? '',
      scope.type ?? '',
      scope.includeArchived ?? false,
    ] as const,
};

// features/payments (глобальный экран «Платежи», карта #573)
export const globalPaymentKeys = {
  all: ['global-payments'] as const,
  /** Фид главного экрана GET /payments (#575): без параметров — весь
   * видимый скоуп целиком; секции и счётчики карточек «Все …» режет фронт. */
  feed: ['global-payments', 'feed'] as const,
  /** Стопки объектов GET /payments/objects (#575): поисковый фильтр —
   * часть ключа ('' = без фильтра). */
  objects: (search = '') => [...globalPaymentKeys.all, 'objects', search] as const,
  /** Поиск GET /payments/search (#575, экран #581): запрос и фильтр чипа
   * (категория; '' = нет; направление из контракта чипов убрано — #602) —
   * части ключа; страница (курсор #597) в ключ не входит — это pageParam
   * бесконечного запроса. Пустой запрос экран не выполняет (стартовое
   * состояние). */
  search: (query = '', category = '') =>
    [...globalPaymentKeys.all, 'search', query, category] as const,
  /** Чипы поиска (matchedCategories) — лёгкий отдельный запрос (limit=1):
   * сервер считает их по всему скоупу, выбранный чип сужает только список
   * (канон сводки операций #543). */
  searchCategories: (query = '') => [...globalPaymentKeys.all, 'search-categories', query] as const,
};

// features/rentals
export const rentalKeys = {
  all: ['rentals'] as const,
  /** Список аренд объекта: незавершённая первая, далее завершённые (#531). */
  list: (propertyId: string) => [...rentalKeys.all, 'list', propertyId] as const,
  /** Итоги аренды за [начало, until] — превью мастера завершения (#534);
   * until в ключе: выбранная дата меняет расчёт. */
  summary: (propertyId: string, rentalId: string, until: string) =>
    [...rentalKeys.all, 'summary', propertyId, rentalId, until] as const,
};

// features/notifications
export const notificationKeys = {
  all: ['notifications'] as const,
  /**
   * Порции ленты GET /notifications (#743): фильтр «Непрочитанные» — часть
   * ключа; страница (курсор) в ключ не входит — это pageParam бесконечного
   * запроса.
   */
  list: (unreadOnly: boolean) => [...notificationKeys.all, 'list', unreadOnly] as const,
  /** Счётчик непрочитанных GET /notifications/unread-count (бейдж). */
  unreadCount: () => [...notificationKeys.all, 'unread-count'] as const,
  /** Страница уведомления GET /notifications/{id} с живыми действиями. */
  detail: (id: string) => [...notificationKeys.all, 'detail', id] as const,
  /** Матрица email-настроек аккаунта GET/PUT /notification-preferences
   * (#743, решение #738). */
  emailPreferences: () => [...notificationKeys.all, 'email-preferences'] as const,
  /** Настройки пушей устройства GET/PUT /push/subscriptions/preferences
   * (#743): endpoint устройства — часть ключа. */
  pushPreferences: (endpoint: string) =>
    [...notificationKeys.all, 'push-preferences', endpoint] as const,
};

/**
 * Скоуп ленты «История действий» (контракт GET /history #708): границы
 * периода ('YYYY-MM-DD'), основные действия, виды, актёры, объекты и
 * поисковый запрос целиком уходят в ключ react-query и в query-параметры.
 * #709 читает ленту без скоупа; поиск (#710), шит фильтров (#711) и
 * прибитые экраны «Действия участника» (#712), «История объекта» (#840)
 * и пары участник-в-объекте (#841) доопределяют поля (пины
 * actor_ids/property_ids).
 */
export type HistoryFeedScope = {
  readonly dateFrom?: string;
  readonly dateTo?: string;
  readonly actions?: ReadonlyArray<string>;
  readonly kinds?: ReadonlyArray<string>;
  readonly actorIds?: ReadonlyArray<string>;
  readonly propertyIds?: ReadonlyArray<string>;
  readonly q?: string;
};

// features/history — «История действий» (карта #704)
export const historyKeys = {
  all: ['history'] as const,
  /** Порции ленты GET /history (#708): скоуп — часть ключа, смена фильтра
   * начинает свежий обход с самой новой страницы; страница (курсор) в ключ
   * не входит — это pageParam бесконечного запроса. */
  feed: (scope: HistoryFeedScope) =>
    [
      ...historyKeys.all,
      'feed',
      scope.dateFrom ?? '',
      scope.dateTo ?? '',
      scope.actions?.join(',') ?? '',
      scope.kinds?.join(',') ?? '',
      scope.actorIds?.join(',') ?? '',
      scope.propertyIds?.join(',') ?? '',
      scope.q ?? '',
    ] as const,
  /** Опции шита фильтров GET /history/filters (#708, экран #711). */
  filters: () => [...historyKeys.all, 'filters'] as const,
};

// features/subscription
export const subscriptionKeys = {
  subscription: ['subscription'] as const,
};

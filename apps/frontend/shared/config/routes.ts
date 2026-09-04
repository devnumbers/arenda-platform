export const ROUTES = {
  login: '/login',
  properties: '/properties',
  property: (id: string) => `/properties/${id}`,
  propertyArchive: '/properties/archive',
  propertyNew: '/properties/new',
  propertyEdit: (id: string) => `/properties/${id}/edit`,
  /** Экран «Платежи объекта» — новый хром (#463). */
  propertyPayments: (id: string) => `/properties/${id}/payments`,
  /** Страницы секций «Платежей объекта» (Figma 1043:57610): клик по
   * заголовку секции ведёт на полный список — просроченные операции
   * объекта, все правила, автоплатежи. */
  propertyPaymentsOverdue: (id: string) => `/properties/${id}/payments/overdue`,
  propertyPaymentsAll: (id: string) => `/properties/${id}/payments/all`,
  propertyPaymentsAuto: (id: string) => `/properties/${id}/payments/auto`,
  /** Страница платежа (#465): карточка, мутации, секции вхождений. */
  propertyPayment: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}`,
  /** Экран «Задачи объекта» (#499): единый список групп-секций. */
  propertyTasks: (id: string) => `/properties/${id}/tasks`,
  /** Экран «Создать задачу» (#500): полноэкранная форма в два шага. */
  propertyTaskCreate: (id: string) => `/properties/${id}/tasks/new`,
  /** Экран «Изменить задачу» (#502): правка правила, тапом по строке списка. */
  propertyTaskEdit: (id: string, ruleId: string) => `/properties/${id}/tasks/${ruleId}/edit`,
  /** Подэкраны страницы платежа (#466): график, история, просрочки. */
  propertyPaymentSchedule: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}/schedule`,
  propertyPaymentHistory: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}/history`,
  propertyPaymentOverdue: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}/overdue`,
  /** Страница операции: вхождение правила; «Отметить оплаченной» — отсюда. */
  propertyOperation: (id: string, operationId: string) =>
    `/properties/${id}/operations/${operationId}`,
  /** Экран «Операции объекта» — срез «Рентли. Новые экраны сервиса» (#474). */
  propertyOperations: (id: string) => `/properties/${id}/operations`,
  /** Экраны «Доходы объекта»/«Расходы объекта» (#475): список одного
   * направления за месяц с листанием. */
  propertyOperationsIncome: (id: string) => `/properties/${id}/operations/income`,
  propertyOperationsExpense: (id: string) => `/properties/${id}/operations/expense`,
  /** Поиск по операциям объекта (Figma 1494-61633…; экран — тикет #476). */
  propertyOperationsSearch: (id: string) => `/properties/${id}/operations/search`,
  /** Страницы выбора периода и категорий (#477, Figma 1495-64015,
   * 1502-65940, 1502-66758 / 1506-72116, 1510-74149). */
  propertyOperationsPeriod: (id: string) => `/properties/${id}/operations/period`,
  propertyOperationsCategories: (id: string) => `/properties/${id}/operations/categories`,
  /** Просмотр проекции будущего вхождения (чисто фронт, без id операции). */
  propertyPaymentProjectedOperation: (id: string, paymentId: string, date: string) =>
    `/properties/${id}/payments/${paymentId}/operations/projected/${date}`,
  /** Экран правки платежа (#467): форма поверх правила, удаление — там же. */
  propertyPaymentEdit: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}/edit`,
  /** Визард создания платежа; тип выбирается в шите выбора «Платёж / Автоплатёж». */
  propertyPaymentNew: (id: string, type: 'payment' | 'autopayment') =>
    `/properties/${id}/payments/new?type=${type}`,
  /** Экран «Контакты объекта» (#508) — новый хром, книга контактов (ADR 0051). */
  propertyContacts: (id: string) => `/properties/${id}/contacts`,
  /** Создание контакта (#509); до тикета путь ведёт на 404. */
  propertyContactNew: (id: string) => `/properties/${id}/contacts/new`,
  /** Карточка контакта (#510): деталка с правкой и удалением. */
  propertyContact: (id: string, contactId: string) =>
    `/properties/${id}/contacts/${contactId}`,
  /** Экран правки контакта (#510): форма создания в режиме правки. */
  propertyContactEdit: (id: string, contactId: string) =>
    `/properties/${id}/contacts/${contactId}/edit`,
  /** Плоская книга контактов (глобальная страница, макеты 1726:65083/…). */
  contacts: '/contacts',
  /** Создание контакта из книги (без фиксированной привязки). */
  contactNew: '/contacts/new',
  /** Карточка контакта из книги. */
  contact: (contactId: string) => `/contacts/${contactId}`,
  /** Правка контакта из книги. */
  contactEdit: (contactId: string) => `/contacts/${contactId}/edit`,
  profile: '/profile',
  profilePersonal: '/profile/personal',
  profileNotifications: '/profile/notifications',
  profileAccount: '/profile/account',
  profileChangePhone: '/profile/account/phone',
  profileTariff: '/profile/tariff',
  profileTariffChange: '/profile/tariff/change',
  profileTariffChangeSuccess: '/profile/tariff/change/success',
  profilePaymentMethods: '/profile/tariff/payment-methods',
  profilePaymentMethodsAdd: '/profile/tariff/payment-methods/add',
  profilePayments: '/profile/tariff/payments',
  profilePaymentDetail: (id: string) => `/profile/tariff/payments/${id}`,
  profileInfo: '/profile/info',
  profilePrivacy: '/profile/info/privacy',
  profileTerms: '/profile/info/terms',
} as const;

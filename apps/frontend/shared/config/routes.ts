export const ROUTES = {
  login: '/login',
  properties: '/properties',
  property: (id: string) => `/properties/${id}`,
  propertyArchive: '/properties/archive',
  propertyNew: '/properties/new',
  /** Поиск по объектам (#586): вход — пилюля на хабе «Объекты». */
  propertySearch: '/properties/search',
  propertyEdit: (id: string) => `/properties/${id}/edit`,
  /** Экран «Об объекте» (деталь #588, шаблон 1550:95856): данные и
   * характеристики read-only, карандаш в шапке ведёт на правку; вход —
   * пункт кебаба «Об объекте» и секция «Квартира» на детали. */
  propertyAbout: (id: string) => `/properties/${id}/about`,
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
  /** Экран «Задачи» — глобальная лента всех задач (карта #518, тикет #523):
   * merged-фид читателя, топ-уровень рядом с объектами. */
  tasks: '/tasks',
  /** Экран «Создать задачу» (#500): полноэкранная форма в два шага. */
  propertyTaskCreate: (id: string) => `/properties/${id}/tasks/new`,
  /** Тот же экран с глобальной ленты «Задачи» (#525): вход без
   * предвыбранного объекта. */
  taskCreate: '/tasks/new',
  /** Экран «Изменить задачу» (#502): правка правила, тапом по строке списка. */
  propertyTaskEdit: (id: string, ruleId: string) => `/properties/${id}/tasks/${ruleId}/edit`,
  /** Плоский маршрут правки безобъектного правила (#537): та же форма
   * «Изменить задачу» вне объекта; тап по активной безобъектной строке
   * глобальной ленты (#523). Правило объекта на нём невидимо (privacy 404). */
  taskEdit: (ruleId: string) => `/tasks/${ruleId}/edit`,
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
  /** Экран «Операции» — глобальная лента по всем объектам (карта #545,
   * тикет #541): платёжные факты видимой книги, вход — ПК-сайдбар (#539). */
  operations: '/operations',
  /** Поиск по глобальным операциям (#543): вход — пилюля на главной. */
  operationsSearch: '/operations/search',
  /** «Выбрать объект» — мультивыбор фильтра глобальных операций (#542). */
  operationsObjects: '/operations/objects',
  /** Глобальный выбор категории фильтра (#544). */
  operationsCategories: '/operations/categories',
  /** Страницы направления глобальной ленты (#548): все доходы/расходы
   * выбранного скоупа за период; вход — карточки сводки на главной. */
  operationsExpenses: '/operations/expenses',
  operationsIncomes: '/operations/incomes',
  /** Визард создания одиночной операции (#570): глобальный вход — с шагом
   * «Выбрать объект»; направление — пресет ?type= (Расходы/Доходы). */
  operationsNew: '/operations/new',
  /** Экран «Платежи» — глобальная страница платежей (карта #573, тикет
   * #578): вход — меню «Ещё» и ПК-сайдбар. */
  payments: '/payments',
  /** Поиск по платежам (#575, экран #581): вход — пилюля на главной. */
  paymentsSearch: '/payments/search',
  /** Страницы категорий главного экрана: избранные (#579), просроченные
   * (#580), объекты и поиск объектов (#582). */
  paymentsFavorites: '/payments/favorites',
  paymentsOverdue: '/payments/overdue',
  paymentsObjects: '/payments/objects',
  paymentsObjectsSearch: '/payments/objects/search',
  /** Экран «Операции объекта» — срез «Рентли. Новые экраны сервиса» (#474). */
  propertyOperations: (id: string) => `/properties/${id}/operations`,
  /** Визард создания операции с объекта (#570): объектного шага нет,
   * категория — последний шаг с сабмитом. */
  propertyOperationsNew: (id: string) => `/properties/${id}/operations/new`,
  /** Экраны «Доходы объекта»/«Расходы объекта» (#475): список одного
   * направления за месяц с листанием. */
  propertyOperationsIncome: (id: string) => `/properties/${id}/operations/income`,
  propertyOperationsExpense: (id: string) => `/properties/${id}/operations/expense`,
  /** Поиск по операциям объекта (Figma 1494-61633…; экран — тикет #476). */
  propertyOperationsSearch: (id: string) => `/properties/${id}/operations/search`,
  /** Страница выбора категорий (#477, Figma 1506-72116, 1510-74149);
   * выбор периода — канонический пикер поверх списков (2026-09-04). */
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
  /** Визард создания аренды (#530): 4 шага на одном маршруте. */
  propertyRentalNew: (id: string) => `/properties/${id}/rentals/new`,
  /** Экран «Аренда» (#531): пустое состояние или детализация текущей аренды. */
  propertyRental: (id: string) => `/properties/${id}/rentals`,
  /** «Условия аренды» read-only (#531) — полный просмотр условий текущей аренды. */
  propertyRentalTerms: (id: string) => `/properties/${id}/rentals/terms`,
  /** Правка условий аренды (#532): форма поверх условий, начало read-only. */
  propertyRentalTermsEdit: (id: string) => `/properties/${id}/rentals/terms/edit`,
  /** Продление аренды (#533): новая дата окончания поверх PATCH plannedEndDate. */
  propertyRentalExtend: (id: string) => `/properties/${id}/rentals/extend`,
  /** Завершение аренды (#534): мастер из подтверждения, даты, залога и итогов. */
  propertyRentalComplete: (id: string) => `/properties/${id}/rentals/complete`,
  /** «Прошлые аренды» (#535): список завершённых аренд объекта. */
  propertyRentalPast: (id: string) => `/properties/${id}/rentals/past`,
  /** Завершённая детализация (#535): карточка прошлой аренды по id. */
  propertyRentalCompleted: (id: string, rentalId: string) =>
    `/properties/${id}/rentals/${rentalId}`,
  /** «История операций» завершённой аренды (#535): paid-вхождения платежа. */
  propertyRentalCompletedHistory: (id: string, rentalId: string) =>
    `/properties/${id}/rentals/${rentalId}/history`,
  /** «Итоги аренды» завершённой (#535): read-only повтор шага итогов мастера. */
  propertyRentalCompletedSummary: (id: string, rentalId: string) =>
    `/properties/${id}/rentals/${rentalId}/summary`,
  /** «Условия аренды» завершённой (#535): тот же экран условий с ?rental=. */
  propertyRentalCompletedTerms: (id: string, rentalId: string) =>
    `/properties/${id}/rentals/terms?rental=${rentalId}`,
  /** Экран «Контакты объекта» (#508) — новый хром, книга контактов (ADR 0054). */
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
  /** Поиск по книге — отдельная страница с поисковой шапкой (как #508). */
  contactSearch: '/contacts/search',
  /** Создание контакта из книги (без фиксированной привязки). */
  contactNew: '/contacts/new',
  /** Карточка контакта из книги. */
  contact: (contactId: string) => `/contacts/${contactId}`,
  /** Правка контакта из книги. */
  contactEdit: (contactId: string) => `/contacts/${contactId}/edit`,
  /** Хаб «Совместный доступ» (карта #692, тикет #696): карточки «Ваши
   * участники» и «Объекты пользователей», CTA «Пригласить участника»;
   * вход — пункт навбара «Участники» и строка в профиле. */
  participants: '/participants',
  /** Экран «Ваши участники» (#697): список агрегатов, поиск, «Отозвать
   * всех». До тикета живёт как нейтральный каркас. */
  participantsList: '/participants/list',
  /** Экран «Объекты пользователей» (#701): чужие объекты с доступом.
   * До тикета живёт как нейтральный каркас. */
  participantsProperties: '/participants/properties',
  /** Экран приглашения участника (#699): мультиобъектный флоу. До тикета
   * живёт как нейтральный каркас. */
  participantsInvite: '/participants/invite',
  /** Экран «Поддержка» (регистрирует единая нав-модель #558; страница живет
   * до переезда на новый хром — карта #556). */
  support: '/support',
  profile: '/profile',
  profileNotifications: '/profile/notifications',
  profileAccount: '/profile/account',
  /** Пикер часового пояса (карта #591, тикет #594): полный список зон РФ
   * с поиском; сохранение — PATCH /me, назад — на аккаунт. */
  profileAccountTimezone: '/profile/account/timezone',
  profileChangePhone: '/profile/account/phone',
  profileTariff: '/profile/tariff',
  /** Экран «О тарифе» (карта #611, тикет #621): детали тарифа с главного
   * «Тариф»; до #621 страница живёт как нейтральный каркас. */
  profileTariffAbout: '/profile/tariff/about',
  /** Флоу отключения тарифа (карта #611, тикет #622) поверх «О тарифе»;
   * в #621 страница живёт как нейтральный каркас под кнопку. */
  profileTariffDisable: '/profile/tariff/disable',
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

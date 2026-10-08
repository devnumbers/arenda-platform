/**
 * Доменная модель контекста «Платежи» на фронте (словарь — payments/CONTEXT.md,
 * ADR 0047): платёж = правило, операция = вхождение; просрочка — вычисляемый
 * сервером статус `overdue`, долг не хранится отдельно. Даты — строки
 * 'YYYY-MM-DD' без времени и зон: «сегодня» для интерфейса считает сервер
 * по TZ собственника объекта, клиент зоны не знает (ADR 0048).
 */

/** Дата-строка 'YYYY-MM-DD' (домен-порт прототипа работает только с ней). */
export type IsoDate = string;

export type PaymentType = 'income' | 'expense';

/** За сколько дней предупреждать о вхождении («Напоминание о платеже»,
 * карта #822): контракт 1|3|7, не задан — напоминаний нет. Напоминание
 * живёт независимо от autoPay. */
export type PaymentReminderOffset = 1 | 3 | 7;

/** Статус вхождения; `overdue` — серверная проекция planned с прошедшей датой. */
export type PaymentOperationStatus = 'planned' | 'paid' | 'overdue';

/** Якорь расписания живёт в регулярности; генерация идёт с даты заведения. */
export type Recurrence =
  | { kind: 'daily' }
  | { kind: 'weekly'; weekdays: ReadonlyArray<number> } // 0=воскресенье..6=суббота
  | { kind: 'monthly'; daysOfMonth: ReadonlyArray<number>; lastDay: boolean } // дни 1..30; короткие месяцы прижимают к последнему дню; lastDay — его фактический последний день
  | { kind: 'yearly'; month: number; day: number }; // месяц 1..12; 29 февраля прижимается

/** Интервал паузы [from, to): from включительно, день возобновления to — нет. */
export type PauseInterval = {
  readonly from: IsoDate;
  /** Не задан — активная бессрочная пауза. */
  readonly to?: IsoDate;
};

export type PaymentCategoryView = {
  readonly source: 'default' | 'custom';
  /** Слаг дефолтного каталога (#447); у пользовательской категории его нет. */
  readonly slug?: string;
  /** Идентификатор пользовательской категории (следующий срез эндпоинтов). */
  readonly id?: string;
  readonly label: string;
};

/** Расписание правила — вход чистых функций порта (`lib/occurrences`). */
export type PaymentSchedule = Pick<
  Payment,
  'recurrence' | 'since' | 'endDate' | 'pauses'
>;

export type Payment = {
  readonly id: string;
  readonly propertyId: string;
  readonly type: PaymentType;
  readonly title: string;
  readonly amountKopecks: number;
  readonly recurrence: Recurrence;
  /** Дата заведения: нижняя граница генерации, серверная, не редактируется. */
  readonly since: IsoDate;
  /** Окончание действия; не задано — бессрочный. */
  readonly endDate?: IsoDate;
  /** Напоминание «за N дней»; не задано — напоминаний нет (карта #822). */
  readonly reminderOffsetDays?: PaymentReminderOffset;
  readonly autoPay: boolean;
  /** Гейт события «Автоплатёж исполнен» (#1189): false — молчащее правило
   * не шлёт уведомление об автоплатеже. Значим только у autoPay-правил. */
  readonly notifyAutoPaid: boolean;
  readonly category: PaymentCategoryView;
  readonly isFavorite: boolean;
  /** Завершённый платёж (CONTEXT.md): неоплаченных вхождений больше нет.
   * Вычисляется сервером на чтение, не хранится. */
  readonly isCompleted: boolean;
  /** «Следующая дата оплаты» (CONTEXT.md, #991): серверная истина строки
   * списка — хранённая плановая от сегодня владельца (ADR 0048), иначе
   * проекция; null — открытая пауза или завершённое правило. Список объекта
   * вычисляет её на каждом правиле; одиночные чтения (страница платежа,
   * ответы мутаций) всегда несут null — считать его «нет даты» там нельзя.
   * Подпись и сортировку строк (хаб, страницы секций) клиент строит из неё,
   * не из проекции правила (#993). */
  readonly nearestDate: IsoDate | null;
  /** Платёж управляется арендой (ADR 0053, #818): сумму, день оплаты,
   * автоплатёж и плановое окончание задаёт аренда — мутации правила
   * (пауза, правка, удаление) дают 409, экран их скрывает; «Оплатить» и
   * звезда остаются. */
  readonly isRentalManaged: boolean;
  /** Управляющая аренда завершена («Завершена» — финал, #1158): «Изменить
   * аренду» на экране платежа живёт, только пока аренда не завершена.
   * Серверная истина состояния аренды — клиентский предикат завершённости
   * правила им не является (долг и «Ожидает действия» держат правило
   * незавершённым при завершённой аренде — и наоборот). */
  readonly isRentalCompleted: boolean;
  readonly pauses: ReadonlyArray<PauseInterval>;
  readonly createdAt: string;
  readonly updatedAt: string;
};

export type PaymentOperation = {
  readonly id: string;
  readonly propertyId: string;
  /** Правило-источник; null — «платёж удалён» или ручной факт. */
  readonly paymentId: string | null;
  /** Плановая дата вхождения; при оплате не сдвигается. */
  readonly date: IsoDate;
  /** Фактическая дата оплаты. */
  readonly paidDate?: IsoDate;
  readonly status: PaymentOperationStatus;
  readonly type: PaymentType;
  readonly title: string;
  readonly amountKopecks: number;
  /** Подпись категории, замороженная при материализации. */
  readonly categoryLabel: string;
  /** Слаг дефолтного каталога для иконки; может отсутствовать. */
  readonly categorySlug?: string;
  /** Имя объекта — подпись строки глобальной ленты (#541); объектные
   * списки его не несут — объект известен из маршрута. */
  readonly propertyName?: string;
  /** Момент последнего касания строки; у оплаченной — момент оплаты
   * (строка после оплаты не меняется): история платежа ставит операцию в
   * ленту по реальному времени и читает порядок оплат внутри дня (#1195).
   * Нет у клиентской проекции будущего вхождения — строки за ней нет. */
  readonly updatedAt?: string;
};

/**
 * Создание платежа: контракт camelCase, тело запроса совпадает с командой
 * один в один (сервер ставит `since`, клиент её не передаёт).
 */
export type PaymentCreateCommand = {
  readonly type: PaymentType;
  readonly title: string;
  readonly amountKopecks: number;
  readonly recurrence: Recurrence;
  readonly categorySlug: string;
  readonly endDate?: IsoDate;
  /** Напоминание «за N дней»; не выбрано — поле не передаётся (карта #822). */
  readonly reminderOffsetDays?: PaymentReminderOffset;
  readonly autoPay?: boolean;
  /** «Уведомлять об автоплатеже» (#1193, макет 3214-72739): только
   * автоплатёжная ветка и только явное «Да, уведомлять» — иначе поле
   * опускается, сервер ставит false (#1189). */
  readonly notifyAutoPaid?: boolean;
};

/**
 * Частичная правка: опущенное поле остаётся без изменений; `endDate`
 * трисостоянен — опущен (сохранить), null (открыть срок), дата (назначить).
 */
export type PaymentUpdateCommand = Partial<Omit<PaymentCreateCommand, 'endDate'>> & {
  readonly endDate?: IsoDate | null;
};

/**
 * Создание ручной операции (#569): контракт camelCase, тело запроса
 * совпадает с командой один в один. Дату (сегодня в TZ собственника) и
 * статус paid ставит сервер — факт рождается оплаченным, клиент их не
 * передаёт.
 */
export type OperationCreateCommand = {
  readonly type: PaymentType;
  readonly title: string;
  readonly amountKopecks: number;
  readonly categorySlug: string;
};

/** Тело атомарного toggle избранного (PUT favorite). */
export type PaymentFavoriteCommand = {
  readonly favorite: boolean;
};
/** Строка категории в сводке периода (#473): слаг для иконки и стиля,
 * подпись-снапшот для текста, сумма по категории в копейках. */
export type OperationsCategorySummary = {
  readonly slug: string;
  readonly label: string;
  readonly type: PaymentType;
  readonly totalKopecks: number;
};

/**
 * Сводка периода объекта (#473) за экранами «Операции объекта»: итоги по
 * направлениям — всегда оба, карточки читают их вместе; разбивка по
 * категориям только с операциями в скоупе, по сумме убывание. Операции без
 * снапшота категории (пользовательские категории) считают в итогах, но
 * строки в разбивке не дают.
 */
export type OperationsSummary = {
  readonly incomeTotalKopecks: number;
  readonly expenseTotalKopecks: number;
  readonly categories: ReadonlyArray<OperationsCategorySummary>;
};

/**
 * Строка глобального фида платежей (карта #573, #575): правило в разрезе
 * всей видимой книги — с именем объекта для подписи карточки. «Ближайший»
 * (`nearestDate`) — просто дата графика без статуса: ранняя хранёная
 * planned от сегодня, иначе проекция правила; null — следующего вхождения
 * нет (открытая пауза или завершённое правило). Просрочка правила — счёт
 * накопленных planned-вхождений в прошлом и возраст старейшего из них
 * (в днях); `today` — календарное «сегодня» владельца объекта (ADR 0048).
 */
export type GlobalPayment = {
  readonly id: string;
  readonly propertyId: string;
  readonly propertyName: string;
  readonly title: string;
  readonly amountKopecks: number;
  readonly type: PaymentType;
  readonly category: PaymentCategoryView;
  readonly autoPay: boolean;
  readonly isFavorite: boolean;
  /** Позиция из сохранённого порядка избранного (#576); null — «в конец». */
  readonly favoriteOrder: number | null;
  readonly today: IsoDate;
  readonly nearestDate: IsoDate | null;
  readonly overdueOperationCount: number;
  readonly overdueDays: number | null;
  /** Старейшая просроченная операция — цель ссылки просроченной карточки
   * главного экрана (#578); null, если просроченных нет. */
  readonly oldestOverdueOperationId: string | null;
};

/** Фид главного экрана «Платежи» (#575): все видимые правила плюс счётчики
 * целого скоупа для карточек «Все избранные»/«Все просроченные». */
export type GlobalPaymentFeed = {
  readonly items: ReadonlyArray<GlobalPayment>;
  readonly favoriteCount: number;
  readonly overdueOperationsCount: number;
};

/** Ответ поиска глобальных платежей (#575): совпавшие строки — состав
 * фида без счётчиков — и чипы совпавших категорий: по одной на категорию,
 * без направления и счётчика (#602). nextCursor — keyset-продолжение
 * порции (#597): opaque-курсор следующей страницы, null = совпадения
 * исчерпаны. */
export type GlobalPaymentSearch = {
  readonly items: ReadonlyArray<GlobalPayment>;
  readonly matchedCategories: ReadonlyArray<PaymentCategoryView>;
  readonly nextCursor: string | null;
};

/** Ключ стопки объекта (#575): правило за карточкой стека и его флаг
 * просрочки — красная точка на карточке объекта. */
export type GlobalPaymentObjectKey = {
  readonly paymentId: string;
  readonly hasOverdue: boolean;
};

/** Объект в глобальных платежах (#575): имя и адрес для карточки, стопки
 * правил («Автоплатежи»/«Платежи») и момент закрепления (#577; null — не
 * закреплён, сервер отдаёт закреплённые первыми). photoUrl — первое (самое
 * старое) фото для аватара карточки (#582); null — фото нет. */
export type GlobalPaymentObject = {
  readonly propertyId: string;
  readonly name: string;
  readonly address: string;
  readonly pinnedAt: string | null;
  readonly photoUrl: string | null;
  readonly autoPayRules: ReadonlyArray<GlobalPaymentObjectKey>;
  readonly otherRules: ReadonlyArray<GlobalPaymentObjectKey>;
};

/**
 * Журнал изменений платежа (ADR 0065, карта #1183): одна строка на действие
 * пользователя — правка с дифом полей, пауза/возобновление с пустым дифом.
 * Значения дифа типизированы по словарю поля, никогда не строки-с-
 * форматированием — тексты чипов экрана «История платежа» собирает фронт.
 */

/** Род строки журнала: правка условий, пауза, возобновление. Создание и
 * удаление строк не пишут (ADR 0065 §4). */
export type PaymentChangeAction = 'updated' | 'paused' | 'resumed';

/** Ссылка на категорию в дифе — снапшот лейбла на момент правки (канон
 * category_label операций, ADR 0049 §1): чип читаем после переименования
 * или удаления пользовательской категории. */
export type PaymentChangeCategoryRef = {
  /** Слаг дефолтного каталога; у пользовательской категории его нет. */
  readonly slug?: string;
  /** Идентификатор пользовательской категории. */
  readonly userCategoryId?: string;
  readonly label: string;
};

/** Один уехавший поле дифа: старое→новое значение, типизировано по словарю
 * поля (ADR 0065 §2). Порядок строк дифа в ответе — словарный. */
export type PaymentFieldChange =
  | { readonly field: 'type'; readonly old: PaymentType | null; readonly new: PaymentType | null }
  | { readonly field: 'title'; readonly old: string | null; readonly new: string | null }
  | { readonly field: 'amount_kopecks'; readonly old: number | null; readonly new: number | null }
  | { readonly field: 'recurrence'; readonly old: Recurrence | null; readonly new: Recurrence | null }
  | {
      readonly field: 'category_slug';
      readonly old: PaymentChangeCategoryRef | null;
      readonly new: PaymentChangeCategoryRef | null;
    }
  | { readonly field: 'end_date'; readonly old: IsoDate | null; readonly new: IsoDate | null }
  | { readonly field: 'auto_pay'; readonly old: boolean | null; readonly new: boolean | null }
  | {
      readonly field: 'reminder_offset_days';
      readonly old: PaymentReminderOffset | null;
      readonly new: PaymentReminderOffset | null;
    };

/** Строка журнала изменений (ADR 0065): кто и какое действие совершил
 * (actor остаётся на DTO — экран актора не показывает) и какие поля
 * двинулись. createdAt — полный момент действия, ISO date-time. */
export type PaymentChangeEntry = {
  readonly id: string;
  readonly action: PaymentChangeAction;
  readonly changes: ReadonlyArray<PaymentFieldChange>;
  readonly createdAt: string;
};

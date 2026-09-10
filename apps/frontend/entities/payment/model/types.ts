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
export type PaymentForm = 'transfer' | 'cash';

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
  readonly autoPay: boolean;
  readonly paymentForm: PaymentForm;
  readonly category: PaymentCategoryView;
  readonly isFavorite: boolean;
  /** Завершённый платёж (CONTEXT.md): неоплаченных вхождений больше нет.
   * Вычисляется сервером на чтение, не хранится. */
  readonly isCompleted: boolean;
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
  /** Снапшот формы правила; у ручных операций не задан. */
  readonly paymentForm?: PaymentForm;
  /** Подпись категории, замороженная при материализации. */
  readonly categoryLabel: string;
  /** Слаг дефолтного каталога для иконки; может отсутствовать. */
  readonly categorySlug?: string;
  /** Имя объекта — подпись строки глобальной ленты (#541); объектные
   * списки его не несут — объект известен из маршрута. */
  readonly propertyName?: string;
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
  readonly paymentForm: PaymentForm;
  readonly categorySlug: string;
  readonly endDate?: IsoDate;
  readonly autoPay?: boolean;
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
 * передаёт; формы оплаты у ручного факта нет.
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

/** Чип категорий поиска платежей (#575, экран #581): категория с
 * направлением и числом совпавших правил; порядок отдаёт сервер. */
export type PaymentSearchCategoryView = {
  readonly category: PaymentCategoryView;
  readonly type: PaymentType;
  readonly count: number;
};

/** Ответ поиска глобальных платежей (#575): совпавшие строки — состав
 * фида без счётчиков — и чипы совпавших категорий. */
export type GlobalPaymentSearch = {
  readonly items: ReadonlyArray<GlobalPayment>;
  readonly matchedCategories: ReadonlyArray<PaymentSearchCategoryView>;
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

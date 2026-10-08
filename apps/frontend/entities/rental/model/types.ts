import type { IsoDate } from '@/shared/lib/calendar';

/**
 * Доменная модель аренды (ADR 0053, словарь rentals/CONTEXT.md): период
 * занятости объекта с условиями, управляющий ровно одним Платежем
 * контекста payments. День оплаты на аренде не хранится — читается из
 * Платежа; ответ API дублирует его полем paymentDay (решение №5).
 */

/** День оплаты: число месяца 1–31 или «последний день месяца»; в коротком
 * месяце оба прижимаются к последнему дню (одно поведение, №5). */
export type RentalPaymentDay = number | 'last';

/** Коммунальные платежи: «Включены в стоимость» / «Только счетчики» /
 * «Вся квитанция». */
export type RentalUtilities = 'included' | 'meters_only' | 'full_receipt';

/** Вычисляемые сервером статусы: хранится только факт завершения (№1). */
export type RentalStatus = 'upcoming' | 'active' | 'needs_attention' | 'completed';

/** Встроенный арендатор — поля карточки контакта; после удаления контакта
 * приходит null без пометки («Контакта нет», решение #528). */
export type RentalTenant = {
  readonly contactId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly phone: string;
};

/** Единственное будущее planned-вхождение Платежа арендной платы. */
export type RentalNextPayment = {
  readonly operationId: string;
  readonly date: IsoDate;
  readonly amountKopecks: number;
  readonly daysUntil: number;
};

/** Состояние Платежа арендной платы в ответе аренды: paymentId — переход на
 * экран платежа (#531); платеж не ищется слагом категории (ADR 0053 §4).
 * Ревизия #1161: у завершённой аренды платёж удалён — paymentId null, а
 * сумма и день оплаты приходят из Архива условий на аренде. */
export type RentalPaymentView = {
  readonly paymentId: string | null;
  readonly amountKopecks: number;
  readonly paymentDay: RentalPaymentDay;
  readonly autoPay: boolean;
  readonly nextPayment: RentalNextPayment | null;
};

/** Прогресс «Оплачено N из M месяцев»: totalMonths/monthsRemaining — только
 * у срочной аренды (null у бессрочной); overdueMonths — серверная просрочка
 * Платежа арендной платы (#817), null — просрочки нет. */
export type RentalProgress = {
  readonly paidMonths: number;
  readonly totalMonths: number | null;
  readonly monthsRemaining: number | null;
  readonly overdueMonths: number | null;
};

/** Аренда с вычисляемым сервером состоянием: статус, прогресс, «сегодня» и
 * будущий платёж — клиент пояса не знает (ADR 0053 §2). */
export type Rental = {
  readonly id: string;
  readonly propertyId: string;
  readonly status: RentalStatus;
  readonly startDate: IsoDate;
  readonly plannedEndDate: IsoDate | null;
  readonly completedDate: IsoDate | null;
  readonly utilities: RentalUtilities;
  readonly depositKopecks: number | null;
  readonly commissionKopecks: number | null;
  readonly depositReturnKopecks: number | null;
  readonly depositReturnComment: string | null;
  readonly tenant: RentalTenant | null;
  readonly comment: string;
  readonly rentPayment: RentalPaymentView;
  readonly progress: RentalProgress;
  readonly today: IsoDate;
  readonly createdAt: string;
};

/**
 * Команда создания аренды (POST /properties/{propertyId}/rentals):
 * camelCase 1:1 с RentalCreateRequest — контракт уже в camelCase,
 * отдельный wire-сериализатор не нужен. Начало — сегодня или позже по TZ
 * собственника; плановое окончание — строго позже начала; залог, комиссия
 * и арендатор необязательны (null = не заданы).
 */
export type RentalCreateCommand = {
  readonly amountKopecks: number;
  readonly paymentDay: RentalPaymentDay;
  readonly startDate: IsoDate;
  readonly plannedEndDate: IsoDate | null;
  readonly utilities: RentalUtilities;
  readonly depositKopecks: number | null;
  readonly commissionKopecks: number | null;
  readonly contactId: string | null;
  readonly autoPay: boolean;
  /** Лид-тайм напоминания о платеже арендной платы, протекает в её Платёж
   * 1:1 (карта #822): литералы контракта 1|3|7 — словарь опций живёт
   * слайсом платежа (PAYMENT_REMINDER_OPTIONS), entity-слой соседних
   * слайсов не импортирует. Дефолт экрана — «Не напоминать»: null едет
   * в команду явно, дефолт не протекает молча (#1198). */
  readonly reminderOffsetDays: 1 | 3 | 7 | null;
};

/**
 * Команда правки условий (PATCH /properties/{propertyId}/rentals/{rentalId},
 * #532): частичное тело — включены только изменённые поля (дифф формы,
 * прецедент правки платежа #467). Nullable-поля — tri-state: опущенное
 * остаётся без изменений, явный null очищает (ADR 0053 §4). Начало не
 * правится; сумма, день оплаты, автоплатёж и окончание сервер синхронно
 * переносит на Платёж арендной платы.
 */
export type RentalUpdateCommand = {
  readonly amountKopecks?: number;
  readonly paymentDay?: RentalPaymentDay;
  readonly autoPay?: boolean;
  readonly plannedEndDate?: IsoDate | null;
  readonly utilities?: RentalUtilities;
  readonly depositKopecks?: number | null;
  readonly commissionKopecks?: number | null;
  readonly comment?: string | null;
};

/**
 * Команда завершения аренды (POST …/complete, #534): дата завершения в
 * границах [начало, сегодня] — «По плану» подставляет клиент (ADR 0053 §3).
 * Возврат залога — запись при завершении: 0 валиден («не вернул»),
 * комментарий только при сумме.
 */
export type RentalCompleteCommand = {
  readonly completedDate: IsoDate;
  readonly depositReturn?: {
    readonly amountKopecks: number;
    readonly comment?: string;
  };
};

/**
 * Итоги аренды (GET …/summary, решение №13): все paid-операции объекта —
 * любого платежа и ручные — с датой вхождения в периоде [from, until].
 * Прибыль = доходы − расходы, может быть отрицательной; вычисляется на
 * чтении, снапшота нет.
 */
export type RentalSummary = {
  readonly from: IsoDate;
  readonly until: IsoDate;
  readonly incomeKopecks: number;
  readonly expenseKopecks: number;
  readonly profitKopecks: number;
};

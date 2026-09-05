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
 * экран платежа (#531); платеж не ищется слагом категории (ADR 0053 §4). */
export type RentalPaymentView = {
  readonly paymentId: string;
  readonly amountKopecks: number;
  readonly paymentDay: RentalPaymentDay;
  readonly autoPay: boolean;
  readonly nextPayment: RentalNextPayment | null;
};

/** Прогресс «Оплачено N из M месяцев»: totalMonths/monthsRemaining — только
 * у срочной аренды (null у бессрочной). */
export type RentalProgress = {
  readonly paidMonths: number;
  readonly totalMonths: number | null;
  readonly monthsRemaining: number | null;
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
};

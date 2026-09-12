import type { TariffName } from '@/shared/model/tariff';

export type PaymentStatus =
  | 'pending'
  | 'succeeded'
  | 'failed'
  | 'refunded';

export type PaymentPeriod = 'month' | 'year';

export const PAYMENT_STATUS_LABELS: Record<PaymentStatus, string> = {
  pending: 'В ожидании',
  succeeded: 'Выполнена',
  failed: 'Не выполнено',
  refunded: 'Возврат',
};

/** Карта, которой платёж был оплачен (#619): снимок на момент создания/
 * финализации платежа — история живёт пережитым удаление способа оплаты.
 * `cardSystem` бэк выводит из BIN-префикса; без распознавания — unknown. */
export type SubscriptionPaymentCard = {
  readonly displayMask: string;
  readonly cardSystem: 'mir' | 'visa' | 'mastercard' | 'unknown';
};

export const PAYMENT_PERIOD_LABELS: Record<PaymentPeriod, string> = {
  month: 'месяц',
  year: 'год',
};

export type SubscriptionStatus = 'active' | 'grace' | 'cancelled';

export type Tariff = {
  id: string;
  name: TariffName;
  monthlyPriceKopecks: number;
  yearlyPriceKopecks: number;
  activePropertyLimit: number;
};

export type PaymentMethod = {
  id: string;
  displayMask: string;
  provider: string;
  isActive: boolean;
  createdAt: string;
};

/** Живая pending-оплата (#616): одна на юзера, с ссылкой подтверждения банка
 * и абсолютным сроком жизни формы — якорем обратного отсчёта. */
export type PendingPayment = {
  tariffName: TariffName;
  period: PaymentPeriod;
  amountKopecks: number;
  confirmUrl: string;
  expiresAt: string;
};

export type Subscription = {
  id: string;
  status: SubscriptionStatus;
  tariff: Tariff;
  pendingTariff?: Tariff;
  autoRenewEnabled: boolean;
  validUntil?: string;
  currentPeriod?: PaymentPeriod;
  activePaymentMethod?: PaymentMethod;
  pendingChangeAt?: string;
  pendingPeriod?: PaymentPeriod;
  pendingPayment?: PendingPayment;
};

export type SubscriptionPayment = {
  id: string;
  tariff: Tariff;
  period: PaymentPeriod;
  amountKopecks: number;
  status: PaymentStatus;
  provider: string;
  paymentUrl: string | null;
  paymentMethod?: SubscriptionPaymentCard;
  succeededAt?: string;
  createdAt: string;
};

export type ChangeTariffResult = {
  paymentId?: string;
  confirmUrl?: string;
};

export type AddPaymentMethodResult = {
  confirmUrl?: string;
  paymentMethod?: PaymentMethod;
};

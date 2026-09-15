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
 * финализации платежа — история переживает удаление способа оплаты.
 * `cardSystem` бэк выводит из BIN-префикса; без распознавания — unknown. */
export type SubscriptionPaymentCard = {
  readonly displayMask: string;
  readonly cardSystem: 'mir' | 'visa' | 'mastercard' | 'unknown';
};

export const PAYMENT_PERIOD_LABELS: Record<PaymentPeriod, string> = {
  month: 'месяц',
  year: 'год',
};

/** Номер карты как в макетах (#622-правка, 1918-73255: «•••• 0700») —
 * только хвост из 4 цифр маски, без BIN и названия системы (решение
 * владельца: систему карты в интерфейсе не определяем). Маска без 4 цифр
 * — как есть. Единый формат показа displayMask во всём биллинге. */
export function cardNumberTail(mask: string): string {
  const digits = mask.replace(/\D/g, '');
  return digits.length < 4 ? mask : `•••• ${digits.slice(-4)}`;
}

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
  /** Система карты — бэк выводит из BIN-префикса маски (#614);
   * без распознавания — unknown. */
  cardSystem: 'mir' | 'visa' | 'mastercard' | 'unknown';
  /** Срок действия в формате провайдера (MMYY); неизвестен — null. */
  expDate?: string;
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

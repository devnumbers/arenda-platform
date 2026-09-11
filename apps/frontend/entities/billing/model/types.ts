import type { TariffName } from '@/shared/model/tariff';

export type PaymentStatus =
  | 'pending'
  | 'succeeded'
  | 'failed'
  | 'refunded';

export type PaymentPeriod = 'month' | 'year';

export const PAYMENT_STATUS_LABELS: Record<PaymentStatus, string> = {
  pending: 'В обработке',
  succeeded: 'Успешно',
  failed: 'Ошибка',
  refunded: 'Возвращён',
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
};

export type SubscriptionPayment = {
  id: string;
  tariff: Tariff;
  period: PaymentPeriod;
  amountKopecks: number;
  status: PaymentStatus;
  provider: string;
  paymentUrl: string | null;
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

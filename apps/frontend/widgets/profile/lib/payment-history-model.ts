import type {
  PaymentPeriod,
  SubscriptionPayment,
  SubscriptionPaymentCard,
} from '@/entities/billing';
import type { IsoDate } from '@/shared/lib/calendar';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { PAYMENT_STATUS_LABELS } from '@/entities/billing';

/**
 * Презентационная модель экрана «Операции» (#624): группировка истории
 * подписочных платежей по датам, знаки и тона сумм, маски карт, копия
 * макета.
 *
 * Группировка — канон дат shared/lib («10 августа», год вне текущего);
 * день берётся по локальным часам смотрящего (dateToIsoLocal): бэк для
 * таймстампов createdAt календарный день не отдаёт, расхождение с TZ
 * собственника ограничено краевыми часами суток (#453, тот же довод у
 * клиентского «сегодня»). Порядок групп повторяет серверную сортировку
 * входа (новые сверху, #619).
 */

/** Пропсы суммы для канонической строки (PaymentRowButton): знак и тон
 * выводятся из статуса платежа (сумма в DTO — сумма списания, всегда
 * положительная). Выполнено/не выполнено — «−», возврат — «+» зелёным,
 * ожидание — без знака; служебный refunding приходит уже нормализованным
 * в pending (маппер entities/billing, решение владельца #614). Тон
 * значения — оверрайдом valueClassName: у не выполненного красным
 * становится только сумма, описание-маска остаётся серым (макет
 * 1877-68603), поэтому канон-флаг danger (красит и описание) не
 * используется. */
export type PaymentRowAmountProps = {
  readonly amountKopecks: number;
  readonly signedAmount: boolean;
  readonly valueClassName?: string;
};

export function paymentRowAmountProps(payment: SubscriptionPayment): PaymentRowAmountProps {
  switch (payment.status) {
    case 'failed':
      return {
        amountKopecks: -payment.amountKopecks,
        signedAmount: true,
        valueClassName: 'text-danger',
      };
    case 'refunded':
      return {
        amountKopecks: payment.amountKopecks,
        signedAmount: true,
        valueClassName: 'text-success',
      };
    case 'pending':
      return { amountKopecks: payment.amountKopecks, signedAmount: false };
    case 'succeeded':
      return { amountKopecks: -payment.amountKopecks, signedAmount: true };
  }
}

/** Подзаголовок строки списка: у выполненной его нет, у остальных —
 * статус (макет 1877-68603: «Не выполнено», «Возврат», «В ожидании»). */
export function paymentRowSubtitle(payment: SubscriptionPayment): string | undefined {
  return payment.status === 'succeeded'
    ? undefined
    : PAYMENT_STATUS_LABELS[payment.status];
}

/** Цвет строки «Статус» в детали (#624, Figma 1904-40495): ожидание —
 * синим primary, не выполнено — danger, возврат — success, выполнена —
 * тёмным (без класса). */
export function paymentStatusTone(status: SubscriptionPayment['status']): string | undefined {
  switch (status) {
    case 'pending':
      return 'text-primary';
    case 'failed':
      return 'text-danger';
    case 'refunded':
      return 'text-success';
    case 'succeeded':
      return undefined;
  }
}

const CARD_SYSTEM_LABELS: Record<SubscriptionPaymentCard['cardSystem'], string | undefined> = {
  mir: 'Мир',
  visa: 'Visa',
  mastercard: 'Mastercard',
  unknown: undefined,
};

/** Маска карты со строкой платежа: «Мир •• 0700» — система от бэка
 * (фронт систему не выводит, #622), банк не называется (решение
 * владельца 11.09, #614); нераспознанная система — без префикса. Без
 * карты (старые платежи) маски нет. */
export function paymentCardMask(payment: SubscriptionPayment): string | undefined {
  const card = payment.paymentMethod;
  if (card === undefined) {
    return undefined;
  }
  const digits = card.displayMask.replace(/\D/g, '');
  const tail = digits.length < 4 ? card.displayMask : `•• ${digits.slice(-4)}`;
  const label = CARD_SYSTEM_LABELS[card.cardSystem];
  return label === undefined ? tail : `${label} ${tail}`;
}

/** Копия макета детали (#624, Figma 1883-71611): «В месяц» / «В год» —
 * в отличие от словаря «месяц/год» для фраз «в месяц» (tariff-overview). */
export function paymentPeriodLabel(period: PaymentPeriod): string {
  return period === 'month' ? 'В месяц' : 'В год';
}

export type PaymentHistoryGroup = {
  /** Локальный календарный день группы ('YYYY-MM-DD') — ключ секции. */
  readonly date: IsoDate;
  readonly label: string;
  readonly payments: ReadonlyArray<SubscriptionPayment>;
};

/** Подряд идущие платежи одного дня складываются в группу (подобно
 * groupPaidOperations учётных операций, #452); «Сегодня»/«Вчера» макет
 * не использует — только дата. */
export function groupSubscriptionPaymentsByDate(
  payments: ReadonlyArray<SubscriptionPayment>,
  today: IsoDate,
): ReadonlyArray<PaymentHistoryGroup> {
  const groups: { date: IsoDate; label: string; payments: SubscriptionPayment[] }[] = [];
  for (const payment of payments) {
    const date = dateToIsoLocal(new Date(payment.createdAt));
    const current = groups.at(-1);
    if (current !== undefined && current.date === date) {
      current.payments.push(payment);
      continue;
    }
    groups.push({ date, label: formatDayMonthWithYear(date, today), payments: [payment] });
  }
  return groups;
}

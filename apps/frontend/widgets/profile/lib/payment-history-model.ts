import type { IsoDate } from '@/shared/lib/calendar';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import type {
  PaymentPeriod,
  SubscriptionPayment,
} from '@/entities/billing';
import { PAYMENT_STATUS_LABELS } from '@/entities/billing';
import { cardNumberTail } from '@/entities/billing';

/**
 * Презентационная модель платёжных экранов профиля (#624): группировка
 * истории подписочных платежей по датам, знаки и тона сумм, маски карт,
 * срок жизни платёжной формы, копия макета.
 *
 * Группировка — канон дат shared/lib («10 августа», год вне текущего);
 * день берётся по локальным часам смотрящего (dateToIsoLocal): бэк для
 * таймстампов createdAt календарный день не отдаёт, расхождение с TZ
 * собственника ограничено краевыми часами суток (#453, тот же довод у
 * клиентского «сегодня»). Порядок групп повторяет серверную сортировку
 * входа (новые сверху, #619).
 */

/** Дедлайн «Вернуться к оплате» (#680) — серверная истина: у банковской
 * pending это expiresAt (тот же момент, что ушёл провайдеру как дедлайн
 * редиректа), поэтому клиент больше не дублирует TTL формы. У платежа
 * без формы срока нет: не-banking статус, MIT-pending, legacy-строка.
 * Единственный предикат «у платежа есть форма» для обоих потребителей. */
export function paymentFormDeadline(payment: SubscriptionPayment): string | null {
  if (
    payment.status !== 'pending' ||
    payment.paymentUrl === null ||
    payment.expiresAt === null
  ) {
    return null;
  }
  return payment.expiresAt;
}

/** Форма просрочена по серверному дедлайну: pending пережил свой срок
 * жизни (#616) — копия экрана меняется с успокаивающей на «проверяем
 * у банка». У платежа без формы просрочки нет. */
export function isPaymentFormExpired(payment: SubscriptionPayment, now: Date): boolean {
  const deadline = paymentFormDeadline(payment);
  if (deadline === null) {
    return false;
  }
  return now.getTime() >= new Date(deadline).getTime();
}

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

/** Маска карты со строкой платежа: «•••• 0700» — только хвост, систему
 * карты в интерфейсе не показываем (решение владельца 12.09, #624; тот
 * же формат, что на «О тарифе» — #622). Без карты (старые платежи)
 * маски нет. */
export function paymentCardMask(payment: SubscriptionPayment): string | undefined {
  const card = payment.paymentMethod;
  if (card === undefined) {
    return undefined;
  }
  return cardNumberTail(card.displayMask);
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

import {
  PAYMENT_PERIOD_LABELS,
  type PendingPayment,
  type Subscription,
} from '@/entities/billing';
import { isPaidTariff, getTariffLabel } from '@/entities/user';
import { dateToIso } from '@/shared/lib/calendar';
import { formatDayMonth } from '@/shared/lib/date-format';
import { formatMoneyKopecks } from '@/shared/lib/format-money';

/** Вариант hero-карточки главного экрана «Тариф» (#620, макеты
 * 1879-70076 «Про», 1917-72334 «Базовый», 1917-72464 «Бизнес» + grace,
 * 1943-115049 «Про остановлен»): вариант выбирает фон, строки и CTA,
 * компонент только рисует. */
export type TariffHero =
  | { kind: 'paid'; title: string; priceLine: string; subLine?: string }
  | { kind: 'basic'; title: string; subLine: string }
  | { kind: 'grace'; title: string; priceLine: string; subLine: string }
  | { kind: 'stopped'; title: string; dateLine?: string; subLine: string };

/** День из серверной даты-времени строкой «10 сентября» (UTC, ADR 0048). */
export function dayMonth(datetime: string): string {
  return formatDayMonth(dateToIso(new Date(datetime)));
}

function priceLine(subscription: Subscription): string {
  const period = subscription.currentPeriod ?? 'month';
  const price = period === 'year'
    ? subscription.tariff.yearlyPriceKopecks
    : subscription.tariff.monthlyPriceKopecks;
  return `Вы платите ${formatMoneyKopecks(price)} в ${PAYMENT_PERIOD_LABELS[period]}`;
}

export function tariffHero(subscription: Subscription): TariffHero {
  const title = getTariffLabel(subscription.tariff.name);

  if (subscription.status === 'cancelled') {
    return {
      kind: 'stopped',
      title: `${title} остановлен`,
      dateLine: subscription.validUntil ? dayMonth(subscription.validUntil) : undefined,
      subLine: 'Действует до',
    };
  }

  if (subscription.status === 'grace') {
    return {
      kind: 'grace',
      title,
      priceLine: priceLine(subscription),
      subLine: subscription.validUntil
        ? `Оплатите тариф до ${dayMonth(subscription.validUntil)}`
        : 'Оплатите тариф',
    };
  }

  if (!isPaidTariff(subscription.tariff.name)) {
    return { kind: 'basic', title, subLine: 'Бесплатно' };
  }

  return {
    kind: 'paid',
    title,
    priceLine: priceLine(subscription),
    subLine: subscription.validUntil ? `Спишем ${dayMonth(subscription.validUntil)}` : undefined,
  };
}

/** Обратный отсчёт до срока жизни платёжной формы (#616): «MM:SS»,
 * в прошлом — «00:00». `now` передаётся параметром — чистая функция. */
export function formatPaymentCountdown(expiresAt: string, now: Date): string {
  const remainingMs = new Date(expiresAt).getTime() - now.getTime();
  const totalSeconds = Math.max(0, Math.floor(remainingMs / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
}

/** Текст синей плашки «Ожидаем оплату» (#620, макет 1927-75410):
 * тариф, сумма и период ждущего платежа + призыв вернуться в банк. */
export function pendingPaymentDescription(pending: PendingPayment): string {
  return `Тариф ${getTariffLabel(pending.tariffName)}`
    + ` за ${formatMoneyKopecks(pending.amountKopecks)}`
    + ` в ${PAYMENT_PERIOD_LABELS[pending.period]}.`
    + ' Вернитесь на страницу банка, чтобы завершить оплату';
}

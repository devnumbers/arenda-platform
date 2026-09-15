import { describe, expect, it } from 'vitest';
import {
  formatPaymentCountdown,
  pendingPaymentDescription,
  tariffHero,
} from './tariff-overview';
import type { PendingPayment, Subscription, Tariff } from '@/entities/billing';

const proTariff: Tariff = {
  id: 'pro',
  name: 'pro',
  monthlyPriceKopecks: 49000,
  yearlyPriceKopecks: 490000,
  activePropertyLimit: 5,
};

const businessTariff: Tariff = {
  id: 'business',
  name: 'business',
  monthlyPriceKopecks: 99000,
  yearlyPriceKopecks: 890000,
  activePropertyLimit: -1,
};

function subscription(overrides: Partial<Subscription>): Subscription {
  return {
    id: 'active-pro-2026-09-10',
    status: 'active',
    tariff: proTariff,
    autoRenewEnabled: true,
    validUntil: '2026-09-10T21:00:00Z',
    currentPeriod: 'month',
    ...overrides,
  };
}

describe('tariffHero', () => {
  it('paid month: price line and the next charge date', () => {
    expect(tariffHero(subscription({}))).toStrictEqual({
      kind: 'paid',
      title: 'Про',
      priceLine: 'Вы платите 490 ₽ в месяц',
      subLine: 'Спишем 10 сентября',
    });
  });

  it('paid year: price line follows the current period', () => {
    expect(tariffHero(subscription({ currentPeriod: 'year' }))).toStrictEqual({
      kind: 'paid',
      title: 'Про',
      priceLine: 'Вы платите 4\u00A0900 ₽ в год',
      subLine: 'Спишем 10 сентября',
    });
  });

  it('basic: free, no price line', () => {
    expect(tariffHero(subscription({
      tariff: { ...proTariff, name: 'basic', monthlyPriceKopecks: 0, yearlyPriceKopecks: 0 },
      validUntil: undefined,
      currentPeriod: undefined,
    }))).toStrictEqual({
      kind: 'basic',
      title: 'Базовый',
      subLine: 'Бесплатно',
    });
  });

  it('grace: pay-by date replaces the charge date, CTA switches to pay', () => {
    expect(tariffHero(subscription({
      status: 'grace',
      tariff: businessTariff,
      currentPeriod: 'year',
      validUntil: '2026-09-17T21:00:00Z',
    }))).toStrictEqual({
      kind: 'grace',
      title: 'Бизнес',
      priceLine: 'Вы платите 8\u00A0900 ₽ в год',
      subLine: 'Оплатите тариф до 17 сентября',
    });
  });

  it('cancelled: stopped title with the valid-until date', () => {
    expect(tariffHero(subscription({ status: 'cancelled' }))).toStrictEqual({
      kind: 'stopped',
      title: 'Про остановлен',
      dateLine: '10 сентября',
      subLine: 'Действует до',
    });
  });

  it('stopped without validUntil omits the date line', () => {
    const hero = tariffHero(subscription({ status: 'cancelled', validUntil: undefined }));
    expect(hero).toStrictEqual({
      kind: 'stopped',
      title: 'Про остановлен',
      dateLine: undefined,
      subLine: 'Действует до',
    });
  });
});

describe('formatPaymentCountdown', () => {
  const expiresAt = '2026-09-10T12:15:00Z';

  it('shows remaining minutes and seconds, zero-padded', () => {
    expect(formatPaymentCountdown(expiresAt, new Date('2026-09-10T11:59:01Z'))).toBe('15:59');
  });

  it('counts down to zero-padded seconds', () => {
    expect(formatPaymentCountdown(expiresAt, new Date('2026-09-10T12:13:30Z'))).toBe('01:30');
  });

  it('clamps at 00:00 once the form expired', () => {
    expect(formatPaymentCountdown(expiresAt, new Date('2026-09-10T12:15:00Z'))).toBe('00:00');
    expect(formatPaymentCountdown(expiresAt, new Date('2026-09-10T13:00:00Z'))).toBe('00:00');
  });
});

describe('pendingPaymentDescription', () => {
  it('names the tariff, the price and the period of the pending payment', () => {
    const pending: PendingPayment = {
      tariffName: 'pro',
      period: 'month',
      amountKopecks: 49000,
      confirmUrl: 'https://payment.tbank.ru/confirm/abc',
      expiresAt: '2026-09-10T12:15:00Z',
    };

    expect(pendingPaymentDescription(pending)).toBe(
      'Тариф Про за 490 ₽ в месяц. Вернитесь на страницу банка, чтобы завершить оплату',
    );
  });
});

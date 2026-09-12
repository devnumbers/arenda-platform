import { describe, expect, it } from 'vitest';
import type { SubscriptionPayment } from '@/entities/billing';
import {
  groupSubscriptionPaymentsByDate,
  paymentCardMask,
  paymentPeriodLabel,
  paymentRowAmountProps,
  paymentRowSubtitle,
  paymentStatusTone,
} from './payment-history-model';

const proTariff = {
  id: 'pro',
  name: 'pro' as const,
  monthlyPriceKopecks: 49000,
  yearlyPriceKopecks: 490000,
  activePropertyLimit: 5,
};

function payment(overrides: Partial<SubscriptionPayment>): SubscriptionPayment {
  return {
    id: 'p1',
    tariff: proTariff,
    period: 'month',
    amountKopecks: 49000,
    status: 'succeeded',
    provider: 'tkassa',
    paymentUrl: null,
    createdAt: '2026-08-10T10:56:00',
    ...overrides,
  };
}

describe('paymentRowAmountProps', () => {
  it('выполнена — тёмный минус (Figma 1877-68603)', () => {
    expect(paymentRowAmountProps(payment({ status: 'succeeded' }))).toStrictEqual({
      amountKopecks: -49000,
      signedAmount: true,
    });
  });

  it('не выполнено — красная только сумма, описание серое (макет 1877-68603)', () => {
    expect(paymentRowAmountProps(payment({ status: 'failed' }))).toStrictEqual({
      amountKopecks: -49000,
      signedAmount: true,
      valueClassName: 'text-danger',
    });
  });

  it('возврат — зелёный плюс', () => {
    expect(
      paymentRowAmountProps(payment({ status: 'refunded', amountKopecks: 440000 })),
    ).toStrictEqual({ amountKopecks: 440000, signedAmount: true, valueClassName: 'text-success' });
  });

  it('в ожидании — без знака (решение владельца, #614)', () => {
    expect(paymentRowAmountProps(payment({ status: 'pending' }))).toStrictEqual({
      amountKopecks: 49000,
      signedAmount: false,
    });
  });
});

describe('paymentRowSubtitle', () => {
  it('выполненная строка — без подзаголовка', () => {
    expect(paymentRowSubtitle(payment({ status: 'succeeded' }))).toBeUndefined();
  });

  it('статусные строки — подписью статуса', () => {
    expect(paymentRowSubtitle(payment({ status: 'failed' }))).toBe('Не выполнено');
    expect(paymentRowSubtitle(payment({ status: 'refunded' }))).toBe('Возврат');
    expect(paymentRowSubtitle(payment({ status: 'pending' }))).toBe('В ожидании');
  });
});

describe('paymentCardMask', () => {
  it('система карты от бэка + хвост маски: «Мир •• 0700»', () => {
    expect(
      paymentCardMask(
        payment({
          paymentMethod: { displayMask: '4300********0700', cardSystem: 'mir' },
        }),
      ),
    ).toBe('Мир •• 0700');
  });

  it('visa и mastercard — латиницей, unknown — без префикса', () => {
    expect(
      paymentCardMask(
        payment({
          paymentMethod: { displayMask: '4111********1111', cardSystem: 'visa' },
        }),
      ),
    ).toBe('Visa •• 1111');
    expect(
      paymentCardMask(
        payment({
          paymentMethod: { displayMask: '5500********4444', cardSystem: 'mastercard' },
        }),
      ),
    ).toBe('Mastercard •• 4444');
    expect(
      paymentCardMask(
        payment({
          paymentMethod: { displayMask: '2200********3333', cardSystem: 'unknown' },
        }),
      ),
    ).toBe('•• 3333');
  });

  it('платёж без карты — маски нет', () => {
    expect(paymentCardMask(payment({ paymentMethod: undefined }))).toBeUndefined();
  });
});

describe('paymentStatusTone', () => {
  it('цвет статуса в детали (Figma 1904-40495): ожидание синим', () => {
    expect(paymentStatusTone('pending')).toBe('text-primary');
    expect(paymentStatusTone('failed')).toBe('text-danger');
    expect(paymentStatusTone('refunded')).toBe('text-success');
    expect(paymentStatusTone('succeeded')).toBeUndefined();
  });
});

describe('paymentPeriodLabel', () => {
  it('копия макета детали: «В месяц» / «В год»', () => {
    expect(paymentPeriodLabel('month')).toBe('В месяц');
    expect(paymentPeriodLabel('year')).toBe('В год');
  });
});

describe('groupSubscriptionPaymentsByDate', () => {
  it('подряд идущие платежи одного локального дня — одна группа', () => {
    const groups = groupSubscriptionPaymentsByDate(
      [
        payment({ id: 'p1', createdAt: '2026-08-10T10:00:00' }),
        payment({ id: 'p2', createdAt: '2026-08-10T22:30:00' }),
      ],
      '2026-09-12',
    );
    expect(groups).toHaveLength(1);
    expect(groups[0]?.date).toBe('2026-08-10');
    expect(groups[0]?.label).toBe('10 августа');
    expect(groups[0]?.payments.map((p) => p.id)).toStrictEqual(['p1', 'p2']);
  });

  it('лейбл с годом вне текущего, порядок входа сохраняется', () => {
    const groups = groupSubscriptionPaymentsByDate(
      [
        payment({ id: 'new', createdAt: '2026-08-10T10:00:00' }),
        payment({ id: 'old', createdAt: '2025-12-08T10:00:00' }),
      ],
      '2026-09-12',
    );
    expect(groups.map((g) => g.label)).toStrictEqual(['10 августа', '8 декабря, 2025']);
    expect(groups.map((g) => g.date)).toStrictEqual(['2026-08-10', '2025-12-08']);
  });
});

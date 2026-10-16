import { describe, expect, it } from 'vitest';
import type { SubscriptionPayment } from '@/entities/billing';
import {
  groupSubscriptionPaymentsByDate,
  isPaymentFormExpired,
  paymentCardMask,
  paymentFormDeadline,
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
    expiresAt: null,
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
  it('маска без системы карты: «•••• 0700» (решение владельца 12.09, #624)', () => {
    expect(
      paymentCardMask(
        payment({
          paymentMethod: { displayMask: '4300********0700', cardSystem: 'mir' },
        }),
      ),
    ).toBe('•••• 0700');
  });

  it('хвост берётся из маски независимо от системы', () => {
    expect(
      paymentCardMask(
        payment({
          paymentMethod: { displayMask: '4111********1111', cardSystem: 'visa' },
        }),
      ),
    ).toBe('•••• 1111');
    expect(
      paymentCardMask(
        payment({
          paymentMethod: { displayMask: '5500********4444', cardSystem: 'unknown' },
        }),
      ),
    ).toBe('•••• 4444');
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

describe('paymentFormDeadline', () => {
  const deadline = '2026-09-15T12:15:00Z';

  it('у банковской pending — серверный expiresAt (#680)', () => {
    expect(
      paymentFormDeadline(
        payment({ status: 'pending', paymentUrl: 'https://pay.tbank.ru/x', expiresAt: deadline }),
      ),
    ).toBe(deadline);
  });

  it('у платежа без формы срока нет: не-banking статус, MIT-pending, legacy-строка', () => {
    expect(paymentFormDeadline(payment({ status: 'succeeded', expiresAt: deadline }))).toBeNull();
    expect(
      paymentFormDeadline(payment({ status: 'pending', expiresAt: deadline })),
    ).toBeNull();
    expect(
      paymentFormDeadline(payment({ status: 'pending', paymentUrl: 'https://pay.tbank.ru/x' })),
    ).toBeNull();
  });
});

describe('isPaymentFormExpired', () => {
  const deadline = '2026-09-15T12:15:00Z';
  const pending = payment({
    status: 'pending',
    paymentUrl: 'https://pay.tbank.ru/x',
    expiresAt: deadline,
  });

  it('после серверного дедлайна форма просрочена, до — жива (#680)', () => {
    expect(isPaymentFormExpired(pending, new Date('2026-09-15T12:16:00Z'))).toBe(true);
    expect(isPaymentFormExpired(pending, new Date('2026-09-15T12:14:00Z'))).toBe(false);
  });

  it('платёж без формы не бывает просроченным', () => {
    expect(
      isPaymentFormExpired(payment({ status: 'pending' }), new Date('2027-01-01T00:00:00Z')),
    ).toBe(false);
  });

  it('не-pending платёж с унаследованным expiresAt — не просрочен (единый предикат с paymentFormDeadline)', () => {
    expect(
      isPaymentFormExpired(
        payment({ status: 'succeeded', expiresAt: deadline }),
        new Date('2027-01-01T00:00:00Z'),
      ),
    ).toBe(false);
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

import { describe, expect, it } from 'vitest';
import { mapSubscriptionResponse } from './mappers';
import type { components } from '@/shared/api/dto';

type SubscriptionResponse = components['schemas']['Subscription'];

const paidPro: SubscriptionResponse = {
  tariff: {
    name: 'pro',
    activePropertyLimit: 5,
    monthlyPriceKopecks: 49000,
    yearlyPriceKopecks: 490000,
  },
  status: 'active',
  source: 'paid',
  validUntil: '2026-09-10T00:00:00Z',
  autoRenewEnabled: true,
  currentPeriod: 'month',
};

describe('mapSubscriptionResponse', () => {
  it('maps the subscription without a pending payment', () => {
    expect(mapSubscriptionResponse(paidPro).pendingPayment).toBeUndefined();
  });

  it('maps the live pending payment with the confirm url and expiry (#616)', () => {
    const response: SubscriptionResponse = {
      ...paidPro,
      pendingPayment: {
        tariffName: 'business',
        period: 'year',
        amountKopecks: 890000,
        confirmUrl: 'https://payment.tbank.ru/confirm/abc',
        expiresAt: '2026-09-10T12:15:00Z',
      },
    };

    expect(mapSubscriptionResponse(response).pendingPayment).toStrictEqual({
      tariffName: 'business',
      period: 'year',
      amountKopecks: 890000,
      confirmUrl: 'https://payment.tbank.ru/confirm/abc',
      expiresAt: '2026-09-10T12:15:00Z',
    });
  });
});

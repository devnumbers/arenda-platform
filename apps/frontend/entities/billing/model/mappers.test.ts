import { describe, expect, it } from 'vitest';
import {
  mapPaymentMethodResponse,
  mapSubscriptionPaymentResponse,
  mapSubscriptionResponse,
} from './mappers';
import type { components } from '@/shared/api/dto';

type SubscriptionResponse = components['schemas']['Subscription'];
type SubscriptionPaymentResponse = components['schemas']['SubscriptionPayment'];

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

describe('mapSubscriptionPaymentResponse', () => {
  const chargedPayment: SubscriptionPaymentResponse = {
    id: '0e6f6c6a-0000-4000-8000-000000000001',
    tariff: {
      name: 'pro',
      activePropertyLimit: 5,
      monthlyPriceKopecks: 49000,
      yearlyPriceKopecks: 490000,
    },
    period: 'month',
    amountKopecks: 49000,
    status: 'succeeded',
    provider: 'tkassa',
    createdAt: '2026-08-10T07:56:00Z',
    succeededAt: '2026-08-10T07:56:10Z',
    paymentMethod: { displayMask: '4300********0700', cardSystem: 'mir' },
  };

  it('переносит карту оплаты и момент успеха (#624, DTO #619)', () => {
    const payment = mapSubscriptionPaymentResponse(chargedPayment);
    expect(payment.paymentMethod).toStrictEqual({
      displayMask: '4300********0700',
      cardSystem: 'mir',
    });
    expect(payment.succeededAt).toBe('2026-08-10T07:56:10Z');
  });

  it('без карты (старый платёж) paymentMethod не определён', () => {
    const payment = mapSubscriptionPaymentResponse({
      ...chargedPayment,
      paymentMethod: undefined,
      succeededAt: null,
    });
    expect(payment.paymentMethod).toBeUndefined();
    expect(payment.succeededAt).toBeUndefined();
  });

  it('переносит срок жизни формы (#680)', () => {
    expect(
      mapSubscriptionPaymentResponse({
        ...chargedPayment,
        expiresAt: '2026-08-10T08:11:00Z',
      }).expiresAt,
    ).toBe('2026-08-10T08:11:00Z');
  });

  it('у платежа без формы (MIT-списание) expiresAt null', () => {
    expect(mapSubscriptionPaymentResponse(chargedPayment).expiresAt).toBeNull();
  });

  it('служебный refunding показывается как pending (решение владельца, #614)', () => {
    const payment = mapSubscriptionPaymentResponse({
      ...chargedPayment,
      status: 'refunding',
    });
    expect(payment.status).toBe('pending');
  });
});

describe('mapPaymentMethodResponse', () => {
  const method: components['schemas']['PaymentMethod'] = {
    id: '1b58a3c8-0000-4000-8000-000000000001',
    provider: 'tkassa',
    displayMask: '2200********0700',
    cardSystem: 'mir',
    expDate: '0927',
    isActive: true,
    createdAt: '2026-09-01T10:00:00Z',
  };

  it('переносит систему и срок карты (#625, контракт #614)', () => {
    const mapped = mapPaymentMethodResponse(method);
    expect(mapped.cardSystem).toBe('mir');
    expect(mapped.expDate).toBe('0927');
  });

  it('без срока карты expDate не определён', () => {
    expect(
      mapPaymentMethodResponse({ ...method, expDate: null }).expDate,
    ).toBeUndefined();
  });
});

import { describe, expect, it } from 'vitest';
import type { PaymentMethod } from '@/entities/billing';
import {
  paymentMethodDeleteGuard,
  paymentMethodTitle,
} from './payment-methods';

function method(overrides: Partial<PaymentMethod> = {}): PaymentMethod {
  return {
    id: '1b58a3c8-0000-4000-8000-000000000001',
    displayMask: '2200********0700',
    cardSystem: 'mir',
    provider: 'tkassa',
    isActive: false,
    createdAt: '2026-09-01T10:00:00Z',
    ...overrides,
  };
}

describe('paymentMethodTitle', () => {
  it('название системы и хвост маски — «Мир •••• 0700» (макет 1879-71079)', () => {
    expect(paymentMethodTitle(method())).toBe('Мир •••• 0700');
  });

  it('Visa и Mastercard — латиницей', () => {
    expect(paymentMethodTitle(method({ cardSystem: 'visa' }))).toBe('Visa •••• 0700');
    expect(paymentMethodTitle(method({ cardSystem: 'mastercard' }))).toBe(
      'Mastercard •••• 0700',
    );
  });

  it('без распознанной системы — только хвост (как в «Операциях», #624)', () => {
    expect(paymentMethodTitle(method({ cardSystem: 'unknown' }))).toBe('•••• 0700');
  });

  it('маска короче 4 цифр — как есть', () => {
    expect(paymentMethodTitle(method({ displayMask: '41' }))).toBe('Мир 41');
    expect(paymentMethodTitle(method({ displayMask: '4111' }))).toBe('Мир •••• 4111');
  });
});

describe('paymentMethodDeleteGuard', () => {
  it('неактивная карта удаляется через подтверждение — без гарда', () => {
    expect(paymentMethodDeleteGuard(method({ isActive: false }), 2)).toBeUndefined();
  });

  it('активная из нескольких — гард «нельзя удалить активный» (1936-114196)', () => {
    expect(
      paymentMethodDeleteGuard(method({ isActive: true }), 2),
    ).toBe('active');
  });

  it('единственная (всегда активная) — гард «нельзя удалить единственный» (1936-113689)', () => {
    expect(
      paymentMethodDeleteGuard(method({ isActive: true }), 1),
    ).toBe('only');
  });
});

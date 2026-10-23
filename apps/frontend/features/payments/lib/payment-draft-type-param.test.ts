import { describe, expect, it } from 'vitest';
import { parsePaymentDraftTypeParam } from './payment-draft-type-param';

describe('parsePaymentDraftTypeParam — тип создания из query (?type=)', () => {
  it('валидный тип проходит', () => {
    expect(parsePaymentDraftTypeParam('payment')).toBe('payment');
    expect(parsePaymentDraftTypeParam('autopayment')).toBe('autopayment');
  });

  it('отсутствующее, массивное и неизвестное значение — undefined', () => {
    expect(parsePaymentDraftTypeParam(undefined)).toBeUndefined();
    expect(parsePaymentDraftTypeParam(['payment'])).toBeUndefined();
    expect(parsePaymentDraftTypeParam('rental')).toBeUndefined();
    expect(parsePaymentDraftTypeParam('')).toBeUndefined();
  });
});

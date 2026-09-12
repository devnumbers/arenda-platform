import { describe, expect, it } from 'vitest';

import { overduePaymentIdsOf } from './overdue-payment-ids';

describe('overduePaymentIdsOf (платежи с накопленной просрочкой)', () => {
  it('собирает id платежей, операции без платежа пропускает', () => {
    expect(
      overduePaymentIdsOf([{ paymentId: 'p2' }, { paymentId: null }, { paymentId: 'p1' }]),
    ).toEqual(new Set(['p1', 'p2']));
  });

  it('пустой список — пустое множество', () => {
    expect(overduePaymentIdsOf([])).toEqual(new Set());
  });
});

import { describe, expect, it } from 'vitest';
import { paymentDetailActions } from './payment-detail-actions-model';

describe('paymentDetailActions — состав круглых действий (#818)', () => {
  it('обычное правило: пауза, правка, оплата', () => {
    expect(paymentDetailActions({ isRentalManaged: false }, false, false))
      .toStrictEqual(['pause', 'edit', 'pay']);
  });

  it('на паузе: возобновление вместо паузы — серверный isCompleted не решает (у бессрочной паузы он true, багфикс «Домофона» полного e2e)', () => {
    expect(paymentDetailActions({ isRentalManaged: false }, true, false))
      .toStrictEqual(['resume', 'edit', 'pay']);
  });

  it('завершённое правило (клиентский предикат, история 44 спеки #453): паузы нет', () => {
    expect(paymentDetailActions({ isRentalManaged: false }, false, true))
      .toStrictEqual(['edit', 'pay']);
  });

  it('управляемый платёж — только «Оплатить»: пауза и правка дают 409, канон отметки оплаты остаётся', () => {
    expect(paymentDetailActions({ isRentalManaged: true }, false, false))
      .toStrictEqual(['pay']);
    expect(paymentDetailActions({ isRentalManaged: true }, true, true))
      .toStrictEqual(['pay']);
  });
});

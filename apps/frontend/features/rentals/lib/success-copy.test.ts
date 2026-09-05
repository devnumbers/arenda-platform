import { describe, expect, it } from 'vitest';
import { rentalSuccessCopy } from './success-copy';

describe('rentalSuccessCopy', () => {
  it('заголовок канонический, описание собирается из условий', () => {
    expect(
      rentalSuccessCopy({ paymentDay: 10, amountKopecks: 5600000, autoPay: false }),
    ).toStrictEqual({
      heading: 'Вы создали аренду',
      // Группировка разрядов — NBSP из toLocaleString('ru-RU').
      description: 'Каждое 10 число месяца 56\u00A0000 ₽, отмечается вручную',
    });
  });

  it('день оплаты «последний день месяца» и автоплатёж читаются в описании', () => {
    expect(
      rentalSuccessCopy({ paymentDay: 'last', amountKopecks: 250000, autoPay: true }),
    ).toStrictEqual({
      heading: 'Вы создали аренду',
      description: 'Каждый последний день месяца 2\u00A0500 ₽, фиксируется автоматически',
    });
  });
});

import { describe, expect, it } from 'vitest';
import { rentalCompletedTitle, rentalExtendSuccessCopy, rentalSuccessCopy } from './success-copy';

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

describe('rentalExtendSuccessCopy', () => {
  it('попап успеха по макету: «еще на N месяцев до ДД.ММ.ГГГГ»', () => {
    // Кейс фрейма 1550:93723 — «+8 месяцев» к 10.06.2028.
    expect(rentalExtendSuccessCopy({ previousEnd: '2028-06-10', newEnd: '2029-02-10' })).toBe(
      'Аренда продлена еще на 8 месяцев до 10.02.2029',
    );
  });

  it('число месяцев склоняется: 21 месяц, 2 месяца, 1 месяц', () => {
    expect(rentalExtendSuccessCopy({ previousEnd: '2026-01-10', newEnd: '2027-10-10' })).toBe(
      'Аренда продлена еще на 21 месяц до 10.10.2027',
    );
    expect(rentalExtendSuccessCopy({ previousEnd: '2026-01-10', newEnd: '2026-03-10' })).toBe(
      'Аренда продлена еще на 2 месяца до 10.03.2026',
    );
    expect(rentalExtendSuccessCopy({ previousEnd: '2026-01-10', newEnd: '2026-02-10' })).toBe(
      'Аренда продлена еще на 1 месяц до 10.02.2026',
    );
  });

  it('продление короче полного месяца — дата без счётчика месяцев', () => {
    expect(rentalExtendSuccessCopy({ previousEnd: '2026-02-10', newEnd: '2026-03-05' })).toBe(
      'Аренда продлена до 05.03.2026',
    );
  });
});

describe('rentalCompletedTitle', () => {
  it('подставляет имя объекта в кавычки-ёлочки', () => {
    expect(rentalCompletedTitle('Моя квартира')).toBe('Аренда объекта «Моя квартира» завершена');
  });
});

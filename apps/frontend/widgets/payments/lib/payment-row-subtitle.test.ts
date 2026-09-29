import { describe, expect, it } from 'vitest';
import { makePayment } from '@/entities/payment';
import { paymentRowSubtitle } from './payment-row-subtitle';

describe('paymentRowSubtitle', () => {
  it('у активной бессрочной платежа — дата следующего вхождения', () => {
    const result = paymentRowSubtitle(makePayment(), '2026-08-27');
    expect(result).toStrictEqual({ kind: 'date', iso: '2026-09-01' });
  });

  it('у платежа на активной бессрочной паузе — «пауза»', () => {
    const result = paymentRowSubtitle(
      makePayment({ pauses: [{ from: '2026-08-01' }] }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'paused' });
  });

  it('день возобновления уже не в паузе — снова дата', () => {
    const result = paymentRowSubtitle(
      makePayment({ pauses: [{ from: '2026-08-01', to: '2026-08-27' }] }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'date', iso: '2026-09-01' });
  });

  it('у завершённого платежа (endDate в прошлом) — нет подзаголовка', () => {
    const result = paymentRowSubtitle(
      makePayment({ endDate: '2026-08-01' }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'none' });
  });

  it('серверный флаг isCompleted гасит календарную дату — правила уже нет', () => {
    // Сценарий бага короткого правила: endDate ещё впереди (01.09), но все
    // вхождения оплачены — флаг важнее календарной проекции.
    const result = paymentRowSubtitle(
      makePayment({ endDate: '2026-09-01', isCompleted: true }),
      '2026-08-31',
    );
    expect(result).toStrictEqual({ kind: 'none' });
  });
});

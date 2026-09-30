import { describe, expect, it } from 'vitest';
import { makePayment } from '@/entities/payment';
import { paymentRowSubtitle } from './payment-row-subtitle';

describe('paymentRowSubtitle', () => {
  it('дата — серверный nearestDate, не клиентская проекция правила', () => {
    // Сценарий #967: ближайшее вхождение (1.09) оплачено вперёд, тик
    // переставил плановую на 15-е; проекция от «сегодня» дала бы 1.09.
    const result = paymentRowSubtitle(
      makePayment({ nearestDate: '2026-09-15' }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'date', iso: '2026-09-15' });
  });

  it('null у сервера (открытая пауза/завершённый) — без даты', () => {
    const result = paymentRowSubtitle(makePayment({ nearestDate: null }), '2026-08-27');
    expect(result).toStrictEqual({ kind: 'none' });
  });

  it('у платежа на активной бессрочной паузе — «пауза»', () => {
    const result = paymentRowSubtitle(
      makePayment({ pauses: [{ from: '2026-08-01' }], nearestDate: null }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'paused' });
  });

  it('день возобновления уже не в паузе — снова дата', () => {
    const result = paymentRowSubtitle(
      makePayment({ pauses: [{ from: '2026-08-01', to: '2026-08-27' }], nearestDate: '2026-09-01' }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'date', iso: '2026-09-01' });
  });

  it('ограниченная пауза с сегодня внутри — «пауза», серверная дата не показывается', () => {
    // Правило возобновится 1.09 и сервер несёт эту дату, но правило сейчас
    // на паузе — подпись о состоянии, не о графике (как сейчас).
    const result = paymentRowSubtitle(
      makePayment({ pauses: [{ from: '2026-08-01', to: '2026-09-01' }], nearestDate: '2026-09-01' }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'paused' });
  });

  it('у завершённого платежа (endDate в прошлом, сервер дал null) — нет подзаголовка', () => {
    const result = paymentRowSubtitle(
      makePayment({ endDate: '2026-08-01', isCompleted: true, nearestDate: null }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'none' });
  });
});

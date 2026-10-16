import { describe, expect, it } from 'vitest';
import { makePayment } from '@/entities/payment';
import { sortPaymentsByNextOccurrence } from './sort-payments-by-next-occurrence';

describe('sortPaymentsByNextOccurrence', () => {
  it('порядок — по серверному nearestDate, не по проекции правила', () => {
    // Сценарий #967: monthly-1 оплатили вперёд — сервер переставил его
    // ближайшую на 20-е, позади weekly-29; проекция от «сегодня» держала бы
    // его первым по голому графику.
    const prepaid = makePayment({ id: 'prepaid', nearestDate: '2026-09-20' });
    const weekly = makePayment({ id: 'weekly', nearestDate: '2026-09-29' });
    const monthly1 = makePayment({ id: 'monthly-1', nearestDate: '2026-10-01' });

    const sorted = sortPaymentsByNextOccurrence([prepaid, monthly1, weekly], '2026-08-27');

    expect(sorted.map((p) => p.id)).toStrictEqual(['prepaid', 'weekly', 'monthly-1']);
  });

  it('паузные (проверка по сегодня) и правила без даты уходят в конец', () => {
    const finished = makePayment({ id: 'finished', isCompleted: true, nearestDate: null });
    const paused = makePayment({ id: 'paused', pauses: [{ from: '2026-08-01' }], nearestDate: null });
    const boundedPaused = makePayment({
      id: 'bounded-paused',
      pauses: [{ from: '2026-08-01', to: '2026-09-01' }],
      nearestDate: '2026-09-01',
    });
    const active = makePayment({ id: 'active', nearestDate: '2026-09-01' });

    const sorted = sortPaymentsByNextOccurrence(
      [finished, boundedPaused, paused, active],
      '2026-08-27',
    );

    // Ограниченная пауза с сегодня внутри показывается «На паузе» — в конец,
    // как и раньше; её серверная дата ключом не становится. Null-ключи
    // стабильны — сохраняют входной порядок (finished, bounded-paused, paused).
    expect(sorted.map((p) => p.id)).toStrictEqual(['active', 'finished', 'bounded-paused', 'paused']);
  });

  it('при равной дате сохраняется серверный порядок (устойчивость)', () => {
    const first = makePayment({ id: 'first', nearestDate: '2026-09-01' });
    const second = makePayment({ id: 'second', nearestDate: '2026-09-01' });

    const sorted = sortPaymentsByNextOccurrence([first, second], '2026-08-27');

    expect(sorted.map((p) => p.id)).toStrictEqual(['first', 'second']);
  });
});

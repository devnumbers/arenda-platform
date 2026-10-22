import { describe, expect, it } from 'vitest';

import { makePayment, type Payment } from '@/entities/payment';

import { propertyPaymentGroups } from './payments-strip';

/** Полоса объекта живёт на правиле «Интернет», 10-е число; остальное —
 * канон makePayment. */
function paymentFixture(overrides: Partial<Payment> = {}): Payment {
  return makePayment({
    id: 'payment-1',
    propertyId: 'property-1',
    title: 'Интернет',
    amountKopecks: 50000,
    recurrence: { kind: 'monthly', daysOfMonth: [10], lastDay: false },
    since: '2026-01-10',
    category: { source: 'default', slug: 'internet', label: 'Интернет' },
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-01-10T00:00:00Z',
    ...overrides,
  });
}

describe('propertyPaymentGroups', () => {
  it('обе группы — максимум 3 платежа в каждой (решение владельца 22.10)', () => {
    // В полуколонке телефона 4-й круг не влезает в один ряд:
    // невлезающие не показываются — максимум 3, один ряд на ширинах ≥ 360.
    const payments = [
      paymentFixture({ id: 'a1', autoPay: true }),
      paymentFixture({ id: 'a2', autoPay: true }),
      paymentFixture({ id: 'a3', autoPay: true }),
      paymentFixture({ id: 'a4', autoPay: true }),
      paymentFixture({ id: 'a5', autoPay: true }),
      paymentFixture({ id: 'p1' }),
      paymentFixture({ id: 'p2' }),
      paymentFixture({ id: 'p3' }),
      paymentFixture({ id: 'p4' }),
      paymentFixture({ id: 'p5' }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(), '2026-10-22');
    expect(groups.map((group) => group.label)).toEqual(['Автоплатежи', 'Платежи']);
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual(['a1', 'a2', 'a3']);
    expect(groups[1]?.items.map((item) => item.payment.id)).toEqual(['p1', 'p2', 'p3']);
  });

  it('одна группа — максимум 7 платежей', () => {
    const payments = Array.from({ length: 9 }, (_, index) =>
      paymentFixture({ id: `p${index}`, autoPay: false }),
    );
    const groups = propertyPaymentGroups(payments, new Set(), '2026-09-11');
    expect(groups).toHaveLength(1);
    expect(groups[0]?.items).toHaveLength(7);
  });

  it('внутри группы просрочки первыми, затем по ближайшему вхождению', () => {
    // Ключ порядка — серверный nearestDate «Следующей даты оплаты» (#993):
    // у просроченных (25-е/28-е) следующее вхождение позади 15-го и 20-го,
    // вперёд их выносит только правило просрочек-первыми.
    const payments = [
      paymentFixture({ id: 'next-month', nearestDate: '2026-09-20' }),
      paymentFixture({ id: 'plain', nearestDate: '2026-09-15' }),
      paymentFixture({ id: 'overdue-1', nearestDate: '2026-09-25' }),
      paymentFixture({ id: 'overdue-2', nearestDate: '2026-09-28' }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(['overdue-1', 'overdue-2']), '2026-09-11');
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual([
      'overdue-1',
      'overdue-2',
      'plain',
      'next-month',
    ]);
    expect(groups[0]?.items.map((item) => item.overdue)).toEqual([true, true, false, false]);
  });

  it('точка просрочки — по идентификаторам платёж с накопленной просрочкой', () => {
    const payments = [
      paymentFixture({ id: 'p1', autoPay: false }),
      paymentFixture({ id: 'p2', autoPay: false }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(['p2']), '2026-09-11');
    // p2 с просрочкой выходит вперёд
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual(['p2', 'p1']);
    expect(groups[0]?.items.map((item) => item.overdue)).toEqual([true, false]);
  });

  it('паузные правила выводятся как обычные — в конце группы (решение владельца 12.10, #1353)', () => {
    // Форма данных как у бекенда: у паузных nearestDate = null.
    const payments = [
      paymentFixture({
        id: 'paused-early',
        pauses: [{ from: '2026-09-01' }],
      }),
      paymentFixture({ id: 'dated', nearestDate: '2026-09-15' }),
      paymentFixture({
        id: 'paused-late',
        pauses: [{ from: '2026-09-01' }],
      }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(), '2026-09-11');
    expect(groups).toHaveLength(1);
    // Паузный круг ничем не отличается: без точки просрочки, в конце
    // группы (дат у паузных нет — сортировка кладёт их за датированными).
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual([
      'dated',
      'paused-early',
      'paused-late',
    ]);
    expect(groups[0]?.items.map((item) => item.overdue)).toEqual([false, false, false]);
  });

  it('объект только с паузными — группа выводится, полоса без пустых групп (#1353)', () => {
    const groups = propertyPaymentGroups(
      [paymentFixture({ id: 'paused', pauses: [{ from: '2026-09-01' }] })],
      new Set(),
      '2026-09-11',
    );
    expect(groups).toHaveLength(1);
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual(['paused']);
  });

  it('паузные занимают слоты лимита как все: поверх лимита не проходят', () => {
    const dated = Array.from({ length: 7 }, (_, index) =>
      paymentFixture({ id: `d${index}`, nearestDate: `2026-09-${String(index + 10)}` }),
    );
    const paused = paymentFixture({ id: 'paused', pauses: [{ from: '2026-09-01' }] });
    const groups = propertyPaymentGroups([...dated, paused], new Set(), '2026-09-11');
    expect(groups).toHaveLength(1);
    expect(groups[0]?.items).toHaveLength(7);
    expect(groups[0]?.items.map((item) => item.payment.id)).not.toContain('paused');
  });

  it('пустая группа не выводится: только автоплатежи — одна группа', () => {
    const payments = [paymentFixture({ id: 'p1', autoPay: true })];
    const groups = propertyPaymentGroups(payments, new Set(), '2026-09-11');
    expect(groups).toHaveLength(1);
    expect(groups[0]?.label).toBe('Автоплатежи');
  });
});

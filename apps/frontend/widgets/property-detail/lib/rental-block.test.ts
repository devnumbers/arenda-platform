import { describe, expect, it } from 'vitest';

import { makeRental } from '@/entities/rental';

import { buildPropertyRentalBlock } from './rental-block';

describe('buildPropertyRentalBlock — активная аренда', () => {
  it('заголовок «Оплачено N из M платежей» и синяя строка дней', () => {
    const block = buildPropertyRentalBlock(makeRental());
    expect(block.paidTitle).toBe('Оплачено 6 из 24 платежей');
    expect(block.paymentLine).toBe('150 дней до следующего платежа');
    expect(block.footerLine).toBe('Осталось 23 месяца аренды');
    expect(block.endOfTerm).toBe(false);
  });

  it('первый платёж ещё не оплачен — «N дней до платежа» (макет 1425:55908)', () => {
    const block = buildPropertyRentalBlock(
      makeRental({
        progress: { paidMonths: 0, totalMonths: 24, monthsRemaining: 24, overdueMonths: null },
        rentPayment: {
          paymentId: 'payment-1',
          amountKopecks: 5600000,
          paymentDay: 1,
          autoPay: false,
          reminderOffsetDays: null,
          nextPayment: {
            operationId: 'operation-1',
            date: '2026-10-11',
            amountKopecks: 5600000,
            daysUntil: 30,
          },
        },
      }),
    );
    expect(block.paidTitle).toBe('Оплачено 0 из 24 платежей');
    expect(block.paymentLine).toBe('30 дней до платежа');
  });

  it('день платежа настал — «Платёж сегодня»', () => {
    const block = buildPropertyRentalBlock(
      makeRental({
        rentPayment: {
          paymentId: 'payment-1',
          amountKopecks: 5600000,
          paymentDay: 1,
          autoPay: false,
          reminderOffsetDays: null,
          nextPayment: {
            operationId: 'operation-1',
            date: '2026-09-11',
            amountKopecks: 5600000,
            daysUntil: 0,
          },
        },
      }),
    );
    expect(block.paymentLine).toBe('Платёж сегодня');
  });

  it('согласование дней: «1 день», «2 дня», «21 день»', () => {
    const line = (daysUntil: number): string =>
      buildPropertyRentalBlock(
        makeRental({
          progress: { paidMonths: 0, totalMonths: 24, monthsRemaining: 24, overdueMonths: null },
          rentPayment: {
            paymentId: 'payment-1',
            amountKopecks: 5600000,
            paymentDay: 1,
            autoPay: false,
            reminderOffsetDays: null,
            nextPayment: {
              operationId: 'operation-1',
              date: '2026-09-12',
              amountKopecks: 5600000,
              daysUntil,
            },
          },
        }),
      ).paymentLine ?? '';
    expect(line(1)).toBe('1 день до платежа');
    expect(line(2)).toBe('2 дня до платежа');
    expect(line(21)).toBe('21 день до платежа');
  });

  it('родительный падеж после «из»: «из 21 платежа»', () => {
    const block = buildPropertyRentalBlock(
      makeRental({
        progress: { paidMonths: 3, totalMonths: 21, monthsRemaining: 18, overdueMonths: null },
      }),
    );
    expect(block.paidTitle).toBe('Оплачено 3 из 21 платежа');
  });

  it('процент прогресса — доля оплаченных', () => {
    const block = buildPropertyRentalBlock(makeRental());
    expect(block.percent).toBe(25);
  });
});

describe('buildPropertyRentalBlock — срок подошёл к концу', () => {
  it('needs_attention: синяя строка «Последний платеж оплачен», футер «Срок аренды подошел к концу»', () => {
    const block = buildPropertyRentalBlock(
      makeRental({
        status: 'needs_attention',
        progress: { paidMonths: 24, totalMonths: 24, monthsRemaining: 0, overdueMonths: null },
      }),
    );
    expect(block.endOfTerm).toBe(true);
    expect(block.paymentLine).toBe('Последний платеж оплачен');
    expect(block.footerLine).toBe('Срок аренды подошел к концу');
    expect(block.percent).toBe(100);
  });
});

describe('buildPropertyRentalBlock — бессрочная аренда', () => {
  it('без тотала: «Оплачено N платежей», бара нет, футер «Прошло N месяцев»', () => {
    const block = buildPropertyRentalBlock(
      makeRental({
        plannedEndDate: null,
        progress: { paidMonths: 3, totalMonths: null, monthsRemaining: null, overdueMonths: null },
      }),
    );
    expect(block.paidTitle).toBe('Оплачено 3 платежа');
    expect(block.percent).toBeNull();
    // Полных месяцев от начала (2026-04-01 → 2026-09-11), не «оплачено».
    expect(block.footerLine).toBe('Прошло 5 месяцев');
  });

  it('согласование: «Оплачено 1 платёж», «Оплачено 21 платёж»', () => {
    const title = (paidMonths: number): string =>
      buildPropertyRentalBlock(
        makeRental({
          plannedEndDate: null,
          progress: { paidMonths, totalMonths: null, monthsRemaining: null, overdueMonths: null },
        }),
      ).paidTitle;
    expect(title(1)).toBe('Оплачено 1 платёж');
    expect(title(21)).toBe('Оплачено 21 платёж');
    expect(title(6)).toBe('Оплачено 6 платежей');
  });
});

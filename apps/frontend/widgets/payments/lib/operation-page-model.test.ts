import { describe, expect, it } from 'vitest';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import type { PaymentOperation } from '@/entities/payment';
import {
  operationDelayRow,
  operationHeroAmount,
  operationSubtitle,
} from './operation-page-model';

/**
 * Модель страницы операции (Figma 1386:67731 / 1419:25859 / 1419:25645 /
 * 1444:66228): подпись под суммой, строка задержки, знаковая сумма и
 * правило «которую операцию можно оплатить».
 */

function operation(overrides: Partial<PaymentOperation> = {}): PaymentOperation {
  return {
    id: 'op-1',
    propertyId: 'prop-1',
    paymentId: 'pay-1',
    date: '2026-11-10',
    status: 'planned',
    type: 'income',
    title: 'Арендная плата',
    amountKopecks: 5_600_000,
    categoryLabel: 'Арендная плата',
    categorySlug: 'rent',
    ...overrides,
  };
}

describe('operationSubtitle', () => {
  it('плановая — «N дней до оплаты» синим', () => {
    expect(operationSubtitle(operation(), '2026-11-05')).toEqual({
      text: '5 дней до оплаты',
      tone: 'primary',
    });
    expect(operationSubtitle(operation(), '2026-11-09')?.text).toBe('1 день до оплаты');
  });

  it('плановая сегодня — «сегодня»', () => {
    expect(operationSubtitle(operation(), '2026-11-10')).toEqual({
      text: 'сегодня',
      tone: 'primary',
    });
  });

  it('просроченная — «просрочена на N дней» красным', () => {
    expect(operationSubtitle(operation({ status: 'overdue' }), '2026-11-11')).toEqual({
      text: 'просрочена на 1 день',
      tone: 'danger',
    });
    expect(operationSubtitle(operation({ status: 'overdue' }), '2026-11-13')?.text).toBe(
      'просрочена на 3 дня',
    );
  });

  it('оплаченная — подписи под суммой нет', () => {
    expect(
      operationSubtitle(operation({ status: 'paid', paidDate: '2026-11-13' }), '2026-11-13'),
    ).toBeNull();
  });
});

describe('operationDelayRow', () => {
  it('просроченная — «Задержана на» от сегодняшнего дня', () => {
    expect(operationDelayRow(operation({ status: 'overdue' }), '2026-11-13')).toEqual({
      label: 'Задержана на',
      text: '3 дня',
    });
  });

  it('оплачена позже срока — «Задержана на»', () => {
    expect(operationDelayRow(operation({ status: 'paid', paidDate: '2026-11-13' }), '2026-11-13')).toEqual({
      label: 'Задержана на',
      text: '3 дня',
    });
  });

  it('оплачена раньше срока — «Заранее на»', () => {
    expect(operationDelayRow(operation({ status: 'paid', paidDate: '2026-11-08' }), '2026-11-13')).toEqual({
      label: 'Заранее на',
      text: '2 дня',
    });
  });

  it('оплачена в срок и плановая — строки нет', () => {
    expect(operationDelayRow(operation({ status: 'paid', paidDate: '2026-11-10' }), '2026-11-13')).toBeNull();
    expect(operationDelayRow(operation(), '2026-11-05')).toBeNull();
  });
});

describe('operationHeroAmount', () => {
  const money = formatMoneyKopecks(5_600_000);

  it('оплаченный доход — зелёная сумма с плюсом', () => {
    expect(operationHeroAmount(operation({ status: 'paid', paidDate: '2026-11-13' }))).toEqual({
      text: `+${money}`,
      tone: 'success',
    });
  });

  it('расход — сумма с минусом (оплаченная — тёмная)', () => {
    const expense = operation({ type: 'expense', status: 'paid', paidDate: '2026-11-13' });
    expect(operationHeroAmount(expense)).toEqual({ text: `−${money}`, tone: 'default' });
    expect(operationHeroAmount(operation({ type: 'expense' })).text).toBe(`−${money}`);
  });

  it('плановый доход — тёмная сумма без знака', () => {
    expect(operationHeroAmount(operation())).toEqual({ text: money, tone: 'default' });
  });

  it('просроченная — красная', () => {
    expect(operationHeroAmount(operation({ status: 'overdue' })).tone).toBe('danger');
  });
});

import { describe, expect, it } from 'vitest';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import type { PaymentOperation } from '@/entities/payment';
import {
  operationDelayRow,
  operationDetailRows,
  operationHeroAmount,
  operationSubtitle,
  paidSuccessReturnPath,
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

describe('operationDetailRows', () => {
  it('manual-операция (paymentId null) — «Дата операции» + «Статус: Выполнена» (1858-105181)', () => {
    const manual = operation({
      paymentId: null,
      status: 'paid',
      paidDate: '2026-11-10',
    });
    expect(operationDetailRows(manual, '2026-11-10')).toEqual([
      { label: 'Дата операции', text: '10 ноября' },
      { label: 'Статус', text: 'Выполнена' },
    ]);
  });

  it('manual-операция прошлого года — дата с годом; оплата и задержка не показываются', () => {
    const manual = operation({
      paymentId: null,
      status: 'paid',
      date: '2025-11-10',
      paidDate: '2025-11-13',
    });
    expect(operationDetailRows(manual, '2026-11-10')).toEqual([
      { label: 'Дата операции', text: '10 ноября, 2025' },
      { label: 'Статус', text: 'Выполнена' },
    ]);
  });

  it('manual-операция с удалённого правила — просроченный статус красным', () => {
    const orphan = operation({ paymentId: null, status: 'overdue' });
    expect(operationDetailRows(orphan, '2026-11-13')).toEqual([
      { label: 'Дата операции', text: '10 ноября' },
      { label: 'Статус', text: 'Просрочена', danger: true },
    ]);
  });

  it('операция правила — плановая/фактическая/задержка/статус как раньше', () => {
    const late = operation({ status: 'paid', paidDate: '2026-11-13' });
    expect(operationDetailRows(late, '2026-11-13')).toEqual([
      { label: 'Фактическая оплата', text: '13 ноября' },
      { label: 'Плановая оплата', text: '10 ноября' },
      { label: 'Задержана на', text: '3 дня' },
      { label: 'Статус', text: 'Выполнена' },
    ]);
  });

  it('операция правила в срок — без строки задержки; плановая — без фактической', () => {
    const ontime = operation({ status: 'paid', paidDate: '2026-11-10' });
    expect(operationDetailRows(ontime, '2026-11-13')).toEqual([
      { label: 'Фактическая оплата', text: '10 ноября' },
      { label: 'Плановая оплата', text: '10 ноября' },
      { label: 'Статус', text: 'Выполнена' },
    ]);
    expect(operationDetailRows(operation(), '2026-11-05')).toEqual([
      { label: 'Плановая оплата', text: '10 ноября' },
      { label: 'Статус', text: 'Запланирована' },
    ]);
  });

  it('просроченная правила — задержка и статус красным', () => {
    expect(operationDetailRows(operation({ status: 'overdue' }), '2026-11-13')).toEqual([
      { label: 'Плановая оплата', text: '10 ноября' },
      { label: 'Задержана на', text: '3 дня' },
      { label: 'Статус', text: 'Просрочена', danger: true },
    ]);
  });
});

describe('paidSuccessReturnPath', () => {
  it('returnTo страницы, с которой перешли к оплате, имеет приоритет (#1072)', () => {
    expect(paidSuccessReturnPath('/properties/prop-1/rentals', 'prop-1', 'pay-1')).toBe(
      '/properties/prop-1/rentals',
    );
  });

  it('без returnTo — страница правила-платежа, контекст операции', () => {
    expect(paidSuccessReturnPath(undefined, 'prop-1', 'pay-1')).toBe(
      ROUTES.propertyPayment('prop-1', 'pay-1'),
    );
  });

  it('без правила функция тотальна — список операций объекта (у manual success не бывает)', () => {
    expect(paidSuccessReturnPath(undefined, 'prop-1', null)).toBe(
      ROUTES.propertyPayments('prop-1'),
    );
  });
});

import { describe, expect, it } from 'vitest';
import { categoryStyle } from '@/features/payment-categories';
import type { GlobalPayment, GlobalPaymentObject } from '@/entities/payment';
import { makeGlobalPayment } from './global-payment-fixtures';
import { paymentObjectStacks } from './payments-objects-model';

/** Фабрика объекта «Объектов» (#582): канонический базовый экземпляр. */
function makeObject(
  overrides: Partial<GlobalPaymentObject> = {},
): GlobalPaymentObject {
  return {
    propertyId: 'property-1',
    name: 'Моя квартира',
    address: 'Новатаров, 8',
    pinnedAt: null,
    photoUrl: null,
    autoPayRules: [],
    otherRules: [],
    ...overrides,
  };
}

/** Ключ стопки: id + флаг просрочки. */
function key(paymentId: string, hasOverdue = false) {
  return { paymentId, hasOverdue };
}

/** Карта фида по id: n правил с одинаковым свойством. */
function feedByCount(count: number, overrides: Partial<GlobalPayment> = {}) {
  const map = new Map<string, GlobalPayment>();
  for (let i = 1; i <= count; i += 1) {
    const payment = makeGlobalPayment({ id: `p-${i}`, ...overrides });
    map.set(payment.id, payment);
  }
  return map;
}

describe('paymentObjectStacks — состав групп', () => {
  it('обе группы: «Автоплатежи» первая, «Платежи» вторая', () => {
    const object = makeObject({
      autoPayRules: [key('p-1')],
      otherRules: [key('p-2')],
    });

    const stacks = paymentObjectStacks(object, feedByCount(2));

    expect(stacks.map((stack) => stack.label)).toEqual([
      'Автоплатежи',
      'Платежи',
    ]);
  });

  it('пустые группы скрыты — только непустая «Платежи»', () => {
    const object = makeObject({ otherRules: [key('p-1')] });

    const stacks = paymentObjectStacks(object, feedByCount(1));

    expect(stacks.map((stack) => stack.label)).toEqual(['Платежи']);
  });

  it('объект без правил — стопок нет', () => {
    expect(paymentObjectStacks(makeObject(), feedByCount(0))).toEqual([]);
  });
});

describe('paymentObjectStacks — обрезка (решение владельца 10.09)', () => {
  it('обе группы есть: максимум 4 иконки в каждой', () => {
    const object = makeObject({
      autoPayRules: [key('a-1'), key('a-2'), key('a-3'), key('a-4'), key('a-5')],
      otherRules: [key('p-1'), key('p-2'), key('p-3'), key('p-4'), key('p-5'), key('p-6')],
    });

    const stacks = paymentObjectStacks(object, feedByCount(6));

    expect(stacks.map((stack) => stack.keys.length)).toEqual([4, 4]);
  });

  it('ровно четыре — без обрезки', () => {
    const object = makeObject({
      autoPayRules: [key('a-1'), key('a-2'), key('a-3'), key('a-4')],
      otherRules: [key('p-1')],
    });

    const stacks = paymentObjectStacks(object, feedByCount(4));

    expect(stacks.map((stack) => stack.keys.length)).toEqual([4, 1]);
  });

  it('одна группа: максимум 7 иконок', () => {
    const object = makeObject({
      otherRules: [
        key('p-1'),
        key('p-2'),
        key('p-3'),
        key('p-4'),
        key('p-5'),
        key('p-6'),
        key('p-7'),
        key('p-8'),
        key('p-9'),
      ],
    });

    const stacks = paymentObjectStacks(object, feedByCount(9));

    expect(stacks.map((stack) => stack.keys.length)).toEqual([7]);
  });

  it('одна автоплатёжная группа: тот же лимит 7', () => {
    const object = makeObject({
      autoPayRules: [key('a-1'), key('a-2'), key('a-3'), key('a-4'), key('a-5'), key('a-6'), key('a-7'), key('a-8')],
    });

    const stacks = paymentObjectStacks(object, feedByCount(8));

    expect(stacks.map((stack) => stack.keys.length)).toEqual([7]);
  });

  it('обрезка сохраняет первых ключи в серверном порядке', () => {
    const object = makeObject({
      autoPayRules: [key('a-1')],
      otherRules: [key('p-1'), key('p-2'), key('p-3'), key('p-4'), key('p-5')],
    });

    const stacks = paymentObjectStacks(object, feedByCount(5));

    expect(
      stacks.map((stack) => stack.keys.map((rule) => rule.paymentId)),
    ).toEqual([['a-1'], ['p-1', 'p-2', 'p-3', 'p-4']]);
  });
});

describe('paymentObjectStacks — джойн с фидом', () => {
  it('иконка и цвет — из категории правила фида', () => {
    const object = makeObject({ otherRules: [key('p-1', true)] });
    const payment = makeGlobalPayment({
      id: 'p-1',
      category: { source: 'default', slug: 'insurance', label: 'Страхование' },
    });

    const stacks = paymentObjectStacks(
      object,
      new Map([['p-1', payment]]),
    );

    const style = categoryStyle('default', 'insurance');
    expect(stacks.flatMap((stack) => stack.keys)).toEqual([
      {
        paymentId: 'p-1',
        hasOverdue: true,
        icon: style.icon,
        color: style.color,
      },
    ]);
  });

  it('правила нет в фиде — дефолтный стиль, флаг просрочки из ключа', () => {
    const object = makeObject({ otherRules: [key('ghost', true)] });

    const stacks = paymentObjectStacks(object, feedByCount(0));

    const fallback = categoryStyle('custom');
    expect(stacks.flatMap((stack) => stack.keys)).toEqual([
      {
        paymentId: 'ghost',
        hasOverdue: true,
        icon: fallback.icon,
        color: fallback.color,
      },
    ]);
  });

  it('пользовательская категория — дефолт пользовательских', () => {
    const object = makeObject({ otherRules: [key('p-1')] });
    const payment = makeGlobalPayment({
      id: 'p-1',
      category: { source: 'custom', label: 'Своя' },
    });

    const stacks = paymentObjectStacks(
      object,
      new Map([['p-1', payment]]),
    );

    const expected = categoryStyle('custom');
    expect(
      stacks.flatMap((stack) =>
        stack.keys.map((rule) => [rule.icon, rule.color]),
      ),
    ).toEqual([[expected.icon, expected.color]]);
  });
});

import { describe, expect, it } from 'vitest';
import { dueDaysLabel, groupDueByDay } from './due-days';
import type { PaymentDueItem } from '../model/types';

/** Строка-фикстура: различаются только daysLeft и идентификаторы. */
const dueItem = (daysLeft: number, operationId: string): PaymentDueItem => ({
  paymentId: `rule-${operationId}`,
  operationId,
  propertyId: '0198b6a7-1000-7000-8000-00000000prop',
  propertyName: 'Квартира',
  title: 'ЖКУ',
  amountKopecks: 500_000,
  type: 'expense',
  category: { source: 'default', slug: 'utilities', label: 'ЖКУ' },
  today: '2026-09-08',
  dueDate: '2026-09-10',
  daysLeft,
  reminderOffsetDays: 3,
});

describe('dueDaysLabel — заголовок реального дня окна (#1347, решение владельца 13.10)', () => {
  it.each([
    [0, 'Сегодня'],
    [1, 'Завтра'],
    [2, 'Через 2 дня'],
    [3, 'Через 3 дня'],
    [4, 'Через 4 дня'],
    [5, 'Через 5 дней'],
    [6, 'Через 6 дней'],
    [7, 'Через 7 дней'],
  ])('%i дн. — «%s»', (daysLeft, label) => {
    expect(dueDaysLabel(daysLeft)).toBe(label);
  });
});

describe('groupDueByDay — группы по реальным дням (#1347, ревизия бакетов 2..3/4..7)', () => {
  it('каждый остаток дней — своя группа, серверный порядок входа сохраняется', () => {
    const groups = groupDueByDay([
      dueItem(0, 'op-today'),
      dueItem(1, 'op-tomorrow'),
      dueItem(2, 'op-in2'),
      dueItem(3, 'op-in3'),
      dueItem(7, 'op-in7'),
    ]);
    expect(groups.map((group) => group.daysLeft)).toStrictEqual([0, 1, 2, 3, 7]);
    expect(groups.map((group) => group.label)).toStrictEqual([
      'Сегодня',
      'Завтра',
      'Через 2 дня',
      'Через 3 дня',
      'Через 7 дней',
    ]);
  });

  it('строки одного дня собираются в одну группу, порядок входа внутри не трогается', () => {
    const groups = groupDueByDay([
      dueItem(3, 'op-first'),
      dueItem(3, 'op-second'),
      dueItem(2, 'op-near'),
    ]);
    expect(groups.map((group) => group.daysLeft)).toStrictEqual([2, 3]);
    expect(groups[1]?.items.map((item) => item.operationId)).toStrictEqual([
      'op-first',
      'op-second',
    ]);
  });

  it('порядок входа не важен: группы всегда по возрастанию дней', () => {
    const forward = groupDueByDay([
      dueItem(0, 'op-a'),
      dueItem(7, 'op-b'),
      dueItem(3, 'op-c'),
    ]);
    const shuffled = groupDueByDay([
      dueItem(3, 'op-c'),
      dueItem(0, 'op-a'),
      dueItem(7, 'op-b'),
    ]);
    expect(forward.map((group) => group.daysLeft)).toStrictEqual([0, 3, 7]);
    expect(shuffled.map((group) => group.daysLeft)).toStrictEqual([0, 3, 7]);
  });

  it('пустой вход — пустой список групп', () => {
    expect(groupDueByDay([])).toStrictEqual([]);
  });
});

import { describe, expect, it } from 'vitest';
import { makePaymentDueItem } from '@/entities/payment';
import {
  DUE_SORT_PARAMS,
  dueGroupsInOrder,
  parseDueSortParams,
  serializeDueSortToParams,
} from './payments-due-model';

describe('parseDueSortParams — ?sort= страницы «Платежи на оплату»', () => {
  it('отсутствующее, пустое, массивное и неизвестное — дефолт «Сначала новые»', () => {
    expect(parseDueSortParams(undefined)).toBe('new');
    expect(parseDueSortParams([])).toBe('new');
    expect(parseDueSortParams('')).toBe('new');
    expect(parseDueSortParams('nonsense')).toBe('new');
  });

  it('«Сначала старые» читается из адреса, массивное значение — битый дубликат (дефолт)', () => {
    expect(parseDueSortParams('old')).toBe('old');
    expect(parseDueSortParams(['old'])).toBe('new');
  });
});

describe('serializeDueSortToParams — конвенция состояния в адресе', () => {
  it('дефолт «Сначала новые» параметра не создаёт', () => {
    expect(serializeDueSortToParams('new')).toStrictEqual({});
  });

  it('«Сначала старые» пишет свой параметр', () => {
    expect(serializeDueSortToParams('old')).toStrictEqual({ sort: 'old' });
    expect(DUE_SORT_PARAMS).toStrictEqual(['sort']);
  });
});

describe('dueGroupsInOrder — направление групп страницы (решение владельца 13.10)', () => {
  // Серверный порядок (#1347): реальные дни по возрастанию, внутри дня —
  // id операции; страницы группирует по daysLeft.
  const serverOrder = [
    makePaymentDueItem({ operationId: 'op-a', dueDate: '2026-10-13', daysLeft: 0 }),
    makePaymentDueItem({ operationId: 'op-b', dueDate: '2026-10-13', daysLeft: 0 }),
    makePaymentDueItem({ operationId: 'op-c', dueDate: '2026-10-14', daysLeft: 1 }),
    makePaymentDueItem({ operationId: 'op-d', dueDate: '2026-10-16', daysLeft: 3 }),
    makePaymentDueItem({ operationId: 'op-e', dueDate: '2026-10-15', daysLeft: 2 }),
  ];

  it('«Сначала новые» — дни по возрастанию, «Сегодня» первая группа', () => {
    const groups = dueGroupsInOrder(serverOrder, 'new');
    expect(groups.map((group) => group.daysLeft)).toStrictEqual([0, 1, 2, 3]);
    expect(groups[0]?.label).toBe('Сегодня');
    // Внутри дня серверный порядок не трогается.
    expect(groups[0]?.items.map((item) => item.operationId)).toStrictEqual([
      'op-a',
      'op-b',
    ]);
  });

  it('«Сначала старые» — группы задом наперёд, внутри дня порядок прежний', () => {
    const groups = dueGroupsInOrder(serverOrder, 'old');
    expect(groups.map((group) => group.daysLeft)).toStrictEqual([3, 2, 1, 0]);
    expect(groups[0]?.label).toBe('Через 3 дня');
    expect(groups.at(-1)?.label).toBe('Сегодня');
    expect(groups.at(-1)?.items.map((item) => item.operationId)).toStrictEqual([
      'op-a',
      'op-b',
    ]);
  });

  it('пустое окно — пустой список групп в обоих направлениях', () => {
    expect(dueGroupsInOrder([], 'new')).toStrictEqual([]);
    expect(dueGroupsInOrder([], 'old')).toStrictEqual([]);
  });
});

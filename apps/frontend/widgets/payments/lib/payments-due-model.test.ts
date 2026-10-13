import { describe, expect, it } from 'vitest';
import { makePaymentDueItem } from '@/entities/payment';
import {
  DUE_SORT_PARAMS,
  dueDaysLabel,
  dueListInOrder,
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

describe('dueListInOrder — направление строк внутри бакетов', () => {
  // Серверный порядок (#1346): бакеты по daysLeft asc, «Сначала новые»
  // внутри (dueDate desc). Здесь — окно «Через 3 дня» (daysLeft 2..3):
  // дальний срок (16-е) стоит перед ближним (15-е).
  const serverOrder = [
    makePaymentDueItem({ operationId: 'o-today', dueDate: '2026-10-13', daysLeft: 0 }),
    makePaymentDueItem({ operationId: 'o-far', dueDate: '2026-10-16', daysLeft: 3 }),
    makePaymentDueItem({ operationId: 'o-near', dueDate: '2026-10-15', daysLeft: 2 }),
  ];

  it('«Сначала новые» — серверный порядок не трогается', () => {
    expect(dueListInOrder(serverOrder, 'new')).toStrictEqual(serverOrder);
  });

  it('«Сначала старые» переворачивает строки только внутри бакета', () => {
    const ordered = dueListInOrder(serverOrder, 'old');
    expect(ordered.map((item) => item.operationId)).toStrictEqual([
      'o-today',
      'o-near',
      'o-far',
    ]);
  });

  it('равные даты в «Сначала старые» — тай-брейк по id операции asc (зеркало серверного desc)', () => {
    const tied = [
      makePaymentDueItem({ operationId: 'o-b', dueDate: '2026-10-13', daysLeft: 0 }),
      makePaymentDueItem({ operationId: 'o-a', dueDate: '2026-10-13', daysLeft: 0 }),
    ];
    expect(dueListInOrder(tied, 'old').map((item) => item.operationId)).toStrictEqual([
      'o-a',
      'o-b',
    ]);
  });
});

describe('dueDaysLabel — метка даты карточки хаба', () => {
  it.each([
    [0, 'Сегодня'],
    [1, 'Завтра'],
    [2, 'Через 2 дня'],
    [3, 'Через 3 дня'],
    [5, 'Через 5 дней'],
    [7, 'Через 7 дней'],
  ])('%i дн. — «%s»', (daysLeft, label) => {
    expect(dueDaysLabel(daysLeft)).toBe(label);
  });
});

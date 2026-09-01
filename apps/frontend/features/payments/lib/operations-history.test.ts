import { describe, expect, it } from 'vitest';
import type { PaymentOperation } from '@/entities/payment';
import { groupOperationsByDate, groupPaidOperations } from './operations-history';

const paid = (id: string, date: string): PaymentOperation => ({
  id,
  propertyId: 'p1',
  paymentId: 'pay1',
  date,
  paidDate: date,
  status: 'paid',
  type: 'expense',
  title: 'Аренда',
  amountKopecks: 4500000,
  paymentForm: 'transfer',
  categoryLabel: 'Арендная плата',
  categorySlug: 'rent',
});

describe('groupPaidOperations', () => {
  it('называет группы «Сегодня», «Вчера» и датой с годом вне текущего', () => {
    const groups = groupPaidOperations(
      [paid('a', '2026-08-27'), paid('b', '2026-08-26'), paid('c', '2026-08-11'), paid('d', '2025-05-13')],
      '2026-08-27',
    );
    expect(groups.map((group) => group.label)).toStrictEqual([
      'Сегодня',
      'Вчера',
      '11 августа',
      '13 мая, 2025',
    ]);
  });

  it('складывает подряд идущие операции одной даты в одну группу в порядке сервера', () => {
    // Порядок входа = серверная сортировка (desc — сначала новые): группы
    // идут в том же порядке, «20 августа» раньше «19 августа».
    const groups = groupPaidOperations(
      [paid('a', '2026-08-20'), paid('b', '2026-08-20'), paid('c', '2026-08-19')],
      '2026-08-27',
    );
    expect(groups).toHaveLength(2);
    expect(groups[0]?.label).toBe('20 августа');
    expect(groups[0]?.operations.map((operation) => operation.id)).toStrictEqual(['a', 'b']);
    expect(groups[1]?.label).toBe('19 августа');
    expect(groups[1]?.operations.map((operation) => operation.id)).toStrictEqual(['c']);
  });

  it('пустая история даёт пустой список групп', () => {
    expect(groupPaidOperations([], '2026-08-27')).toStrictEqual([]);
  });
});

describe('groupOperationsByDate', () => {
  it('называет группы с датой через запятую: «Сегодня, 27 августа»', () => {
    const groups = groupOperationsByDate(
      [paid('a', '2026-08-27'), paid('b', '2026-08-26'), paid('c', '2026-08-11'), paid('d', '2025-05-13')],
      '2026-08-27',
    );
    expect(groups.map((group) => group.label)).toStrictEqual([
      'Сегодня, 27 августа',
      'Вчера, 26 августа',
      '11 августа',
      '13 мая, 2025',
    ]);
  });

  it('складывает операции одной даты в одну группу в порядке сервера', () => {
    const groups = groupOperationsByDate(
      [paid('a', '2026-08-20'), paid('b', '2026-08-20')],
      '2026-08-27',
    );
    expect(groups).toHaveLength(1);
    expect(groups[0]?.label).toBe('20 августа');
    expect(groups[0]?.operations.map((operation) => operation.id)).toStrictEqual(['a', 'b']);
  });

  it('пустой список даёт пустой список групп', () => {
    expect(groupOperationsByDate([], '2026-08-27')).toStrictEqual([]);
  });
});

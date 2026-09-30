import { describe, expect, it } from 'vitest';
import type { PaymentOperation } from '@/entities/payment';
import {
  groupOperationsByDate,
  groupPaidOperations,
  parseHistoryOrderParams,
  serializeHistoryOrderToParams,
} from './operations-history';

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

/** Оплаченный вперёд: плановая дата вхождения (период, за который платят)
 * расходится с фактической датой оплаты (решение #933/#994). */
const paidAhead = (id: string, date: string, paidDate: string): PaymentOperation => ({
  ...paid(id, date),
  paidDate,
});

describe('groupPaidOperations', () => {
  it('называет группы «Сегодня», «Вчера» и датой с годом всегда (1302:52209)', () => {
    const groups = groupPaidOperations(
      [paid('a', '2026-08-27'), paid('b', '2026-08-26'), paid('c', '2026-08-11'), paid('d', '2025-05-13')],
      '2026-08-27',
    );
    expect(groups.map((group) => group.label)).toStrictEqual([
      'Сегодня',
      'Вчера',
      '11 августа, 2026',
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
    expect(groups[0]?.label).toBe('20 августа, 2026');
    expect(groups[0]?.operations.map((operation) => operation.id)).toStrictEqual(['a', 'b']);
    expect(groups[1]?.label).toBe('19 августа, 2026');
    expect(groups[1]?.operations.map((operation) => operation.id)).toStrictEqual(['c']);
  });

  it('досрочно оплаченное будущее вхождение остаётся в плановой группе своего периода — учёт, не касса (#466)', () => {
    // История платежа сортируется по плановой (дефолт контракта, #992):
    // платёж за октябрь, оплаченный сегодня, стоит выше сентябрьских —
    // и группируется октябрём периода, не днём оплаты.
    const groups = groupPaidOperations(
      [paidAhead('a', '2026-10-01', '2026-08-27'), paid('b', '2026-08-20')],
      '2026-08-27',
    );
    expect(groups.map((group) => group.date)).toStrictEqual(['2026-10-01', '2026-08-20']);
    expect(groups[0]?.label).toBe('1 октября, 2026');
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

describe('groupOperationsByDate — лента по факту оплаты: ключ группы paidDate (#933/#994)', () => {
  it('группирует фактической датой: вход отсортирован по факту, плановые даты немонотонны — группы монотонны', () => {
    // Серверная сортировка sort=paid_date desc (решение #933): оплаченный
    // сегодня платёж за октябрь стоит выше вчерашнего факта, хотя плановая
    // дата октябрьская. Ключ группы — день оплаты, не день периода.
    const groups = groupOperationsByDate(
      [
        paidAhead('a', '2026-10-01', '2026-09-30'),
        paidAhead('b', '2026-11-01', '2026-09-30'),
        paid('c', '2026-09-29'),
      ],
      '2026-09-30',
    );
    expect(groups.map((group) => group.date)).toStrictEqual(['2026-09-30', '2026-09-29']);
    expect(groups.map((group) => group.label)).toStrictEqual([
      'Сегодня, 30 сентября',
      'Вчера, 29 сентября',
    ]);
    expect(groups[0]?.operations.map((operation) => operation.id)).toStrictEqual(['a', 'b']);
  });

  it('оплаченные наперёд месяцы одной даты оплаты — одна группа, лента не разваливается на повторяющиеся плановые группы', () => {
    // Группировка по плановой дате при сортировке по факту дала бы
    // «Сегодня», «1 октября», «1 ноября»… вперемешку — банковский порядок
    // требует одного дня факта одной группой подряд.
    const groups = groupOperationsByDate(
      [
        paidAhead('a', '2026-10-01', '2026-09-30'),
        paidAhead('b', '2026-11-01', '2026-09-30'),
        paidAhead('c', '2026-12-01', '2026-09-30'),
      ],
      '2026-09-30',
    );
    expect(groups).toHaveLength(1);
    expect(groups[0]?.label).toBe('Сегодня, 30 сентября');
    expect(groups[0]?.operations).toHaveLength(3);
  });
});

describe('parseHistoryOrderParams — разбор ?order= страниц истории (#785)', () => {
  it('отсутствие и неизвестное значение — дефолт «сначала новые»', () => {
    expect(parseHistoryOrderParams(undefined)).toBe('desc');
    expect(parseHistoryOrderParams('')).toBe('desc');
    expect(parseHistoryOrderParams('newest')).toBe('desc');
  });

  it('читает «сначала старые»', () => {
    expect(parseHistoryOrderParams('asc')).toBe('asc');
  });
});

describe('serializeHistoryOrderToParams — патч ?order= для адреса (#785)', () => {
  it('дефолт «сначала новые» параметров не создаёт', () => {
    expect(serializeHistoryOrderToParams('desc')).toStrictEqual({});
  });

  it('«сначала старые» пишет order=asc', () => {
    expect(serializeHistoryOrderToParams('asc')).toStrictEqual({ order: 'asc' });
  });
});

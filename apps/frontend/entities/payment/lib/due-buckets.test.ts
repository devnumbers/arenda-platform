import { describe, expect, it } from 'vitest';
import { DUE_BUCKET_LABELS, DUE_BUCKET_ORDER, dueBucketOf, groupDueByBucket } from './due-buckets';
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

describe('dueBucketOf — бакет по остатку дней (#1346)', () => {
  it('контрактные границы: 0 → сегодня, 1 → завтра', () => {
    expect(dueBucketOf(0)).toBe('today');
    expect(dueBucketOf(1)).toBe('tomorrow');
  });

  it('промежуточные остатки прижимаются к ближнему заголовку: 2..3 → «Через 3 дня», 4..7 → «Через 7 дней»', () => {
    expect(dueBucketOf(2)).toBe('in3days');
    expect(dueBucketOf(3)).toBe('in3days');
    expect(dueBucketOf(4)).toBe('in7days');
    expect(dueBucketOf(7)).toBe('in7days');
  });
});

describe('groupDueByBucket — группировка секции', () => {
  it('бакеты идут в фиксированном порядке, вход секции уже отсортирован сервером', () => {
    const groups = groupDueByBucket([
      dueItem(0, 'op-today'),
      dueItem(1, 'op-tomorrow'),
      dueItem(2, 'op-in3'),
      dueItem(4, 'op-in7'),
    ]);
    expect(groups.map((group) => group.bucket)).toStrictEqual(['today', 'tomorrow', 'in3days', 'in7days']);
    expect(groups.map((group) => group.label)).toStrictEqual([
      DUE_BUCKET_LABELS.today,
      DUE_BUCKET_LABELS.tomorrow,
      DUE_BUCKET_LABELS.in3days,
      DUE_BUCKET_LABELS.in7days,
    ]);
  });

  it('пустые бакеты не показываются, порядок внутри сохраняется', () => {
    const groups = groupDueByBucket([
      dueItem(3, 'op-first'),
      dueItem(2, 'op-second'),
      dueItem(7, 'op-far'),
    ]);
    expect(groups.map((group) => group.bucket)).toStrictEqual(['in3days', 'in7days']);
    const in3 = groups[0];
    expect(in3?.items.map((item) => item.operationId)).toStrictEqual(['op-first', 'op-second']);
    expect(in3?.items[0]?.daysLeft).toBe(3);
  });

  it('пустое окно — пустая группировка', () => {
    expect(groupDueByBucket([])).toStrictEqual([]);
  });

  it('порядок бакетов каноничен', () => {
    expect(DUE_BUCKET_ORDER).toStrictEqual(['today', 'tomorrow', 'in3days', 'in7days']);
  });
});

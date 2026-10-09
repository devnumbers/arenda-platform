import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapGlobalPaymentSearch, mapPayment, mapPaymentOperation } from './mappers';

type PaymentDto = components['schemas']['PaymentResponse'];
type OperationDto = components['schemas']['OperationResponse'];

const paymentDto: PaymentDto = {
  id: '0198b6a7-1000-7000-8000-000000000001',
  propertyId: '0198b6a7-1000-7000-8000-00000000prop',
  type: 'expense',
  title: 'Арендная плата',
  amountKopecks: 5_600_000,
  recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: true },
  since: '2026-01-31',
  endDate: null,
  autoPay: false,
  notifyAutoPaid: false,
  category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
  isFavorite: false,
  isCompleted: false,
  isRentalManaged: false,
  isRentalCompleted: false,
  nearestDate: '2026-09-01',
  pauses: [
    { fromDate: '2026-03-01', toDate: '2026-04-01' },
    { fromDate: '2026-05-01', toDate: null },
  ],
  createdAt: '2026-01-31T09:00:00Z',
  updatedAt: '2026-05-01T10:30:00Z',
};

describe('mapPayment — DTO → entity', () => {
  const payment = mapPayment(paymentDto);

  it('скалярные поля переносятся один в один (контракт camelCase)', () => {
    expect(payment.id).toBe(paymentDto.id);
    expect(payment.title).toBe('Арендная плата');
    expect(payment.amountKopecks).toBe(5_600_000);
    expect(payment.recurrence).toStrictEqual({ kind: 'monthly', daysOfMonth: [], lastDay: true });
    expect(payment.since).toBe('2026-01-31');
    expect(payment.isFavorite).toBe(false);
    expect(payment.category).toStrictEqual({
      source: 'default',
      slug: 'rent',
      id: undefined,
      label: 'Арендная плата',
    });
  });

  it('nullables нормализуются к опциональности', () => {
    expect(payment.endDate).toBeUndefined();
  });

  it('isRentalManaged переносится как есть — флаг гейта мутаций аренды (#818)', () => {
    expect(payment.isRentalManaged).toBe(false);
    const managed = mapPayment({ ...paymentDto, isRentalManaged: true });
    expect(managed.isRentalManaged).toBe(true);
  });

  it('isRentalCompleted переносится как есть — состояние аренды для «Изменить аренду» (#1158)', () => {
    expect(payment.isRentalCompleted).toBe(false);
    const completedRental = mapPayment({ ...paymentDto, isRentalCompleted: true });
    expect(completedRental.isRentalCompleted).toBe(true);
  });

  it('nearestDate переносится как есть — «Следующая дата оплаты» сервера (#993)', () => {
    // Список объекта несёт её на каждом правиле; явный null (пауза/
    // завершённый) остаётся null'ом — «нет даты» часть семантики.
    expect(payment.nearestDate).toBe('2026-09-01');
    expect(mapPayment({ ...paymentDto, nearestDate: null }).nearestDate).toBeNull();
  });

  it('напоминание: оффсет приходит числом, null и отсутствие — «нет напоминания» (карта #822)', () => {
    expect(payment.reminderOffsetDays).toBeUndefined();
    expect(mapPayment({ ...paymentDto, reminderOffsetDays: null }).reminderOffsetDays).toBeUndefined();
    expect(mapPayment({ ...paymentDto, reminderOffsetDays: 3 }).reminderOffsetDays).toBe(3);
  });

  it('notifyAutoPaid переносится как есть — гейт события «Автоплатёж исполнен» (#1189)', () => {
    expect(payment.notifyAutoPaid).toBe(false);
    expect(mapPayment({ ...paymentDto, notifyAutoPaid: true }).notifyAutoPaid).toBe(true);
  });

  it('интервалы пауз переименовываются в [from, to), открытая бессрочная без to', () => {
    expect(payment.pauses).toStrictEqual([
      { from: '2026-03-01', to: '2026-04-01' },
      { from: '2026-05-01', to: undefined },
    ]);
  });

  it('endDate датой приходит как есть, категория custom несёт id вместо слага', () => {
    const ended: PaymentDto = {
      ...paymentDto,
      endDate: '2026-12-31',
      category: { source: 'custom', id: '0198b6a7-user', label: 'Своя' },
    };
    const mapped = mapPayment(ended);
    expect(mapped.endDate).toBe('2026-12-31');
    expect(mapped.category.slug).toBeUndefined();
    expect(mapped.category.id).toBe('0198b6a7-user');
  });
});

const operationDto: OperationDto = {
  id: '0198b6a7-op00-7000-8000-000000000001',
  propertyId: '0198b6a7-1000-7000-8000-00000000prop',
  paymentId: null,
  date: '2026-08-27',
  paidDate: null,
  status: 'overdue',
  type: 'expense',
  title: 'ЖКУ',
  amountKopecks: 320_000,
  categoryLabel: 'Коммунальные услуги',
  categorySlug: null,
  updatedAt: '2026-08-27T10:00:00Z',
};

describe('mapPaymentOperation — DTO → entity', () => {
  const operation = mapPaymentOperation(operationDto);

  it('просрочка и ручные поля нормализуются: nulls → опциональность', () => {
    expect(operation.status).toBe('overdue');
    expect(operation.paymentId).toBeNull();
    expect(operation.paidDate).toBeUndefined();
    expect(operation.categorySlug).toBeUndefined();
    expect(operation.amountKopecks).toBe(320_000);
  });

  it('оплаченное вхождение переносит фактическую дату и снапшот категории', () => {
    const paid: OperationDto = {
      ...operationDto,
      status: 'paid',
      paidDate: '2026-08-25',
      paymentId: '0198b6a7-rule',
      categorySlug: 'utilities',
    };
    const mapped = mapPaymentOperation(paid);
    expect(mapped.paidDate).toBe('2026-08-25');
    expect(mapped.categorySlug).toBe('utilities');
  });
});

const globalItemDto: components['schemas']['PaymentGlobalItem'] = {
  id: '0198b6a7-rule',
  propertyId: '0198b6a7-1000-7000-8000-00000000prop',
  propertyName: 'Моя квартира',
  title: 'Арендная плата',
  amountKopecks: 5_600_000,
  type: 'expense',
  category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
  autoPay: true,
  isFavorite: true,
  favoriteOrder: 1,
  today: '2026-09-09',
  nearestDate: '2026-09-15',
  overdueOperationCount: 0,
  overdueDays: null,
  oldestOverdueOperationId: null,
};

describe('mapGlobalPaymentSearch — DTO → entity (поиск #575, чипы #581, курсор #597, total #599)', () => {
  const searchDto: components['schemas']['PaymentsSearchGlobalResponse'] = {
    items: [globalItemDto],
    matchedCategories: [
      { source: 'default', slug: 'rent', label: 'Арендная плата' },
      { source: 'custom', id: '0198b6a7-user', label: 'Своя категория' },
    ],
    nextCursor: 'cursor-of-page-two',
    total: 2,
  };

  const search = mapGlobalPaymentSearch(searchDto);

  it('строки — тот же маппер, что у фида главного экрана', () => {
    expect(search.items).toHaveLength(1);
    expect(search.items[0]?.id).toBe('0198b6a7-rule');
    expect(search.items[0]?.favoriteOrder).toBe(1);
    expect(search.items[0]?.category).toStrictEqual({
      source: 'default',
      slug: 'rent',
      id: undefined,
      label: 'Арендная плата',
    });
  });

  it('чипы — чистые категории: по одной на категорию, без направления и счётчика (#602)', () => {
    expect(search.matchedCategories).toStrictEqual([
      {
        source: 'default',
        slug: 'rent',
        id: undefined,
        label: 'Арендная плата',
      },
      {
        source: 'custom',
        slug: undefined,
        id: '0198b6a7-user',
        label: 'Своя категория',
      },
    ]);
  });

  it('nextCursor проходит opaque-строкой, null — исчерпано (#597)', () => {
    expect(search.nextCursor).toBe('cursor-of-page-two');
    expect(mapGlobalPaymentSearch({ ...searchDto, nextCursor: null }).nextCursor).toBeNull();
  });
});

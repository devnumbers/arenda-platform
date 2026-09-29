import { describe, expect, it } from 'vitest';
import { paymentOperationKeys } from '@/shared/api/query-keys';
import { paymentOperationsGateQueryOptions } from './queries';

describe('paymentOperationsGateQueryOptions — пара гасящих списков «Оплатить»', () => {
  const propertyId = '018f0000-0000-7000-8000-000000000001';
  const paymentId = '018f0000-0000-7000-8000-000000000002';

  it('возвращает ровно два статусных списка: просроченные, затем плановые', () => {
    const [overdue, planned] = paymentOperationsGateQueryOptions({ propertyId, paymentId });

    expect(overdue.queryKey).toStrictEqual(
      paymentOperationKeys.byPaymentWithStatus(propertyId, paymentId, 'overdue'),
    );
    expect(planned.queryKey).toStrictEqual(
      paymentOperationKeys.byPaymentWithStatus(propertyId, paymentId, 'planned'),
    );
    expect(typeof overdue.queryFn).toBe('function');
    expect(typeof planned.queryFn).toBe('function');
  });
});

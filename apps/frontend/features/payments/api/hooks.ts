'use client';

import {
  useQuery,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapPayment, mapPaymentOperation } from '@/entities/payment';
import type { Payment, PaymentOperation } from '@/entities/payment';
import { paymentKeys, paymentOperationKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type PaymentsResponse = components['schemas']['PaymentsResponse'];
type OperationsResponse = components['schemas']['OperationsResponse'];

/** Список платежей объекта — правил с флагом автоплатежа и избранным
 * (ADR 0049): без пагинации, порядок — серверный (по дате заведения). */
export function usePayments(propertyId: string): UseQueryResult<Payment[], ApiError> {
  return useQuery({
    queryKey: paymentKeys.list(propertyId),
    queryFn: async () => {
      const response = await apiClient<PaymentsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/payments`,
      );
      return response.items.map(mapPayment);
    },
    enabled: Boolean(propertyId),
  });
}

/**
 * Просроченные операции объекта (секция «Просроченные» экрана «Платежи
 * объекта»): статус overdue вычисляет сервер по «сегодня» в TZ собственника
 * (ADR 0048), сортировка asc — старейшая просрочка первой, долг разбирают
 * по порядку накопления. Порция — серверный дефолт 50; полный список с
 * пагинацией — отдельный подэкран следующего среза.
 */
export function usePropertyOverdueOperations(
  propertyId: string,
): UseQueryResult<PaymentOperation[], ApiError> {
  return useQuery({
    queryKey: paymentOperationKeys.overdueByProperty(propertyId),
    queryFn: async () => {
      const response = await apiClient<OperationsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/operations?status=overdue&order=asc`,
      );
      return response.items.map(mapPaymentOperation);
    },
    enabled: Boolean(propertyId),
  });
}

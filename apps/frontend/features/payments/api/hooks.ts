'use client';

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type UseInfiniteQueryResult,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapPayment, mapPaymentOperation } from '@/entities/payment';
import type {
  Payment,
  PaymentCreateCommand,
  PaymentFavoriteCommand,
  PaymentOperation,
} from '@/entities/payment';
import {
  paymentKeys,
  paymentOperationKeys,
  type PaymentOperationOrder,
  type PaymentOperationStatusFilter,
} from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type PaymentsResponse = components['schemas']['PaymentsResponse'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type PaymentResponseDto = components['schemas']['PaymentResponse'];

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

/**
 * Создание платежа (визард #464): контракт POST создания camelCase — команда
 * уходит телом без переупаковки; `since` проставляет сервер (сегодня в TZ
 * собственника). Инвалидация списков платежей и операций объекта.
 */
export function useCreatePayment(
  propertyId: string,
): UseMutationResult<Payment, ApiError, PaymentCreateCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: PaymentCreateCommand) => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments`,
        { method: 'POST', body: JSON.stringify(command) },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}

/**
 * Чтение платежа для страницы платежа (#465). Порции списков операций
 * отдельно — этот хук тянет только правило.
 */
export function usePayment(
  propertyId: string,
  paymentId: string,
): UseQueryResult<Payment, ApiError> {
  return useQuery({
    queryKey: paymentKeys.detail(propertyId, paymentId),
    queryFn: async () => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`,
      );
      return mapPayment(response);
    },
    enabled: Boolean(propertyId) && Boolean(paymentId),
  });
}

/**
 * Операции платежа одного статуса — вход мутации «Оплатить» (head asc =
 * старейшее неоплаченное; просроченные запрашиваются отдельным ключом и
 * имеют приоритет над плановыми) и секция «Просроченные» страницы платежа
 * (#465). Порядок asc закреплён контрактом «долг разбирают по порядку
 * накопления»; порция — серверный дефолт 50.
 */
export function usePaymentOperationsByStatus(
  propertyId: string,
  paymentId: string,
  status: PaymentOperationStatusFilter,
): UseQueryResult<PaymentOperation[], ApiError> {
  return useQuery({
    queryKey: paymentOperationKeys.byPaymentWithStatus(propertyId, paymentId, status),
    queryFn: async () => {
      const response = await apiClient<OperationsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`
          + `/operations?status=${status}&order=asc`,
      );
      return response.items.map(mapPaymentOperation);
    },
    enabled: Boolean(propertyId) && Boolean(paymentId),
  });
}

/** Размер порции всех списков операций (правило платформы, резолюция #452:
 * по 50 + бесконечный скролл; серверный дефолт — те же 50). */
export const OPERATIONS_PAGE_SIZE = 50;

/**
 * Порции операций платежа для подэкранов страницы (#466): «История»
 * (status=paid, порядок по чипу «Новые») и полный список просроченных
 * (status=overdue, asc — долг разбирают по порядку накопления). pageParam —
 * offset; следующая страница есть, пока порция полная. Направление — часть
 * ключа: переключение чипа читает другой кэш с первой порции.
 */
export function usePaymentOperationsPaged(
  propertyId: string,
  paymentId: string,
  params: { readonly status: PaymentOperationStatusFilter; readonly order: PaymentOperationOrder },
): UseInfiniteQueryResult<ReadonlyArray<PaymentOperation>, ApiError> {
  const { status, order } = params;
  return useInfiniteQuery({
    queryKey: paymentOperationKeys.byPaymentPaged(propertyId, paymentId, status, order),
    queryFn: async ({ pageParam }) => {
      const response = await apiClient<OperationsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`
          + `/operations?status=${status}&order=${order}`
          + `&limit=${OPERATIONS_PAGE_SIZE}&offset=${String(pageParam)}`,
      );
      return response.items.map(mapPaymentOperation);
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) =>
      lastPage.length < OPERATIONS_PAGE_SIZE
        ? undefined
        : allPages.length * OPERATIONS_PAGE_SIZE,
    select: (data) => data.pages.flat(),
    enabled: Boolean(propertyId) && Boolean(paymentId),
  });
}

/**
 * Пауза платежа (история 18 спеки #453): бессрочно с сегодняшнего дня,
 * генерация останавливается. Сервер в транзакции гоняет тик, поэтому
 * инвалидируются и правила, и операции.
 */
export function usePausePayment(
  propertyId: string,
  paymentId: string,
): UseMutationResult<Payment, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}/pause`,
        { method: 'POST' },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}

/** Возобновление без подтверждения (история 20); расписание идёт от якорей. */
export function useResumePayment(
  propertyId: string,
  paymentId: string,
): UseMutationResult<Payment, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}/resume`,
        { method: 'POST' },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}

/**
 * Атомарный toggle избранного (PUT favorite c телом `{favorite}`,
 * резолюция #452) — без read-modify-write через PATCH.
 */
export function useSetPaymentFavorite(
  propertyId: string,
  paymentId: string,
): UseMutationResult<Payment, ApiError, PaymentFavoriteCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: PaymentFavoriteCommand) => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}/favorite`,
        { method: 'PUT', body: JSON.stringify(command) },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
    },
  });
}

/**
 * «Оплатить сейчас» (история 24/25): `planned → paid` у конкретного
 * вхождения, дата оплаты — серверная, расписание не сдвигается. Какая
 * операция гасится («старейшее неоплаченное»), решает виджет по спискам
 * операций; сервер подтверждает контрактной ошибкой 409 на повторную
 * оплату.
 */
export function usePayOperation(
  propertyId: string,
): UseMutationResult<PaymentOperation, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (operationId: string) => {
      const response = await apiClient<components['schemas']['OperationResponse']>(
        `/properties/${encodeURIComponent(propertyId)}`
          + `/operations/${encodeURIComponent(operationId)}/pay`,
        { method: 'POST' },
      );
      return mapPaymentOperation(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
    },
  });
}

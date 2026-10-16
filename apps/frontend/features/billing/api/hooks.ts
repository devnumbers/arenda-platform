'use client';

import {
  useQuery,
  useQueryClient,
  type UseQueryResult,
  type UseMutationResult,
} from '@tanstack/react-query';
import { useGuardedMutation } from '@/shared/lib/hooks/use-guarded-mutation';
import { apiClient } from '@/shared/api/client';
import { nullOn404 } from '@/shared/api/null-on-404';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { type TariffName } from '@/entities/user';
import {
  mapAddPaymentMethodResponse,
  mapChangeTariffResponse,
  mapPaymentMethodResponse,
  mapSubscriptionPaymentResponse,
  mapSubscriptionResponse,
  mapTariffResponse,
} from '@/entities/billing';
import type {
  AddPaymentMethodResult,
  ChangeTariffResult,
  PaymentMethod,
  Subscription,
  SubscriptionPayment,
  Tariff,
} from '@/entities/billing';
import { billingKeys } from '@/shared/api/query-keys';

type AutoRenewRequest = components['schemas']['AutoRenewRequest'];
type CancelSubscriptionRequest = components['schemas']['CancelSubscriptionRequest'];
type ChangeTariffRequest = Omit<
  components['schemas']['ChangeTariffRequest'],
  'tariffName'
> & {
  tariffName: TariffName;
};
type AddPaymentMethodRequest = components['schemas']['AddPaymentMethodRequest'];
type SubscriptionPaymentsResponse =
  components['schemas']['SubscriptionPaymentsResponse'];

const PAYMENT_POLL_INTERVAL_MS = 5000;
const PAYMENT_POLL_MAX_AGE_MS = 30 * 60 * 1000;

/** Polling продолжается, пока есть pending-платёж не старше PAYMENT_POLL_MAX_AGE_MS. */
function hasRecentPendingPayment(
  data: SubscriptionPaymentsResponse | undefined,
  paymentId?: string,
): boolean {
  if (!data) return false;
  return data.items.some((item) => {
    if (item.status !== 'pending') return false;
    if (paymentId && item.id !== paymentId) return false;
    const ageMs = Date.now() - new Date(item.createdAt).getTime();
    return ageMs < PAYMENT_POLL_MAX_AGE_MS;
  });
}

export function useTariffs(): UseQueryResult<Tariff[], ApiError> {
  return useQuery({
    queryKey: billingKeys.tariffs,
    queryFn: () => apiClient<components['schemas']['TariffsResponse']>('/tariffs'),
    select: (data) => data.items.map(mapTariffResponse),
  });
}

/** 404 → null — «подписки нет» (#768): отсутствие подписки — состояние
 * данных, а не ошибка; правило — в nullOn404. */
export function useSubscription(): UseQueryResult<Subscription | null, ApiError> {
  return useQuery({
    queryKey: billingKeys.subscription,
    queryFn: () => nullOn404(() => apiClient<components['schemas']['Subscription']>('/subscription')),
    select: (data) => (data === null ? null : mapSubscriptionResponse(data)),
  });
}

export function useToggleAutoRenew(): UseMutationResult<
  void,
  ApiError,
  AutoRenewRequest
> {
  const queryClient = useQueryClient();

  return useGuardedMutation({
    mutationFn: async ({ enabled }: AutoRenewRequest) => {
      await apiClient<void>('/subscription/auto-renew', {
        method: 'PATCH',
        body: JSON.stringify({ enabled }),
      });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useCancelSubscription(): UseMutationResult<
  void,
  ApiError,
  CancelSubscriptionRequest | void
> {
  const queryClient = useQueryClient();

  return useGuardedMutation({
    mutationFn: async (request?: CancelSubscriptionRequest | void) => {
      await apiClient<void>('/subscription/cancel', {
        method: 'POST',
        body: JSON.stringify(request ?? {}),
      });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useResumeSubscription(): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();

  return useGuardedMutation({
    mutationFn: async () => {
      await apiClient<void>('/subscription/resume', { method: 'POST' });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useChangeTariff(): UseMutationResult<
  ChangeTariffResult,
  ApiError,
  ChangeTariffRequest
> {
  const queryClient = useQueryClient();

  return useGuardedMutation({
    mutationFn: async ({ tariffName, period }: ChangeTariffRequest) => {
      const response = await apiClient<components['schemas']['ChangeTariffResponse']>(
        '/subscription/change',
        {
          method: 'POST',
          body: JSON.stringify({ tariffName, period }),
        },
      );
      return mapChangeTariffResponse(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function usePaymentMethods(): UseQueryResult<PaymentMethod[], ApiError> {
  return useQuery({
    queryKey: billingKeys.paymentMethods,
    queryFn: () =>
      apiClient<components['schemas']['PaymentMethodsResponse']>('/subscription/payment-methods'),
    select: (data) => data.items.map(mapPaymentMethodResponse),
  });
}

export function useAddPaymentMethod(): UseMutationResult<
  AddPaymentMethodResult,
  ApiError,
  AddPaymentMethodRequest
> {
  const queryClient = useQueryClient();

  return useGuardedMutation({
    mutationFn: async (body: AddPaymentMethodRequest) => {
      const response = await apiClient<components['schemas']['AddPaymentMethodResponse']>(
        '/subscription/payment-methods',
        {
          method: 'POST',
          body: JSON.stringify(body),
        },
      );
      return mapAddPaymentMethodResponse(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
    },
  });
}

export function useActivatePaymentMethod(): UseMutationResult<
  void,
  ApiError,
  string
> {
  const queryClient = useQueryClient();

  return useGuardedMutation({
    mutationFn: async (id: string) => {
      await apiClient<void>(
        `/subscription/payment-methods/${encodeURIComponent(id)}/activate`,
        { method: 'POST' },
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useSyncPaymentMethods(): UseMutationResult<
  components['schemas']['PaymentMethodsResponse'],
  ApiError,
  void
> {
  const queryClient = useQueryClient();

  return useGuardedMutation({
    mutationFn: () =>
      apiClient<components['schemas']['PaymentMethodsResponse']>(
        '/subscription/payment-methods/sync',
        { method: 'POST' },
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useDeletePaymentMethod(): UseMutationResult<
  void,
  ApiError,
  string
> {
  const queryClient = useQueryClient();

  return useGuardedMutation({
    mutationFn: async (id: string) => {
      await apiClient<void>(
        `/subscription/payment-methods/${encodeURIComponent(id)}`,
        { method: 'DELETE' },
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
    onError: () => {
      // Отказ (409 in use — карта успела стать активной) означает, что
      // локальный список устарел: пересинхронизируем, иначе гард удаления
      // и радио продолжат показывать ушедшее состояние (#625).
      void queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
    },
  });
}

export function useSubscriptionPayments(): UseQueryResult<
  SubscriptionPayment[],
  ApiError
> {
  return useQuery({
    queryKey: billingKeys.payments,
    queryFn: () =>
      apiClient<components['schemas']['SubscriptionPaymentsResponse']>('/subscription/payments'),
    select: (data) => data.items.map(mapSubscriptionPaymentResponse),
  });
}

export function useSubscriptionPayment(
  id: string,
  enabled = true,
): UseQueryResult<SubscriptionPayment | undefined, ApiError> {
  return useQuery({
    queryKey: billingKeys.payment(id),
    queryFn: () =>
      apiClient<components['schemas']['SubscriptionPaymentsResponse']>('/subscription/payments'),
    select: (data) =>
      data.items
        .map(mapSubscriptionPaymentResponse)
        .find((payment) => payment.id === id),
    enabled,
    refetchInterval: (query) =>
      hasRecentPendingPayment(query.state.data, id)
        ? PAYMENT_POLL_INTERVAL_MS
        : false,
  });
}
